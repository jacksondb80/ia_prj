package stock

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type StockResponse struct {
	Success bool        `json:"success"`
	Result  *StockData  `json:"result"`
	Errors  interface{} `json:"errors"`
}

type StockData struct {
	ID              string          `json:"id"`
	StockLevelTotal int             `json:"stockLevelTotal"`
	TotalReserved   int             `json:"totalReserved"`
	Locations       []StockLocation `json:"locations"`
}

type StockLocation struct {
	LocationID string `json:"locationId"`
	StockLevel int    `json:"stockLevel"`
	Reserved   int    `json:"reserved"`
}

func CheckProductAvailability(productID string) (bool, error) {
	// TODO: Substitua pela URL real do seu endpoint de estoque
	url := fmt.Sprintf("https://sag-api-azu.frigelar.com.br:5443/gateway/site-estoque/1.0/v1/%s", productID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Adicione headers de autenticação se necessário
	req.Header.Set("x-Gateway-APIKey", "07e8b969-9712-4e96-9fd4-8f96aeaf8d0c")

	log.Printf("[CheckStock] Checking availability for %s at %s", productID, url)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var stockResp StockResponse
	if err := json.NewDecoder(resp.Body).Decode(&stockResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	// Considera disponível se a flag Available for true OU se a quantidade for maior que 0
	return stockResp.Success && stockResp.Result.StockLevelTotal > 0, nil
}
