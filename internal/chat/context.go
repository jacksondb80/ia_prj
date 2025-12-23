package chat

import (
	"context"
	"fmt"
	"iaprj/internal/config"
	"log"
	"regexp"
	"strconv"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"iaprj/internal/model"
	"iaprj/internal/repository"
)

const (
	maxContextChunks   = 30
	maxContextProducts = 15   // Limite máximo de produtos enviados para a IA analisar
	minSimilarity      = 0.15 // Reduzido para permitir que marcas impulsionadas (House Brand) apareçam mesmo com score menor
)

// extractCEP encontra um CEP no formato 00000-000 ou 00000000 no texto.
func extractCEP(text string) string {
	// Regex para CEP brasileiro
	re := regexp.MustCompile(`\b\d{5}-?\d{3}\b`)
	return re.FindString(text)
}

// extractTargetBTU tenta extrair uma capacidade explícita (ex: 12000, 12 mil, 12k)
func extractTargetBTU(text string) int {
	text = strings.ToLower(text)

	// Regex para "12000", "12.000", "9000"
	reNum := regexp.MustCompile(`\b(\d{1,3})[.]?(\d{3})\b`)
	if match := reNum.FindStringSubmatch(text); len(match) > 0 {
		val, _ := strconv.Atoi(match[1] + match[2])
		// Filtro de sanidade (7k a 80k)
		if val >= 7000 && val <= 80000 {
			return val
		}
	}

	// Regex para "12 mil", "12k", "12mil"
	reK := regexp.MustCompile(`\b(\d{1,2})\s*(?:k|mil)\b`)
	if match := reK.FindStringSubmatch(text); len(match) > 0 {
		val, _ := strconv.Atoi(match[1])
		return val * 1000
	}

	return 0
}

