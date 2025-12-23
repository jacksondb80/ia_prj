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
}
