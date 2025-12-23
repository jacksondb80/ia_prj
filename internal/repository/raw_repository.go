package repository

import (
	"database/sql"
	"iaprj/internal/model"
)

type RawRepository struct {
	DB *sql.DB
}

func (r *RawRepository) Save(p model.RawProduct) error {
	var exists bool
	err := r.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM product_raw_knowledge WHERE produto_id = $1)", p.ProdutoID).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		_, err = r.DB.Exec(`
			UPDATE product_raw_knowledge
			SET source_url = $1, raw_content = $2, sync_status = 'S'
			WHERE produto_id = $3
		`, p.SourceURL, p.Content, p.ProdutoID)
	} else {
		_, err = r.DB.Exec(`
			INSERT INTO product_raw_knowledge
			(id, produto_id, source_url, raw_content, sync_status)
			VALUES ($1, $2, $3, $4, 'S')
		`, p.ID, p.ProdutoID, p.SourceURL, p.Content)
	}

	return err
}

func (r *RawRepository) List() ([]model.RawProduct, error) {
	rows, err := r.DB.Query(`
		SELECT id, produto_id, source_url, raw_content
		FROM product_raw_knowledge
		WHERE sync_status = 'S'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.RawProduct
	for rows.Next() {
		var p model.RawProduct
		rows.Scan(&p.ID, &p.ProdutoID, &p.SourceURL, &p.Content)
		list = append(list, p)
	}

	return list, nil
}

func (r *RawRepository) MarkAsProcessed(produtoID string) error {
	_, err := r.DB.Exec(`
		UPDATE product_raw_knowledge
		SET sync_status = 'N'
		WHERE produto_id = $1
	`, produtoID)
	return err
}
