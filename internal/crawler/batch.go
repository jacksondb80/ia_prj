package crawler

import "log"

func CrawlBatch(productIDs []string, batchSize int, handler func(OCCProduct)) {
	for i := 0; i < len(productIDs); i += batchSize {
		end := i + batchSize
		if end > len(productIDs) {
			end = len(productIDs)
		}

		products, err := FetchProductsByIDs(productIDs[i:end])
		if err != nil {
			log.Println("Erro batch:", err)
			continue
		}

		for _, p := range products {
			handler(p)
		}
	}
}
