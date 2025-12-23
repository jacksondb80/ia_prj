package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"iaprj/internal/model"
)

const (
	sessionTTL   = 30 * time.Minute
	historyLimit = 6
)

type SessionStore struct {
	Client *redis.Client
}

func (s *SessionStore) Get(sessionID string) ([]model.ChatMessage, error) {
	ctx := context.Background()

	val, err := s.Client.Get(ctx, sessionID).Result()
	if err != nil {
		return nil, nil
	}

	var msgs []model.ChatMessage
	json.Unmarshal([]byte(val), &msgs)

	return msgs, nil
}

func (s *SessionStore) Append(
	sessionID string,
	msg model.ChatMessage,
) error {

	ctx := context.Background()

	history, _ := s.Get(sessionID)
	history = append(history, msg)

	if len(history) > historyLimit {
		history = history[len(history)-historyLimit:]
	}

	b, _ := json.Marshal(history)

	return s.Client.Set(ctx, sessionID, b, sessionTTL).Err()
}
