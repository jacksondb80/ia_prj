package repository

import (
	"context"
	_ "database/sql"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VectorResult struct {
	ProdutoID  string
	SourceURL  string
	ImageURL   string
	Brand      string
	Btus       int
	Ciclo      string
	Voltagem   string
	Tecnologia string
	Type       string
	Content    string
	Score      float64
	SalePrice  float32
	Length     float32
	Weight     float32
	Width      float32
	Height     float32
}

type VectorRepository struct {
	DB *pgxpool.Pool
}

func (r *VectorRepository) Save(
	produtoID, sourceURL, imageURL, brand string, btus int, ciclo string, voltagem string, tecnologia string, productType string, content string,
	embedding []float32, salesPrice float32, length float32, weight float32, width float32, height float32,
) error {

	// converte []float32 para "[v1,v2,...]" (pgvector espera colchetes)
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 64)
	}
	embStr := "[" + strings.Join(parts, ",") + "]"

	// Remove sequências de bytes inválidas para evitar erro "invalid byte sequence for encoding UTF8"
	validContent := strings.ToValidUTF8(content, "")

	_, err := r.DB.Exec(context.Background(), `
		INSERT INTO product_knowledge
		(id, produto_id, source_url, image_url, brand, btus, ciclo, voltagem, tecnologia, type, content, embedding, sale_price, length, weight, width, height)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, uuid.New(), produtoID, sourceURL, imageURL, brand, btus, ciclo, voltagem, tecnologia, productType, validContent, embStr, salesPrice, length, weight, width, height)

	return err
}

func (r *VectorRepository) SearchSimilar(
	embedding []float32,
	minScore float64,
	limit int,
	preferredBrands map[string]float64,
	targetBTU int,
	filters map[string]string,
) ([]VectorResult, error) {

	// converte []float32 para "[v1,v2,...]" (pgvector espera colchetes)
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 64)
	}
	embStr := "[" + strings.Join(parts, ",") + "]"

	// --- Geração de Query Dinâmica para pesos de marcas ---
	params := []interface{}{embStr, minScore, limit}
	paramIndex := 4 // $1, $2, $3 já estão em uso

	whereClause := "1 - (embedding <=> $1) > $2"
	if targetBTU > 0 {
		minBTU := int(float64(targetBTU) * 0.8)
		maxBTU := int(float64(targetBTU) * 1.2)
		// Permite o alvo dentro do range (+/- 20%) OU produtos onde a extração falhou (0)
		whereClause += fmt.Sprintf(" AND (btus >= $%d AND btus <= $%d OR btus = 0)", paramIndex, paramIndex+1)
		params = append(params, minBTU, maxBTU)
		paramIndex += 2
	}

	// Adiciona filtros estruturados
	for key, value := range filters {
		if key == "type_exclude" {
			whereClause += fmt.Sprintf(" AND type NOT ILIKE $%d", paramIndex)
			params = append(params, "%"+value+"%")
		} else {
			whereClause += fmt.Sprintf(" AND %s ILIKE $%d", key, paramIndex)
			params = append(params, "%"+value+"%") // Use full wildcard for flexibility
		}
		paramIndex++
	}

	// Ordena as marcas para ter uma string de query determinística (bom para cache de query)
	brands := make([]string, 0, len(preferredBrands))
	for brand := range preferredBrands {
		brands = append(brands, brand)
	}
	sort.Strings(brands)

	var caseBuilder strings.Builder
	// Agora usamos a coluna 'brand' estruturada, o que é MUITO mais rápido e preciso
	// do que fazer ILIKE no texto inteiro.
	caseBuilder.WriteString("CASE ")
	for _, brand := range brands {
		weight := preferredBrands[brand]
		caseBuilder.WriteString(fmt.Sprintf("WHEN brand ILIKE $%d THEN %f ", paramIndex, weight))
		params = append(params, "%"+brand+"%") // Passa a marca com wildcard para garantir match
		paramIndex++
	}
	caseBuilder.WriteString("ELSE 1.0 END")

	orderByClause := fmt.Sprintf("(embedding <=> $1) * (%s)", caseBuilder.String())

	query := fmt.Sprintf(`
		SELECT produto_id, source_url, image_url, brand, btus, ciclo, voltagem, tecnologia, type, content, score
		FROM (
			SELECT DISTINCT ON (produto_id) produto_id, source_url, image_url, brand, btus, ciclo, voltagem, tecnologia, type, content,
			       1 - (embedding <=> $1) AS score,
			       %s AS sort_val
			FROM product_knowledge		
			WHERE stock = 1 AND %s
			ORDER BY produto_id, sort_val ASC
		) sub
		ORDER BY sort_val ASC
		LIMIT $3
		`, orderByClause, whereClause)

	log.Printf("[Repository] SearchSimilar Query: %s | Params: %v", query, params)

	rows, err := r.DB.Query(
		context.Background(),
		query,
		params...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []VectorResult

	for rows.Next() {
		var r VectorResult
		if err := rows.Scan(&r.ProdutoID, &r.SourceURL, &r.ImageURL, &r.Brand, &r.Btus, &r.Ciclo, &r.Voltagem, &r.Tecnologia, &r.Type, &r.Content, &r.Score); err != nil {
			continue // Pula linhas com erro de scan
		}
		res = append(res, r)
	}

	return res, nil
}

// SearchByBrand busca produtos de uma marca específica, aplicando filtro de BTUs se necessário.
// Diferente do SearchSimilar, não aplica corte por score mínimo (minScore), garantindo que a marca apareça.
func (r *VectorRepository) SearchByBrand(
	brand string,
	targetBTU int,
	limit int,
	filters map[string]string,
	embedding []float32,
) ([]VectorResult, error) {

	// converte []float32 para "[v1,v2,...]" (pgvector espera colchetes)
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 64)
	}
	embStr := "[" + strings.Join(parts, ",") + "]"

	params := []interface{}{embStr, "%" + brand + "%", limit}
	whereClause := "brand ILIKE $2"
	paramIndex := 4

	if targetBTU > 0 {
		minBTU := int(float64(targetBTU) * 0.8)
		maxBTU := int(float64(targetBTU) * 1.2)
		whereClause += fmt.Sprintf(" AND (btus >= $%d AND btus <= $%d OR btus = 0)", paramIndex, paramIndex+1)
		params = append(params, minBTU, maxBTU)
		paramIndex += 2
	}

	// Adiciona filtros estruturados
	for key, value := range filters {
		if key == "type_exclude" {
			whereClause += fmt.Sprintf(" AND type NOT ILIKE $%d", paramIndex)
			params = append(params, "%"+value+"%")
		} else {
			whereClause += fmt.Sprintf(" AND %s ILIKE $%d", key, paramIndex)
			params = append(params, "%"+value+"%")
		}
		paramIndex++
	}

	query := fmt.Sprintf(`
		SELECT produto_id, source_url, image_url, brand, btus, ciclo, voltagem, tecnologia, type, content, 
		       1 - (embedding <=> $1) AS score,sale_price, length, weight, width, height
		FROM product_knowledge
		WHERE  stock = 1 AND  %s
		ORDER BY embedding <=> $1 ASC
		LIMIT $3
	`, whereClause)

	rows, err := r.DB.Query(context.Background(), query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []VectorResult
	for rows.Next() {
		var r VectorResult
		if err := rows.Scan(&r.ProdutoID, &r.SourceURL, &r.ImageURL, &r.Brand, &r.Btus, &r.Ciclo, &r.Voltagem, &r.Tecnologia, &r.Type, &r.Content, &r.Score, &r.SalePrice, &r.Length, &r.Weight, &r.Width, &r.Height); err == nil {
			res = append(res, r)
		}
	}
	return res, nil
}

// SearchByMetadata performs a simple SQL search based on structured metadata filters.
// It does not use vector similarity.
func (r *VectorRepository) SearchByMetadata(
	targetBTU int,
	filters map[string]string,
	limit int,
) ([]VectorResult, error) {
	// Constrói cláusulas base (sem limite e sem filtro específico de marca EOS/Não-EOS por enquanto)
	var params []interface{}
	paramIndex := 1
	var whereClauses []string

	if targetBTU > 0 {
		minBTU := int(float64(targetBTU) * 0.8)
		maxBTU := int(float64(targetBTU) * 1.2)
		whereClauses = append(whereClauses, fmt.Sprintf("(btus >= $%d AND btus <= $%d)", paramIndex, paramIndex+1))
		params = append(params, minBTU, maxBTU)
		paramIndex += 2
	}

	// Adiciona filtros estruturados
	hasBrandFilter := false
	for key, value := range filters {
		if key == "brand" {
			hasBrandFilter = true
		}
		if key == "type_exclude" {
			whereClauses = append(whereClauses, fmt.Sprintf("type NOT ILIKE $%d", paramIndex))
			params = append(params, "%"+value+"%")
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("%s ILIKE $%d", key, paramIndex))
			params = append(params, "%"+value+"%")
		}
		paramIndex++
	}

	baseWhere := "1=1"
	if len(whereClauses) > 0 {
		baseWhere = strings.Join(whereClauses, " AND ")
	}

	// Função auxiliar para executar a query
	runQuery := func(extraWhere string, queryLimit int) ([]VectorResult, error) {
		if queryLimit <= 0 {
			return nil, nil
		}

		// Copia params para não afetar a próxima execução
		currentParams := make([]interface{}, len(params))
		copy(currentParams, params)
		currentParams = append(currentParams, queryLimit)

		fullWhere := baseWhere
		if extraWhere != "" {
			fullWhere += " AND " + extraWhere
		}

		query := fmt.Sprintf(`
			SELECT produto_id, source_url, image_url, brand, btus, ciclo, voltagem, tecnologia, type, content, score, sale_price, length, weight, width, height
			FROM (
				SELECT DISTINCT ON (produto_id) produto_id, source_url, image_url, brand, btus, ciclo, voltagem, tecnologia, type, content, 0 AS score, sale_price, length, weight, width, height
				FROM product_knowledge
				WHERE  stock = 1 AND  %s
				ORDER BY produto_id, btus ASC
			) sub
			ORDER BY btus ASC
			LIMIT $%d
		`, fullWhere, paramIndex)

		log.Printf("[Repository] Metadata Query: %s | Params: %v", query, currentParams)

		rows, err := r.DB.Query(context.Background(), query, currentParams...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var results []VectorResult
		for rows.Next() {
			var r VectorResult
			if err := rows.Scan(&r.ProdutoID, &r.SourceURL, &r.ImageURL, &r.Brand, &r.Btus, &r.Ciclo, &r.Voltagem, &r.Tecnologia, &r.Type, &r.Content, &r.Score, &r.SalePrice, &r.Length, &r.Weight, &r.Width, &r.Height); err == nil {
				results = append(results, r)
			}
		}
		return results, nil
	}

	// Se o usuário informou uma marca, fazemos a busca direta sem forçar os 2 primeiros EOS
	if hasBrandFilter {
		return runQuery("", limit)
	}

	// 1. Busca até 2 produtos EOS
	eosResults, err := runQuery("brand ILIKE '%EOS%'", 2)
	if err != nil {
		return nil, err
	}

	// 2. Busca o restante (Não EOS)
	remainingLimit := limit - len(eosResults)
	otherResults, err := runQuery("brand NOT ILIKE '%EOS%'", remainingLimit)
	if err != nil {
		return nil, err
	}

	// Combina os resultados
	return append(eosResults, otherResults...), nil
}

// GetChunksByProductID retrieves all content chunks for a given product ID, ordered to reconstruct the document.
func (r *VectorRepository) GetChunksByProductID(productID string) ([]string, error) {
	query := `SELECT content FROM product_knowledge WHERE produto_id = $1 ORDER BY created_at ASC`
	rows, err := r.DB.Query(context.Background(), query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contents []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			contents = append(contents, content)
		}
	}
	return contents, nil
}
