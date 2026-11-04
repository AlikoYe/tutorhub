package main

import (
	"context"
	"strings"
	"sync"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// recordingSender records every Telegram call the app makes.
type recordingSender struct {
	mu       sync.Mutex
	sent     []*tgbot.SendMessageParams
	edited   []*tgbot.EditMessageTextParams
	answered []*tgbot.AnswerCallbackQueryParams
}

func (s *recordingSender) SendMessage(_ context.Context, p *tgbot.SendMessageParams) (*models.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, p)
	return &models.Message{}, nil
}

func (s *recordingSender) EditMessageText(_ context.Context, p *tgbot.EditMessageTextParams) (*models.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edited = append(s.edited, p)
	return &models.Message{}, nil
}

func (s *recordingSender) AnswerCallbackQuery(_ context.Context, p *tgbot.AnswerCallbackQueryParams) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answered = append(s.answered, p)
	return true, nil
}

// messagesTo returns the texts sent to chatID so far.
func (s *recordingSender) messagesTo(chatID int64) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, p := range s.sent {
		if id, ok := p.ChatID.(int64); ok && id == chatID {
			out = append(out, p.Text)
		}
	}
	return out
}

// countContaining counts messages to chatID containing substr.
func (s *recordingSender) countContaining(chatID int64, substr string) int {
	n := 0
	for _, m := range s.messagesTo(chatID) {
		if strings.Contains(m, substr) {
			n++
		}
	}
	return n
}
