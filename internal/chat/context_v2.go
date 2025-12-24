package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"iaprj/internal/model"
	"iaprj/internal/repository"
)

type ExtractedFilters struct {
	Brand      string `json:"brand"`
	Btus       int    `json:"btus"`
	Ciclo      string `json:"ciclo"`
	Voltagem   string `json:"voltagem"`
	Tecnologia string `json:"tecnologia"`
	Type       string `json:"type"`
}

// extractFiltersWithLLM usa a IA para identificar a intenção de busca e filtros estruturados.
func extractFiltersWithLLM(client *openai.Client, text string) (ExtractedFilters, error) {
	prompt := `Você é um especialista em extração de dados para e-commerce de ar condicionado.
Analise o texto do usuário (que pode conter histórico) e extraia os filtros de busca em formato JSON.

Regras de Extração:
1. **brand**: Marca do produto (ex: Samsung, LG, EOS, Midea, Gree, Electrolux, Daikin, Fujitsu, Springer, Carrier, Philco, Consul).
2. **btus**: Capacidade em BTUs (inteiro). 
   - Se o usuário informar BTUs explicitamente (ex: "12000", "12k"), use esse valor.
   - Se informar área (m²) ou pessoas, CALCULE: (600 * m²) + (600 * pessoas extras acima de 2). Se tiver sol forte, use base 800 * m². Arredonde para o padrão comercial mais próximo (9000, 12000, 18000, 24000, etc).
3. **ciclo**: "Frio" ou "Quente/Frio".
4. **voltagem**: "110V" ou "220V".
5. **tecnologia**: "Inverter" ou "Convencional".
6. **type**: Tipo do aparelho. Valores aceitos: "Split", "Janela", "Portátil", "Cassete", "Piso Teto", "Multi Split".
   - Se não for especificado, assuma "Split".

Retorne APENAS o JSON, sem markdown.`

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: prompt},
				{Role: "user", Content: text},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
			Temperature:    0.0, // Temperatura zero para máxima precisão
		},
	)
	if err != nil {
		return ExtractedFilters{}, err
	}
	var filters ExtractedFilters
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &filters)
	return filters, err
}

// resolveProductReference tenta identificar referências a itens anteriores (ex: "produto 2")
// e substitui pelo nome do produto encontrado na última resposta da IA.
func resolveProductReference(userMsg string, history []model.ChatMessage) string {
	// Regex para capturar "produto 2", "item 3", "número 1", "opção 2", etc.
	reRef := regexp.MustCompile(`(?i)\b(?:produto|item|número|numero|opção|opcao|ar)\s+(\d+)\b`)
	match := reRef.FindStringSubmatch(userMsg)

	if len(match) < 2 {
		return userMsg
	}

	indexStr := match[1]

	// Busca a última mensagem do assistente no histórico
	var lastAssistantMsg string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			lastAssistantMsg = history[i].Content
			break
		}
	}

	if lastAssistantMsg == "" {
		return userMsg
	}

	// Tentativa 1: Formato de lista Markdown (ex: "1. **Nome do Produto**" ou "1. Nome do Produto")
	// Procura por: inicio da linha, numero, ponto, espaço, opcional bold, texto do produto
	reList := regexp.MustCompile(fmt.Sprintf(`(?m)^\s*%s\.\s*(?:\*\*)?([^\n*]+)`, indexStr))
	matchList := reList.FindStringSubmatch(lastAssistantMsg)
	if len(matchList) >= 2 {
		productName := strings.TrimSpace(matchList[1])
		return strings.Replace(userMsg, match[0], productName, 1)
	}

	// Tentativa 2: Formato explícito "Item X:" (caso a IA tenha repetido o formato do contexto interno)
	reItem := regexp.MustCompile(fmt.Sprintf(`(?i)Item\s+%s:\s*\nProduto:\s*(.*?)\n`, indexStr))
	matchItem := reItem.FindStringSubmatch(lastAssistantMsg)
	if len(matchItem) >= 2 {
		return strings.Replace(userMsg, match[0], matchItem[1], 1)
	}

	return userMsg
}

