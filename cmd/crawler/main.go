package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"iaprj/internal/config"
	"iaprj/internal/crawler"
	"iaprj/internal/db"
	"iaprj/internal/model"
	"iaprj/internal/repository"
)

// go run cmd/crawler/main.go -mode=ids -ids="kit123,kit456,kit789"
// go run cmd/crawler/main.go -mode=category -cat="ar-condicionado"
func main() {
	mode := flag.String("mode", "category", "Modo de execução: 'ids' ou 'category'")
	cat := flag.String("cat", "ar-condicionado", "ID da categoria para busca")
	idsArg := flag.String("ids", "kit11106,kit9428,kit9429,kit10572", "IDs dos produtos separados por vírgula")
	flag.Parse()

	cfg := config.Load()
	dbConn, _ := db.New(cfg.DatabaseURL)

	repo := &repository.RawRepository{DB: dbConn}

	handler := func(p crawler.OCCProduct) {
		text := crawler.ProductToText(&p)
		// salvar no postgres
		fmt.Println(text)
		repo.Save(model.RawProduct{
			ID:        uuid.New().String(),
			ProdutoID: p.ID,
			SourceURL: p.Url,
			Content:   text,
		})
	}

	if *mode == "ids" {
		ids := strings.Split(*idsArg, ",")
		for i := range ids {
			ids[i] = strings.TrimSpace(ids[i])
		}
		crawler.CrawlBatch(ids, 10, handler)
	} else {
		if err := crawler.FetchProductsByCategory(*cat, handler); err != nil {
			log.Printf("Erro ao buscar categoria %s: %v", *cat, err)
		}
	}

	log.Println("Crawler finalizado")
}
