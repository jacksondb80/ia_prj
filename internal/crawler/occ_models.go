package crawler

type OCCProductResponse struct {
	Items []OCCProduct `json:"items"`
}

// OCCCaregoryResponse defines the structure of the category API response.
type OCCCaregoryResponse struct {
	TotalResults int          `json:"totalResults"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
	Links        []OCCLink    `json:"links"`
	Items        []OCCProduct `json:"items"`
}

// OCCLink defines a link in the API response.
type OCCLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

type OCCProduct struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"displayName"`
	Description        string `json:"description"`
	LongDesc           string `json:"longDescription"`
	Brand              string `json:"brand"`
	Route              string `json:"route"`
	Btus               string `json:"x_quantidadeDeBTUs"`
	Ciclo              string `json:"x_ciclo"`
	Tecnologia         string `json:"x_tecnologia"`
	Serpentina         string `json:"x_serpentina"`
	Fase               string `json:"x_fase"`
	Warranty           string `json:"x_warranty"`
	GarantiaCompressor string `json:"x_garantiaDoCompressor"`
	Voltagem           string `json:"x_tension"`
	Categoria          string `json:"x_categorias"`
	CategoriaExtra     string `json:"x_categoria"`
	Tipo               string `json:"type"`
	Variants           string `json:"x_variants"`
	MobileDescription  string `json:"x_descrioLongMobile"`
	Url                string `json:"url"`
	Slug               string `json:"seoUrlSlugDerived"`
	PrimaryImg         string `json:"primaryFullImageURL"`
}
