package bot

import (
	"context"
	"errors"
	"sync"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// fakeSender records everything the bot sends.
type fakeSender struct {
	mu       sync.Mutex
	sent     []*tgbot.SendMessageParams
	edited   []*tgbot.EditMessageTextParams
	answered []*tgbot.AnswerCallbackQueryParams
	fail     bool
}

func (f *fakeSender) SendMessage(_ context.Context, p *tgbot.SendMessageParams) (*models.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, p)
	if f.fail {
		return nil, errors.New("telegram down")
	}
	return &models.Message{}, nil
}

func (f *fakeSender) EditMessageText(_ context.Context, p *tgbot.EditMessageTextParams) (*models.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.edited = append(f.edited, p)
	return &models.Message{}, nil
}

func (f *fakeSender) AnswerCallbackQuery(_ context.Context, p *tgbot.AnswerCallbackQueryParams) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answered = append(f.answered, p)
	return true, nil
}

func privateMessage(chatID int64, text string) *models.Update {
	return &models.Update{Message: &models.Message{
		Text: text,
		Chat: models.Chat{ID: chatID, Type: models.ChatTypePrivate},
		From: &models.User{ID: chatID, FirstName: "Test"},
	}}
}
