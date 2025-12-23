package db

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	_ "github.com/lib/pq"
)

func New(url string) (*sql.DB, error) {
	return sql.Open("postgres", url)
}

func NewPgx(url string) (*pgx.Conn, error) {

	conn, err := pgx.Connect(context.Background(), url)
	return conn, err
}
