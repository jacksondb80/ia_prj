package db

import (
	"context"
	"database/sql"
	"log"

	"github.com/jackc/pgx/v5"
	_ "github.com/lib/pq"
)

func New(url string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", url)
	if err != nil {
		log.Fatal(err)
	}
	err = conn.Ping()
	if err != nil {
		log.Fatal(err)
	}
	return conn, err
}

func NewPgx(url string) (*pgx.Conn, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	err = conn.Ping(ctx)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return conn, err
}
