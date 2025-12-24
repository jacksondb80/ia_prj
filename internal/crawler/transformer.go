package crawler

import (
	"fmt"
	"regexp"
	"strings"
)

func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

func ProductToText(p *OCCProduct) string {
	var sb strings.Builder

	// 1. Título principal - a informação mais importante para o embedding.
	sb.WriteString(p.DisplayName + "\n\n")

	// 2. Descrições de marketing (MOVIDO PARA CIMA)
	if p.LongDesc != "" {
		sb.WriteString("Descrição Detalhada:\n" + stripHTML(p.LongDesc) + "\n\n")
	} else if p.Description != "" { // Fallback para descrição curta
		sb.WriteString("Descrição:\n" + stripHTML(p.Description) + "\n\n")
	}

	// 2. Bloco de especificações técnicas chave - para busca assertiva
	sb.WriteString("--- Especificações Técnicas ---\n")
	if p.Brand != "" {
		sb.WriteString("Marca: " + p.Brand + "\n")
	}
	if p.Btus != "" {
		sb.WriteString("Capacidade: " + p.Btus + " BTUs\n")
	}
	if p.Ciclo != "" {
		sb.WriteString("Ciclo: " + p.Ciclo + "\n")
	}
	if p.Tecnologia != "" {
		sb.WriteString("Tecnologia: " + p.Tecnologia + "\n")
	}
	if p.Voltagem != "" {
		sb.WriteString("Voltagem: " + p.Voltagem + "\n")
	}
	if p.Fase != "" {
		sb.WriteString("Fase: " + p.Fase + "\n")
	}
	if p.Serpentina != "" {
		sb.WriteString("Serpentina: " + p.Serpentina + "\n")
	}
	if p.Categoria != "" {
		sb.WriteString("Categoria: " + p.Categoria + "\n")
	}
	if p.Tipo != "" {
		sb.WriteString("Tipo: " + p.Tipo + "\n")
	}

	if p.Length > 0 {
		sb.WriteString("Comprimento: " + fmt.Sprintf("%g", p.Length) + "\n")
	}
	if p.Width > 0 {
		sb.WriteString("Largura: " + fmt.Sprintf("%g", p.Width) + "\n")
	}
	if p.Height > 0 {
		sb.WriteString("Altura: " + fmt.Sprintf("%g", p.Height) + "\n")
	}
	if p.Weight > 0 {
		sb.WriteString("Peso: " + fmt.Sprintf("%g", p.Weight) + "\n")
	}
	sb.WriteString("-----------------------------\n\n")
	// 3. Preço de Venda
	if p.SalePrice > 0 {
		sb.WriteString("Preço de Venda: " + fmt.Sprintf("%g", p.SalePrice) + "\n")
	}

	// 4. Metadados para extração posterior (não para embedding direto, mas precisa estar no texto)
	p.Url = "https://www.frigelar.com.br/" + p.Slug + "/p/" + p.ID
	sb.WriteString("URL: " + p.Url + "\n")

	if p.PrimaryImg != "" {
		sb.WriteString("Imagem Principal: https://www.frigelar.com.br/" + p.PrimaryImg + "\n")
	}

	return sb.String()
}
