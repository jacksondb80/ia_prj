package crawler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// FetchProductsByCategory fetches all products for a given category, handling pagination.
func FetchProductsByCategory(categoryID string, handler func(OCCProduct)) error {
	baseURL := "https://www.frigelar.com.br"
	nextURL := fmt.Sprintf("%s/ccstoreui/v1/products?categoryId=%s&includeChildren=true&limit=50", baseURL, categoryID)
	//https://www.frigelar.com.br/ccstoreui/v1/products?categoryId=ar-condicionado&includeChildren=true&page=0&offset=0&limit=2

	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request for %s: %w", nextURL, err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to fetch %s: %w", nextURL, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("OCC status %d for %s", resp.StatusCode, nextURL)
		}

		var result OCCCaregoryResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode response from %s: %w", nextURL, err)
		}

		for _, p := range result.Items {
			handler(p)
		}

		// Find next page link
		nextURL = ""
		for _, link := range result.Links {
			if link.Rel == "next" {
				href := strings.TrimSpace(link.Href)
				if strings.HasPrefix(href, "/") {
					nextURL = baseURL + href
				} else {
					nextURL = href
				}
				break
			}
		}
	}

	return nil
}
