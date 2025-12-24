package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
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
	Stock     int     `json:"stock"`
}

type ToolPriceResponse struct {
	ProdutoID string  `json:"produto_id"`
	Preco     float64 `json:"preco"`
	Frete     float64 `json:"frete"`
	Estoque   bool    `json:"estoque"`
}

type Payload struct {
	Address Address `json:"address"`
	Items   []Item  `json:"items"`
}

type Address struct {
	Country    string `json:"country"`
	LastName   string `json:"lastName"`
	FirstName  string `json:"firstName"`
	Address1   string `json:"address1"`
	Address2   string `json:"address2"`
	PostalCode string `json:"postalCode"`
	State      string `json:"state"`
}

type Item struct {
	ID            string   `json:"id"`
	Qtd           int      `json:"qtd"`
	Width         float32  `json:"width"`
	Height        float32  `json:"height"`
	Weight        float32  `json:"weight"`
	Length        float32  `json:"length"`
	Price         float32  `json:"price"`
	ProductType   string   `json:"productType"`
	Collections   []string `json:"collections,omitempty"`
	CommerceGroup string   `json:"commerceGroup,omitempty"`
}

type ShippingResponse struct {
	ShippingGroups []ShippingGroup `json:"shippingGroups"`
}

type ShippingGroup struct {
	ShippingGroupID string         `json:"shippingGroupId"`
	Produtos        []Produto      `json:"produtos"`
	LocationID      string         `json:"locationId"`
	PostalCode      string         `json:"postalCode"`
	Type            string         `json:"type"`
	ShippingMethods ShippingMethod `json:"shippingMethods"`
	PrecoEspecial   float64        `json:"precoEspecial"`
}

type Produto struct {
	ID               string   `json:"id"`
	Quantidade       int      `json:"quantidade"`
	TakeOutAvailable bool     `json:"takeOutAvailable"`
	TakeOutStores    []string `json:"takeOutStores"`
	FreeShipping     bool     `json:"freeShipping"`
}

type ShippingMethod struct {
	Success bool             `json:"success"`
	Methods []ShippingDetail `json:"methods"`
}

type ShippingDetail struct {
	EligibleForProductWithSurcharges bool    `json:"eligibleForProductWithSurcharges"`
	EstimatedDeliveryDateGuaranteed  bool    `json:"estimatedDeliveryDateGuaranteed"`
	InternationalDutiesTaxesFees     float64 `json:"internationalDutiesTaxesFees"`
	TaxIncluded                      bool    `json:"taxIncluded"`
	ShippingTax                      float64 `json:"shippingTax"`
	Taxcode                          int     `json:"taxcode"`
	Currency                         string  `json:"currency"`
	ShippingCost                     float64 `json:"shippingCost"`
	DisplayName                      string  `json:"displayName"`
	EstimatedDeliveryDate            string  `json:"estimatedDeliveryDate"`
	ShippingTotal                    float64 `json:"shippingTotal"`
	DiasExtraFrete                   int     `json:"diasExtraFrete"`
	DeliveryDays                     int     `json:"deliveryDays"`
	Simfrete                         string  `json:"Simfrete"`
	LocationID                       string  `json:"locationId"`
}

type AddressLookupResponse struct {
	Success bool         `json:"success"`
	Result  *AddressData `json:"result"`
	Errors  interface{}  `json:"errors"`
}

type AddressData struct {
	ZipCode          string  `json:"zipCode"`
	StreetType       string  `json:"streetType"`
	Street           string  `json:"street"`
	Complement       string  `json:"complement"`
	Place            string  `json:"place"`
	Neighborhood     string  `json:"neighborhood"`
	City             string  `json:"city"`
	StateAbreviation string  `json:"stateAbreviation"`
	State            string  `json:"state"`
	IbgeCityCode     string  `json:"ibgeCityCode"`
	IbgeStateCode    *string `json:"ibgeStateCode"` // null no JSON
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	DataOrigin       string  `json:"dataOrigin"`
}

func GetPrice(req ToolPriceRequest) ToolPriceResponse {
	if req.CEP != "" {
		// Tenta calcular o frete via API
		item := Item{
			ID:          req.ProdutoID,
			Qtd:         1,
			Width:       req.Width,
			Height:      req.Height,
			Weight:      req.Weight,
			Length:      req.Length,
			Price:       req.SalePrice,
			ProductType: "ar-condicionado",
		}
		if resp, err := CalculateShipping(req.CEP, []Item{item}); err == nil && len(resp.ShippingGroups) > 0 {
			if resp.ShippingGroups[0].LocationID == "UNAVAILABLE" {
				return ToolPriceResponse{
					ProdutoID: req.ProdutoID,
					Preco:     float64(req.SalePrice),
					Frete:     50.0 + rand.Float64()*200.0, // Frete entre R$ 50 e R$ 250
					Estoque:   false,
				}
			}
			if methods := resp.ShippingGroups[0].ShippingMethods.Methods; len(methods) > 0 {
				return ToolPriceResponse{
					ProdutoID: req.ProdutoID,
					Preco:     float64(req.SalePrice),
					Frete:     methods[0].ShippingCost,
					Estoque:   true,
				}
			}
		}
	}

	// Retorna preço base
	return ToolPriceResponse{
		ProdutoID: req.ProdutoID,
		Preco:     float64(req.SalePrice),
		Frete:     50.0 + rand.Float64()*200.0, // Frete entre R$ 50 e R$ 250
		Estoque:   req.Stock > 0,
	}
}

func CalculateShipping(cep string, items []Item) (*ShippingResponse, error) {
	const apiURL = "https://api.frigelar.com.br/fgl_svc_logistic/logistic"

	state := "SC" // Fallback
	if addr, err := LookupAddress(cep); err == nil && addr.Success && addr.Result != nil {
		state = addr.Result.StateAbreviation
		log.Printf("[CalculateShipping] Estado identificado para CEP %s: %s", cep, state)
	}

	payload := Payload{
		Address: Address{
			Country:    "BR",
			PostalCode: cep,
			State:      state,
		},
		Items: items,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Printf("[CalculateShipping] Request Body: %s", string(jsonData))

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("[CalculateShipping] Sending POST to %s", apiURL)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Printf("[CalculateShipping] Response Status: %d | Body: %s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var shippingResp ShippingResponse
	if err := json.NewDecoder(resp.Body).Decode(&shippingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &shippingResp, nil
}

func LookupAddress(cep string) (*AddressLookupResponse, error) {
	url := fmt.Sprintf("https://fglazprdassvczipcode.azurewebsites.net/api/v1/ZipCode/%s", cep)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	const token = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxNzM5Y2JmMi0wNGRhLTQ0ZjAtYTBjNC0yZTQ5OTQ0YjkwMTQiLCJhcHBJZCI6IjQxNGYzMWU4LTUyZDQtNDA3Yy1iZmNhLTYxZDZhZDUzMDUzOSIsImN1c3RvbWVyRXJwSWQiOiIwIiwicmVnaW9uSWQiOiIwIiwiaWF0IjoiMTU4MDM5NDU1NSIsImV4cCI6IjE5MjQ5NTc3NTUiLCJqdGkiOiJiZTNhOGUzOS0zYjYzLTQ2YzUtYmZiZS0wYThiNmZlMDZhMjYiLCJhdWQiOiJGcmlnZWxhciIsImlzcyI6IkZyaWdlbGFyIn0.jPbPnV32CIYP_OvYT7i8qy7K9aC0hfzE86ttX3Td9Qg3Qc9Z91VzJa6C7WpFZJsPx2OjbHnfO0dV7CFL-Q164Q"
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result AddressLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
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