// tryCalculateBTU tenta extrair dados de área, sol e pessoas para calcular BTUs
func tryCalculateBTU(text string) (int, bool) {
	// Regex para Área (ex: 20m2, 20 m², 20 metros, 20mt, 20mts)
	reArea := regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:m²|m2|metros|mt|mts)`)
	matchArea := reArea.FindStringSubmatch(text)

	if len(matchArea) == 0 {
		return 0, false
	}

	// Remove a área encontrada do texto para evitar que o número da área seja confundido com pessoas
	textWithoutArea := strings.Replace(text, matchArea[0], "", 1)

	// Regex para Sol (ex: sol da manhã, sol tarde, ou apenas manhã/tarde isolado)
	reSun := regexp.MustCompile(`(?i)\b(?:sol\s*(?:da\s*)?)?(manh[ãa]|tarde)\b`)
	matchSun := reSun.FindStringSubmatch(text)

	// Regex para Pessoas (ex: 2 pessoas, 3 pessoas, ou apenas um número isolado como "3")
	rePeople := regexp.MustCompile(`(?i)\b(\d+)\s*(?:pessoas?|pess|p)?\b`)
	matchPeople := rePeople.FindStringSubmatch(textWithoutArea)

	if len(matchSun) > 1 {
		areaStr := strings.Replace(matchArea[1], ",", ".", 1)
		area, _ := strconv.ParseFloat(areaStr, 64)

		people := 2 // Default se não informado
		if len(matchPeople) > 1 {
			people, _ = strconv.Atoi(matchPeople[1])
		}

		return CalculateBTU(area, matchSun[1], people), true
	}
	return 0, false
}

// extractFilters extracts structured filters from text.
func extractFilters(text string) map[string]string {
	filters := make(map[string]string)
	lowerText := strings.ToLower(text)

	// Tecnologia
	if strings.Contains(lowerText, "inverter") {
		filters["tecnologia"] = "Inverter"
	} else if strings.Contains(lowerText, "on/off") || strings.Contains(lowerText, "convencional") {
		filters["tecnologia"] = "Convencional"
	}

	// Ciclo
	if strings.Contains(lowerText, "quente e frio") || strings.Contains(lowerText, "quente/frio") {
		filters["ciclo"] = "Quente/Frio"
	} else if strings.Contains(lowerText, "só frio") || (strings.Contains(lowerText, "frio") && !strings.Contains(lowerText, "quente")) {
		filters["ciclo"] = "Frio"
	}

	// Voltagem
	if strings.Contains(lowerText, "220v") || strings.Contains(lowerText, "220 v") {
		filters["voltagem"] = "220V"
	} else if strings.Contains(lowerText, "110v") || strings.Contains(lowerText, "110 v") || strings.Contains(lowerText, "127v") {
		filters["voltagem"] = "110V" // Assuming 127V maps to 110V products
	}

	// Tipo de Ar
	if strings.Contains(lowerText, "multi split") || strings.Contains(lowerText, "multisplit") {
		filters["type"] = "Multi Split"
	} else if strings.Contains(lowerText, "janela") {
		filters["type"] = "Janela"
	} else if strings.Contains(lowerText, "piso teto") {
		filters["type"] = "Piso Teto"
	} else if strings.Contains(lowerText, "cassete") {
		filters["type"] = "Cassete"
	} else if strings.Contains(lowerText, "portátil") || strings.Contains(lowerText, "portatil") {
		filters["type"] = "Portátil"
	} else if strings.Contains(lowerText, "split") {
		filters["type"] = "Split" // User explicitly asked for split
	}

	return filters
}

func buildContext(
	req ChatRequest,
	history []model.ChatMessage,
	vectorRepo *repository.VectorRepository,
	session *SessionStore,
) (string, error) {
	cfg := config.Load()
	userMessage := req.Message

	// Combina a mensagem atual com as últimas 3 mensagens do usuário para manter o contexto
	var userHistory []string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			userHistory = append([]string{history[i].Content}, userHistory...)
			if len(userHistory) >= 3 {
				break
			}
		}
	}
	searchQuery := strings.Join(append(userHistory, userMessage), " ")

	// Tenta extrair BTU da mensagem atual (prioridade sobre histórico e cálculos antigos)
	currentBTU := extractTargetBTU(userMessage)

	var targetBTU int
	var calculatedBTU int
	var isCalculated bool

	// Verifica se já existe um BTU calculado na sessão
	sessionBTU, err := session.GetCalculatedBTU(req.SessionID)
	if err == nil && sessionBTU > 0 {
		targetBTU = sessionBTU
		log.Printf("Usando BTU da sessão: %d", targetBTU)
	}

	if currentBTU > 0 {
		targetBTU = currentBTU
		session.SetCalculatedBTU(req.SessionID, 0)
	} else {
		// Se não pediu explicitamente agora, tenta calcular com base no histórico ou extrair do histórico
		calculatedBTU, isCalculated = tryCalculateBTU(searchQuery)
		if isCalculated {
			// Salva o BTU calculado na sessão para ser usado em perguntas futuras
			session.SetCalculatedBTU(req.SessionID, calculatedBTU)
			log.Printf("Novo BTU calculado e salvo na sessão: %d", calculatedBTU)
			// Adiciona o termo de busca para encontrar produtos compatíveis
			searchQuery += fmt.Sprintf(" ar condicionado %d btus", calculatedBTU)
			targetBTU = calculatedBTU
		} else if targetBTU == 0 {
			targetBTU = extractTargetBTU(searchQuery)
		}
	}

	// Extrai filtros estruturados da query de busca
	structuredFilters := extractFilters(searchQuery)

	// Se a busca foi por cálculo de BTU, priorize o tipo 'Split' para evitar resultados inesperados.
	if isCalculated {
		structuredFilters["type"] = "Split"
	}

	// DEFAULT BEHAVIOR: If no specific type is mentioned, exclude Multi-Splits from generic searches.
	if _, typeMentioned := structuredFilters["type"]; !typeMentioned {
		structuredFilters["type_exclude"] = "Multi Split"
	}

	var finalResults []repository.VectorResult
	var searchStrategy string

	// A busca agora é sempre Híbrida/Vetorial para garantir que a priorização de marca (EOS) seja respeitada em todas as buscas.
	searchStrategy = "Semântica/Híbrida (Vetorial)"
	log.Println("Iniciando busca Semântica/Híbrida...")
	// 1. gerar embedding da pergunta
	client := openai.NewClient(cfg.OpenAIKey)

	embResp, err := client.CreateEmbeddings(
		context.Background(),
		openai.EmbeddingRequest{
			Model: "text-embedding-3-small",
			Input: searchQuery,
		},
	)
	if err != nil {
		return "", err
	}

	queryVector := embResp.Data[0].Embedding

	// Marcas que queremos dar um "boost" na relevância (aparecerão primeiro se tiverem score similar)
	// O multiplicador é aplicado na distância, então quanto MENOR o valor, MAIOR a prioridade.
	boostedBrands := map[string]float64{
		"EOS":        0.05, // Prioridade Máxima (House Brand) - Sempre no topo
		"Samsung":    0.94,
		"Midea":      0.95,
		"LG":         0.96, // Prioridade Média
		"Gree":       0.96,
		"Electrolux": 0.97, // Prioridade Normal
	}

	// DETECÇÃO DINÂMICA: Se o usuário citou a marca explicitamente, aplicamos SUPER BOOST
	lowerQuery := strings.ToLower(searchQuery)
	for brand := range boostedBrands {
		if strings.Contains(lowerQuery, strings.ToLower(brand)) {
			boostedBrands[brand] = 0.05 // Se pediu explicitamente, garante topo absoluto
		}
	}

	// ESTRATÉGIA HÍBRIDA:
	// 1. Busca Dedicada: Garante que produtos da "House Brand" (EOS) apareçam se existirem,
	//    independente do score de similaridade ser baixo.
	houseResults, _ := vectorRepo.SearchByBrand(
		"EOS",
		targetBTU,
		maxContextChunks, // Tenta pegar até 5 produtos EOS
		structuredFilters,
		queryVector,
	)

	// 2. Busca Semântica: Busca concorrentes e outros produtos relevantes.
	//    Aumentamos o limite para garantir variedade na mistura.
	semanticResults, err := vectorRepo.SearchSimilar(
		queryVector,
		minSimilarity,
		maxContextChunks,
		boostedBrands,
		targetBTU,
		structuredFilters,
	)
	if err != nil {
		return "", err
	}

	// Se nenhum resultado for encontrado com o limiar mínimo,
	// fazemos uma busca "best-effort" (fallback)
	if len(semanticResults) < 5 {
		log.Printf("Encontrados apenas %d resultados semânticos. Buscando fallback...", len(semanticResults))
		fallbackResults, err := vectorRepo.SearchSimilar(
			queryVector,
			0.0,              // Sem limiar mínimo de score
			maxContextChunks, // Busca mais opções para compensar o filtro de estoque e garantir variedade
			boostedBrands,
			targetBTU,
			structuredFilters,
		)
		if err != nil {
			return "", err // Retorna erro se a segunda busca falhar
		}
		semanticResults = fallbackResults
	}

	// 3. Merge e Deduplicação
	// Adiciona primeiro os resultados da House Brand (EOS), depois os semânticos.
	seenIDs := make(map[string]bool)
	addUnique := func(list []repository.VectorResult) {
		for _, r := range list {
			if !seenIDs[r.ProdutoID] {
				finalResults = append(finalResults, r)
				seenIDs[r.ProdutoID] = true
			}
		}
	}
	addUnique(houseResults)    // Prioridade total
	addUnique(semanticResults) // Complemento

	// Limita a quantidade de produtos ANTES de buscar preços e ordenar, para garantir que a relevância seja o primeiro critério.
	if len(finalResults) > maxContextProducts {
		finalResults = finalResults[:maxContextProducts]
	}

	log.Printf("Estratégia de busca utilizada: %s", searchStrategy)

	if len(finalResults) == 0 {
		return "Desculpe, não encontrei produtos correspondentes à sua busca.", nil
	}

	// 3. Extrair CEP e montar contexto textual com preços

	type productWithPrice struct {
		result       repository.VectorResult
		priceInfo    ToolPriceResponse
		priceDetails string
	}

	var products []productWithPrice
	cep := extractCEP(searchQuery)

	eosCount := 0
	for _, r := range finalResults {
		priceInfo := GetPrice(ToolPriceRequest{ProdutoID: r.ProdutoID, CEP: cep})

		// Filtra para mostrar apenas produtos com estoque somente se o CEP foi informado
		if cep != "" && !priceInfo.Estoque {
			continue
		}

		// Lógica de Diversidade: Limita a quantidade de produtos EOS para não monopolizar a lista.
		// Garante prioridade (aparecem primeiro), mas não exclusividade.
		if strings.EqualFold(r.Brand, "EOS") {
			if eosCount >= 3 {
				continue // Já temos 3 EOS, pula este para dar chance a outras marcas
			}
			eosCount++
		}

		var priceDetails string
		// Por padrão, mostramos apenas o preço.
		priceDetails = fmt.Sprintf("Preço: R$%.2f", priceInfo.Preco)

		// Se um CEP foi informado, adicionamos os detalhes de frete e estoque.
		if cep != "" {
			if priceInfo.Estoque {
				priceDetails += fmt.Sprintf(" | Frete para %s: R$%.2f | Em estoque", cep, priceInfo.Frete)
			} else {
				priceDetails += fmt.Sprintf(" | Indisponível para o CEP %s", cep)
			}
		} else {
			priceDetails += " (para saber o frete e o estoque, por favor, informe seu CEP)"
		}

		products = append(products, productWithPrice{
			result:       r,
			priceInfo:    priceInfo,
			priceDetails: priceDetails,
		})
	}

	var builder strings.Builder
	if isCalculated {
		builder.WriteString(fmt.Sprintf("SISTEMA: Com base nos dados (Área, Sol, Pessoas), o cálculo recomendado é de %d BTUs. Sugira produtos dessa capacidade.\n\n", calculatedBTU))
	}
	builder.WriteString("PRODUTOS ENCONTRADOS (Use EXCLUSIVAMENTE os dados abaixo. Para cada item, exiba a imagem com Markdown ![Nome](URL), inclua uma breve descrição técnica e crie um link de compra):\n\n")

	for i, p := range products {
		// Constrói uma descrição técnica estruturada e concisa para a IA
		var techDesc strings.Builder

		// Extrai o título do conteúdo completo
		displayName := strings.Split(p.result.Content, "\n")[0]
		techDesc.WriteString(displayName)

		// Adiciona um resumo da descrição de marketing, extraindo do conteúdo completo.
		descRegex := regexp.MustCompile(`(?is)(?:Descrição Detalhada|Descrição):\s*(.*?)\s*URL:`)
		match := descRegex.FindStringSubmatch(p.result.Content)
		if len(match) > 1 {
			marketingSummary := strings.TrimSpace(match[1])
			techDesc.WriteString("\n\n" + marketingSummary)
		}

		// Adiciona os metadados estruturados para garantir que a IA os veja claramente
		techDesc.WriteString("\n\n--- Especificações Chave ---\n")
		if p.result.Btus > 0 {
			techDesc.WriteString(fmt.Sprintf("Capacidade: %d BTUs\n", p.result.Btus))
		}
		if p.result.Ciclo != "" {
			techDesc.WriteString(fmt.Sprintf("Ciclo: %s\n", p.result.Ciclo))
		}
		if p.result.Tecnologia != "" {
			techDesc.WriteString(fmt.Sprintf("Tecnologia: %s\n", p.result.Tecnologia))
		}
		if p.result.Voltagem != "" {
			techDesc.WriteString(fmt.Sprintf("Voltagem: %s\n", p.result.Voltagem))
		}
		if p.result.Type != "" {
			techDesc.WriteString(fmt.Sprintf("Tipo: %s\n", p.result.Type))
		}

		builder.WriteString(
			fmt.Sprintf("Item %d:\nDescrição: %s\nDados de Venda: %s\nLink do Produto: %s\nURL da Imagem: %s\n\n", i+1, techDesc.String(), p.priceDetails, p.result.SourceURL, p.result.ImageURL),
		)
	}

	return builder.String(), nil
}
