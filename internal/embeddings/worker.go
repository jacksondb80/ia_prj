package embeddings

import (
	"log"
	"sync"

	"iaprj/internal/model"
	"iaprj/internal/repository"
)

func RunWorkers(
	products []model.RawProduct,
	vectorRepo *repository.VectorRepository,
	rawRepo *repository.RawRepository,
	workers int,
) {

	const maxWorkers = 8
	jobs := make(chan model.RawProduct)
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				process(p, vectorRepo, rawRepo)
			}
		}()
	}

	for _, p := range products {
		jobs <- p
	}
	close(jobs)
	wg.Wait()
}

func process(
	p model.RawProduct,
	vectorRepo *repository.VectorRepository,
	rawRepo *repository.RawRepository,
) {
	chunks := Chunk(p.Content, 1000)
	success := true
	for _, c := range chunks {
		embedding, err := Embed(c)
		if err != nil {
			log.Printf("Erro ao gerar embedding para %s: %v", p.ProdutoID, err)
			success = false
			continue
		}
		if err := vectorRepo.Save(p.ProdutoID, p.SourceURL, p.ImageURL, p.Brand, p.Btus, p.Ciclo, p.Voltagem, p.Tecnologia, p.Type, c, embedding); err != nil {
			log.Printf("Erro ao salvar vetor para %s: %v", p.ProdutoID, err)
			success = false
		}
	}
	if success {
		rawRepo.MarkAsProcessed(p.ProdutoID)
		log.Printf("Sucesso ao processar produto %s", p.ProdutoID)
	} else {
		log.Printf("Falha ao processar produto %s", p.ProdutoID)
	}
}
