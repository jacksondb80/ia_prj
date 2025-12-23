package main

import (
	"context"
	"html"
	"log"
	"regexp"
	"strconv"
	"strings"

	"iaprj/internal/config"
	"iaprj/internal/db"
	"iaprj/internal/embeddings"
	"iaprj/internal/model"
	"iaprj/internal/observability"
	"iaprj/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	observability.Start(cfg.MetricsPort)

	dbConn, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco de dados (db): %v", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Não foi possível criar o pool de conexões: %v\n", err)
	}
	defer pool.Close()

	rawRepo := &repository.RawRepository{DB: dbConn}
	vectorRepo := &repository.VectorRepository{DB: pool}

	products, err := rawRepo.List()
	if err != nil {
		log.Fatalf("Erro ao listar produtos: %v", err)
	}

	// Etapa de Limpeza e Enriquecimento dos dados antes de gerar embeddings
	log.Println("Iniciando limpeza dos dados brutos...")
	for i := range products {
		cleanProductData(&products[i])
		// Log de auditoria para verificar se a extração está funcionando
		if i < 10 || products[i].Brand == "EOS" { // Loga os primeiros 10 e todos os EOS encontrados
			log.Printf("[AUDIT] ID: %s | Marca: '%s' | BTUs: %d | Ciclo: '%s' | Volt: '%s' | Tech: '%s' | Tipo: '%s'", products[i].ProdutoID, products[i].Brand, products[i].Btus, products[i].Ciclo, products[i].Voltagem, products[i].Tecnologia, products[i].Type)
		}
	}

	embeddings.RunWorkers(products, vectorRepo, rawRepo, cfg.WorkerCount)

	log.Println("Embeddings finalizadas")
}

// cleanProductData remove ruídos do conteúdo e ajusta a URL da imagem
func cleanProductData(p *model.RawProduct) {
	// 1. Decodificar HTML entities (ex: &nbsp; -> espaço, &aacute; -> á)
	content := html.UnescapeString(p.Content)

	// 2. Extrair URLs (Source e Imagem) se estiverem no conteúdo
	// Usa (?i) para case-insensitive e \s* para pegar quebras de linha entre o label e o link
	reUrl := regexp.MustCompile(`(?i)URL:\s*(https?://\S+)`)
	if match := reUrl.FindStringSubmatch(content); len(match) > 1 {
		p.SourceURL = match[1]
	}

	// Procura por "Imagem Principal: http..."
	reImg := regexp.MustCompile(`(?i)Imagem Principal:\s*(https?://\S+)`)
	if match := reImg.FindStringSubmatch(content); len(match) > 1 {
		p.ImageURL = match[1]
	}

	// Extrair Marca (ex: "Marca: LG")
	reBrand := regexp.MustCompile(`(?i)Marca:\s*(.+)`)
	if match := reBrand.FindStringSubmatch(content); len(match) > 1 {
		p.Brand = strings.TrimSpace(match[1])
	}

	// Extrair BTUs (ex: "Capacidade: 9000 BTUs")
	reBtu := regexp.MustCompile(`(?i)Capacidade:\s*([\d\.]+)`)
	if match := reBtu.FindStringSubmatch(content); len(match) > 1 {
		valStr := strings.ReplaceAll(match[1], ".", "")
		val, _ := strconv.Atoi(valStr)
		p.Btus = val
	}

	// Extrair Ciclo (ex: "Ciclo: Quente/Frio")
	reCiclo := regexp.MustCompile(`(?i)Ciclo:\s*(.+)`)
	if match := reCiclo.FindStringSubmatch(content); len(match) > 1 {
		p.Ciclo = strings.TrimSpace(match[1])
	}

	// Extrair Voltagem (ex: "Voltagem: 220V")
	reVolt := regexp.MustCompile(`(?i)Voltagem:\s*(.+)`)
	if match := reVolt.FindStringSubmatch(content); len(match) > 1 {
		p.Voltagem = strings.TrimSpace(match[1])
	}

	// Extrair Tecnologia (ex: "Tecnologia: Inverter")
	reTech := regexp.MustCompile(`(?i)Tecnologia:\s*(.+)`)
	if match := reTech.FindStringSubmatch(content); len(match) > 1 {
		p.Tecnologia = strings.TrimSpace(match[1])
	}

	// Extrair Tipo (ex: "Categoria: Ar-Condicionado Split")
	reType := regexp.MustCompile(`(?i)Categoria:\s*Ar-Condicionado\s*(.+)`)
	if match := reType.FindStringSubmatch(content); len(match) > 1 {
		p.Type = strings.TrimSpace(match[1])
	}

	// 3. Remover o bloco de JSON de variantes (gera muito ruído no embedding)
	// Remove "Variants: { ... }"
	reVariants := regexp.MustCompile(`(?s)Variants:\s*\{.*?\}`)
	content = reVariants.ReplaceAllString(content, "")

	// 4. Limpar linhas desnecessárias
	lines := strings.Split(content, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Remove metadados que não ajudam na descrição semântica ou são redundantes
		if strings.HasPrefix(trimmed, "Slug:") ||
			strings.HasPrefix(trimmed, "URL:") ||
			strings.HasPrefix(trimmed, "Imagem Principal:") ||
			strings.HasPrefix(trimmed, "Categoria Extra:") {
			continue
		}
		// Remove também a URL crua se ela ficou solta numa linha (já extraímos para o campo estruturado)
		if trimmed == p.SourceURL || trimmed == p.ImageURL {
			continue
		}

		cleanLines = append(cleanLines, line)
	}

	p.Content = strings.Join(cleanLines, "\n")
}
