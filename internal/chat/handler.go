package chat

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"iaprj/internal/model"
	"iaprj/internal/repository"
)

var ctx = context.Background()

type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type ChatResponse struct {
	Answer string `json:"answer"`
}

func Handler(
	vectorRepo *repository.VectorRepository,
	session *SessionStore,
	client *openai.Client,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		history, _ := session.Get(req.SessionID)

		contextText, err := buildContext(req, history, vectorRepo, session)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		answer, err := CallLLM(
			client,
			SystemPrompt(),
			contextText,
			history,
			req.Message,
		)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		log.Printf("Resposta da IA: %s", answer)

		// salva histórico
		session.Append(req.SessionID, model.ChatMessage{
			Role:    "user",
			Content: req.Message,
		})
		session.Append(req.SessionID, model.ChatMessage{
			Role:    "assistant",
			Content: answer,
		})

		json.NewEncoder(w).Encode(ChatResponse{
			Answer: answer,
		})
	}
}

// --- SessionStore Methods ---

func (s *SessionStore) SetCalculatedBTU(sessionID string, btu int) {
	key := "btu:" + sessionID
	s.Client.Set(ctx, key, btu, 20*time.Minute)
}

func (s *SessionStore) GetCalculatedBTU(sessionID string) (int, error) {
	key := "btu:" + sessionID
	val, err := s.Client.Get(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	btu, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}
	// Estende a expiração sempre que o valor é lido
	s.Client.Expire(ctx, key, 20*time.Minute)
	return btu, nil
}
