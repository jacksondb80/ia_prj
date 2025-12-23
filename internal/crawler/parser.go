package crawler

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ParseProduct(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	var content []string
	doc.Find("h1, h2, p, li").Each(func(_ int, s *goquery.Selection) {
		content = append(content, strings.TrimSpace(s.Text()))
	})

	return strings.Join(content, "\n"), nil
}
