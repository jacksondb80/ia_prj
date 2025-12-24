package chat

import (
	"math/rand"
	"strings"
)

type ToolPriceRequest struct {
	ProdutoID string  `json:"produto_id"`
	CEP       string  `json:"cep"`
	SalePrice float32 `json:"sale_price"`
	Length    float32 `json:"length"`
	Weight    float32 `json:"weight"`
	Width     float32 `json:"width"`
	Height    float32 `json:"height"`
}

type ToolPriceResponse struct {
	ProdutoID string  `json:"produto_id"`
	Preco     float64 `json:"preco"`
	Frete     float64 `json:"frete"`
	Estoque   bool    `json:"estoque"`
}

func GetPrice(req ToolPriceRequest) ToolPriceResponse {
	// 🔗 Aqui entra sua API real (ERP / ecommerce)
	if req.CEP != "" {
		// Busca preço com frete e estoque
	}

	// Retorna preço base
	return ToolPriceResponse{
		ProdutoID: req.ProdutoID,
		Preco:     float64(req.SalePrice),
		Frete:     50.0 + rand.Float64()*200.0, // Frete entre R$ 50 e R$ 250
		Estoque:   rand.Float64() > 0.2,        // 80% de chance de ter estoque
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
