package embeddings

import (
	"context"
	"iaprj/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

var cfg = config.Load()
var client = openai.NewClient(cfg.OpenAIKey)

func Embed(text string) ([]float32, error) {
	resp, err := client.CreateEmbeddings(
		context.Background(),
		openai.EmbeddingRequest{
			Model: "text-embedding-3-small",
			Input: text,
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}
