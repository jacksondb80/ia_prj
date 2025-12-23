package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	RedisURL        string
	OpenAIKey       string
	MetricsPort     string
	WorkerCount     int
	RehydrationMode string
}

func Load() *Config {
	// Carrega .env da raiz do projeto
	_ = godotenv.Load("../../.env")
	// Se não encontrar, tenta no diretório atual
	_ = godotenv.Load()
	return &Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		OpenAIKey:       os.Getenv("OPENAI_API_KEY"),
		MetricsPort:     getEnv("METRICS_PORT", "9090"),
		WorkerCount:     5,
		RehydrationMode: getEnv("REHYDRATION_MODE", "first"), // Pode ser "full" ou "first"
	}
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
