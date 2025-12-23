package chat

import (
	"context"

	openai "github.com/sashabaranov/go-openai"

	"iaprj/internal/model"
)

func CallLLM(
	client *openai.Client,
	systemPrompt string,
	contextText string,
	history []model.ChatMessage,
	userMessage string,
) (string, error) {

	var messages []openai.ChatCompletionMessage

	// system
	messages = append(messages,
		openai.ChatCompletionMessage{
			Role:    "system",
			Content: systemPrompt,
		},
		openai.ChatCompletionMessage{
			Role:    "system",
			Content: "CONTEXTO TÉCNICO:\n" + contextText,
		},
	)

	// histórico
	for _, m := range history {
		messages = append(messages,
			openai.ChatCompletionMessage{
				Role:    m.Role,
				Content: m.Content,
			},
		)
	}

	// nova pergunta
	messages = append(messages,
		openai.ChatCompletionMessage{
			Role:    "user",
			Content: userMessage,
		},
	)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       openai.GPT4oMini,
			Messages:    messages,
			Temperature: 0.3,
		},
	)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
