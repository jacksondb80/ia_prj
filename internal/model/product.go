package model

type RawProduct struct {
	ID         string
	ProdutoID  string
	SourceURL  string
	ImageURL   string
	Brand      string // Nova coluna estruturada
	Btus       int    // Nova coluna estruturada
	Ciclo      string
	Voltagem   string
	Tecnologia string
	Type       string
	Content    string
	SalePrice  float32 // Nova coluna estruturada
	Length     float32 // Nova coluna estruturada
	Weight     float32 // Nova coluna estruturada
	Width      float32 // Nova coluna estruturada
	Height     float32 // Nova coluna estruturada
}
