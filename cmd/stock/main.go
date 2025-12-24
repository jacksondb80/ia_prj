package main

import (
	"context"
	"log"
	"sync"
	"time"

	"iaprj/internal/config"
	"iaprj/internal/repository"
	"iaprj/internal/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	log.Println("Iniciando serviço de atualização de estoque...")

	// Conecta ao banco de dados
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao banco de dados: %v", err)
	}
	defer pool.Close()

	repo := &repository.VectorRepository{DB: pool}

	// Busca produtos para atualizar
	log.Println("Buscando produtos no banco de dados...")
	products, err := repo.GetAllProductsForUpdate()
	if err != nil {
		log.Fatalf("Erro ao buscar produtos: %v", err)
	}
	log.Printf("Encontrados %d produtos para verificação.", len(products))

	const numWorkers = 8
	jobs := make(chan repository.VectorResult, len(products))
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				// Pequeno delay para não sobrecarregar a API
				time.Sleep(100 * time.Millisecond)

				available := 0
				isAvailable, err := stock.CheckProductAvailability(p.ProdutoID)
				if err != nil {
					log.Printf("Erro ao verificar %s: %v", p.ProdutoID, err)
				} else if isAvailable {
					available = 1
				}

				if err := repo.UpdateStock(p.ProdutoID, available); err != nil {
					log.Printf("Erro ao atualizar estoque para %s: %v", p.ProdutoID, err)
				} else {
					log.Printf("Produto %s atualizado. Estoque: %d", p.ProdutoID, available)
				}
			}
		}()
	}

	for _, p := range products {
		jobs <- p
	}
	close(jobs)

	wg.Wait()

	log.Println("Atualização de estoque finalizada.")
}
