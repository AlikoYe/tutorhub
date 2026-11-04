package bot

import (
	"context"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Sender is the part of the Telegram API the app uses; *tgbot.Bot implements it.
type Sender interface {
	SendMessage(ctx context.Context, params *tgbot.SendMessageParams) (*models.Message, error)
	EditMessageText(ctx context.Context, params *tgbot.EditMessageTextParams) (*models.Message, error)
	AnswerCallbackQuery(ctx context.Context, params *tgbot.AnswerCallbackQueryParams) (bool, error)
}

// LogSender logs what it would send. Used when BOT_TOKEN is empty (dev).
type LogSender struct {
	Log *slog.Logger
}

func (s LogSender) SendMessage(ctx context.Context, p *tgbot.SendMessageParams) (*models.Message, error) {
	s.Log.InfoContext(ctx, "telegram disabled, would send",
		slog.Any("chat_id", p.ChatID), slog.String("text", p.Text), slog.String("buttons", buttons(p.ReplyMarkup)))
	return &models.Message{}, nil
}

func (s LogSender) EditMessageText(ctx context.Context, p *tgbot.EditMessageTextParams) (*models.Message, error) {
	s.Log.InfoContext(ctx, "telegram disabled, would edit",
		slog.Any("chat_id", p.ChatID), slog.Int("message_id", p.MessageID), slog.String("text", p.Text))
	return &models.Message{}, nil
}

func (s LogSender) AnswerCallbackQuery(ctx context.Context, p *tgbot.AnswerCallbackQueryParams) (bool, error) {
	s.Log.InfoContext(ctx, "telegram disabled, would answer callback", slog.String("text", p.Text))
	return true, nil
}

// buttons lists inline button labels for logs and tests, e.g. "✅ Confirm | ❌ Reject".
func buttons(markup models.ReplyMarkup) string {
	kb, ok := markup.(*models.InlineKeyboardMarkup)
	if !ok || kb == nil {
		return ""
	}
	var labels []string
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			labels = append(labels, b.Text)
		}
	}
	return strings.Join(labels, " | ")
}
