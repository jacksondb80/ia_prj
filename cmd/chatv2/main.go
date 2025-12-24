package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	openai "github.com/sashabaranov/go-openai"

	"iaprj/internal/chat"
	"iaprj/internal/config"
	"iaprj/internal/repository"
)

func main() {
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Erro ao conectar no Postgres (pgxpool): %v", err)
	}
	err = pool.Ping(context.Background())
	if err != nil {
		log.Fatalf("Erro ao conectar no Postgres (pgxpool): %v", err)
	}
	defer pool.Close()

	vectorRepo := &repository.VectorRepository{DB: pool}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	sessionStore := &chat.SessionStore{
		Client: redisClient,
	}

	client := openai.NewClient(cfg.OpenAIKey)

	// Usa o HandlerV2 que implementa a lógica de busca por metadados primeiro
	http.Handle(
		"/chat",
		chat.HandlerV2(vectorRepo, sessionStore, client),
	)

	http.HandleFunc("/view", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, "./views/index2.html")
	})

	log.Println("Chat IA V2 (Metadata First) rodando :8090")
	http.ListenAndServe(":8090", nil)
}
