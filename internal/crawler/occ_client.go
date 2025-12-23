package crawler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

func FetchProductsByIDs(ids []string) ([]OCCProduct, error) {
	idList := strings.Join(ids, ",")

	url := fmt.Sprintf(
		"https://www.frigelar.com.br/ccstoreui/v1/products?productIds=%s&pageSize=%d",
		idList,
		len(ids),
	)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OCC status %d", resp.StatusCode)
	}

	var result OCCProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}