func buildContextV2(
	req ChatRequest,
	history []model.ChatMessage,
	vectorRepo *repository.VectorRepository,
	session *SessionStore,
	client *openai.Client,
) (string, error) {
	//cfg := config.Load()

	// Resolve referências a produtos anteriores (ex: "produto 2" -> "Ar Condicionado Samsung...")
	userMessage := resolveProductReference(req.Message, history)
	if userMessage != req.Message {
		log.Printf("[ChatV2] Referência resolvida: '%s' -> '%s'", req.Message, userMessage)
	}

	// Histórico recente para contexto
	var userHistory []string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			userHistory = append([]string{history[i].Content}, userHistory...)
			if len(userHistory) >= 5 {
				break
			}
		}
	}
	searchQuery := strings.Join(append(userHistory, userMessage), " ")

	log.Printf("[ChatV2] Iniciando análise de contexto. Query expandida: %s", searchQuery)

	// 1. Extração de Filtros via IA
	log.Printf("[ChatV2] Solicitando extração de filtros para a IA...")
	extracted, err := extractFiltersWithLLM(client, searchQuery)
	if err != nil {
		log.Printf("[ChatV2] Erro na extração via IA: %v. Usando fallback manual.", err)
		// Em caso de erro crítico na IA de extração, poderíamos ter um fallback,
		// mas aqui vamos seguir com filtros vazios ou parciais.
	}

	// Lógica de BTU: IA vs Sessão
	// Se a IA detectou/calculou BTUs agora, atualizamos a sessão.
	// Se a IA retornou 0, tentamos pegar da sessão (cálculo anterior).
	targetBTU := extracted.Btus
	sessionBTU, err := session.GetCalculatedBTU(req.SessionID)

	if targetBTU > 0 {
		// IA encontrou um novo valor (explícito ou calculado), atualiza sessão
		session.SetCalculatedBTU(req.SessionID, targetBTU)
	} else {
		// IA não encontrou nada novo, usa o da sessão se existir
		if err == nil && sessionBTU > 0 {
			targetBTU = sessionBTU
		}
	}

	// Mapeia struct para map[string]string para o repositório
	filters := make(map[string]string)
	if extracted.Brand != "" {
		filters["brand"] = extracted.Brand
	}
	if extracted.Ciclo != "" {
		filters["ciclo"] = extracted.Ciclo
	}
	if extracted.Voltagem != "" {
		filters["voltagem"] = extracted.Voltagem
	}
	if extracted.Tecnologia != "" {
		filters["tecnologia"] = extracted.Tecnologia
	}
	if extracted.Type != "" {
		filters["type"] = extracted.Type
	} else {
		filters["type"] = "Split" // Default
		filters["type_exclude"] = "Multi Split"
	}

	log.Printf("[ChatV2] Filtros IA -> Brand: '%s', BTU: %d, Filtros: %v", extracted.Brand, targetBTU, filters)

	var finalResults []repository.VectorResult
	searchStrategy := "Indefinida"

	// 2. Tentativa de Busca Simples (SQL/Metadata)
	// Realizamos a busca simples se houver pelo menos um filtro forte (Marca ou BTU > 0) ou filtros explícitos.
	// Se a query for muito genérica, podemos pular direto para vetor, mas a regra pede "pesquisas sem vetorização" primeiro.

	log.Printf("[ChatV2] Tentando busca por Metadados (SQL)...")

	metadataResults, err := vectorRepo.SearchByMetadata(targetBTU, filters, 5) // Top 5
	if err != nil {
		log.Printf("[ChatV2] Erro na busca por metadados: %v", err)
	}

	if len(metadataResults) > 0 {
		searchStrategy = "Metadados (SQL Simples)"
		finalResults = metadataResults
		log.Printf("[ChatV2] Sucesso: Encontrados %d produtos via Metadados.", len(finalResults))
	}
	// 3. Fallback para Busca Vetorial
	// Se não encontrou nada via SQL (ou filtros não foram suficientes), usa Embedding.
	searchStrategy = "Semântica (Vetorial - Fallback)"
	log.Println("[ChatV2] Nenhum resultado via Metadados. Iniciando busca Vetorial (Fallback)...")

	log.Printf("[ChatV2] Gerando embedding para: %s", searchQuery)

	charCount := len(searchQuery)
	tokenEstimate := charCount / 4
	log.Printf("[Embedding] Payload Stats: %d caracteres | ~%d tokens estimados", charCount, tokenEstimate)

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
	log.Printf("[ChatV2] Embedding gerado com sucesso.")
	queryVector := embResp.Data[0].Embedding

	// Configuração de Boost de Marcas (igual ao chat v1)
	boostedBrands := map[string]float64{
		"EOS": 0.05, "Samsung": 0.94, "Midea": 0.95, "LG": 0.96, "Gree": 0.96, "Electrolux": 0.97,
	}
	if extracted.Brand != "" {
		boostedBrands[extracted.Brand] = 0.05
	}

	log.Printf("[ChatV2] Executando SearchSimilar no banco...")

	// Busca Vetorial
	vectorResults, err := vectorRepo.SearchSimilar(
		queryVector,
		minSimilarity,
		maxContextChunks,
		boostedBrands,
		targetBTU,
		filters,
	)
	if err != nil {
		return "", err
	}

	// Se ainda assim não encontrar e tiver BTU restritivo, tenta relaxar o BTU (Fallback de BTU)
	if len(vectorResults) == 0 && targetBTU > 0 {
		log.Println("[ChatV2] Vetorial com BTU falhou. Tentando sem restrição de BTU...")
		vectorResults, _ = vectorRepo.SearchSimilar(
			queryVector, minSimilarity, maxContextChunks, boostedBrands, 0, filters,
		)
	}

	finalResults = append(finalResults, vectorResults...)

	// Limita a 10 produtos conforme solicitado
	if len(finalResults) > 10 {
		finalResults = finalResults[:10]
	}

	// Enriquece os resultados com o conteúdo completo (todos os chunks)
	for i := range finalResults {
		chunks, err := vectorRepo.GetChunksByProductID(finalResults[i].ProdutoID)
		if err == nil && len(chunks) > 0 {
			finalResults[i].Content = strings.Join(chunks, "\n")
		}
	}

	log.Printf("[ChatV2] Estratégia final: %s | Produtos selecionados: %d", searchStrategy, len(finalResults))

	if len(finalResults) == 0 {
		return "Desculpe, não encontrei produtos correspondentes à sua busca.", nil
	}

	// 4. Montagem do Contexto (Preços e Formatação)
	type productWithPrice struct {
		result       repository.VectorResult
		priceInfo    ToolPriceResponse
		priceDetails string
	}

	var products []productWithPrice
	cep := extractCEP(searchQuery)

	for _, r := range finalResults {
		priceInfo := GetPrice(ToolPriceRequest{ProdutoID: r.ProdutoID, CEP: cep, SalePrice: r.SalePrice, Length: r.Length, Weight: r.Weight, Width: r.Width, Height: r.Height})

		if cep != "" && !priceInfo.Estoque {
			continue
		}

		priceDetails := fmt.Sprintf("Preço: R$%.2f", priceInfo.Preco)
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
	if targetBTU > 0 {
		builder.WriteString(fmt.Sprintf("SISTEMA: Capacidade considerada: %d BTUs.\n\n", targetBTU))
	}
	builder.WriteString("PRODUTOS ENCONTRADOS:\n\n")

	// Regex para limpar espaços ao redor de quebras de linha e compactar múltiplos enters
	reClean := regexp.MustCompile(`\s*\n\s*`)

	for i, p := range products {
		cleanContent := reClean.ReplaceAllString(p.result.Content, "\n")
		builder.WriteString(
			fmt.Sprintf("Item %d:\nProduto: %s\nMarca: %s\nSpecs: %d BTUs, %s, %s\nDados de Venda: %s\nLink: %s\nImagem: %s\nDescrição: %s\n\n",
				i+1, strings.Split(p.result.Content, "\n")[0], p.result.Brand,
				p.result.Btus, p.result.Ciclo, p.result.Tecnologia,
				p.priceDetails, p.result.SourceURL, p.result.ImageURL, strings.TrimSpace(cleanContent)),
		)
	}

	return builder.String(), nil
}
