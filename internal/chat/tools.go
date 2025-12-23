package chat

import (
	"math/rand"
	"strings"
)

type ToolPriceRequest struct {
	ProdutoID string `json:"produto_id"`
	CEP       string `json:"cep"`
}

type ToolPriceResponse struct {
	ProdutoID string  `json:"produto_id"`
	Preco     float64 `json:"preco"`
	Frete     float64 `json:"frete"`
	Estoque   bool    `json:"estoque"`
}

func GetPrice(req ToolPriceRequest) ToolPriceResponse {
	// 🔗 Aqui entra sua API real (ERP / ecommerce)
	return ToolPriceResponse{
		ProdutoID: req.ProdutoID,
		Preco:     1500.0 + rand.Float64()*3500.0, // Preço entre R$ 1.500 e R$ 5.000
		Frete:     50.0 + rand.Float64()*200.0,    // Frete entre R$ 50 e R$ 250
		Estoque:   rand.Float64() > 0.2,           // 80% de chance de ter estoque
	}
}

func CreateCartURL(produtoID string) string {
	return "https://seusite.com/carrinho?produto=" + produtoID
}

// CalculateBTU estima a capacidade necessária e arredonda para o tamanho comercial mais próximo
func CalculateBTU(area float64, sunPosition string, people int) int {
	base := 600.0
	if strings.Contains(strings.ToLower(sunPosition), "tarde") {
		base = 800.0
	}

	btu := area * base
	if people > 1 {
		btu += float64(people-1) * 600.0
	}

	// Tamanhos comerciais comuns
	sizes := []int{7500, 9000, 12000, 18000, 24000, 27000, 30000, 36000, 48000, 60000}

	for _, size := range sizes {
		if btu <= float64(size) {
			return size
		}
	}
	return 60000 // Retorna o maior se passar
}
