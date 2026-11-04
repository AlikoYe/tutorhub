// Package bot runs the Telegram bot (long polling): commands, the menu button, notifications
// and inline-button callbacks. Every user-facing text is in texts.go.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// StartSpecialistPrefix marks a /start payload that opens a specialist profile: t_<id>.
const StartSpecialistPrefix = "t_"

// CommandHandler handles "/<command> <args>" in a private chat.
type CommandHandler func(ctx context.Context, msg *models.Message, args string)

// CallbackHandler handles an inline-button press whose data starts with the registered prefix.
type CallbackHandler func(ctx context.Context, q *models.CallbackQuery)

// Bot dispatches Telegram updates to registered handlers. Run needs a real client;
// HandleUpdate works with any Sender, so tests drive it directly.
type Bot struct {
	api       *tgbot.Bot
	send      Sender
	links     Links
	log       *slog.Logger
	commands  map[string]CommandHandler
	callbacks map[string]CallbackHandler
}

// New creates the Telegram client.
func New(token string, links Links, log *slog.Logger) (*Bot, error) {
	b := NewWithSender(nil, links, log)
	api, err := tgbot.New(token,
		tgbot.WithDefaultHandler(func(ctx context.Context, _ *tgbot.Bot, u *models.Update) { b.HandleUpdate(ctx, u) }),
		tgbot.WithErrorsHandler(func(err error) {
			log.Warn("telegram bot error", slog.Any("err", err))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	b.api = api
	b.send = api
	return b, nil
}

// NewWithSender builds a Bot without a Telegram client (tests, dev without a token).
func NewWithSender(send Sender, links Links, log *slog.Logger) *Bot {
	b := &Bot{send: send, links: links, log: log, commands: map[string]CommandHandler{}, callbacks: map[string]CallbackHandler{}}
	b.OnCommand("start", b.handleStart)
	b.OnCommand("help", b.handleHelp)
	return b
}

// Sender returns the client used to send messages.
func (b *Bot) Sender() Sender { return b.send }

// Links returns the bot's link builder.
func (b *Bot) Links() Links { return b.links }

// OnCommand registers a handler for /name. A later registration replaces an earlier one.
func (b *Bot) OnCommand(name string, h CommandHandler) { b.commands[name] = h }

// OnCallback registers a handler for callback data starting with prefix (e.g. "ls:").
func (b *Bot) OnCallback(prefix string, h CallbackHandler) { b.callbacks[prefix] = h }

// Run sets the menu button and long-polls for updates until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	if url := b.links.App("/"); url != "" {
		_, err := b.api.SetChatMenuButton(ctx, &tgbot.SetChatMenuButtonParams{
			MenuButton: models.MenuButtonWebApp{
				Type:   models.MenuButtonTypeWebApp,
				Text:   textMenuButton,
				WebApp: models.WebAppInfo{URL: url},
			},
		})
		if err != nil {
			b.log.WarnContext(ctx, "set chat menu button", slog.Any("err", err))
		}
	}
	b.log.Info("telegram bot started (long polling)")
	b.api.Start(ctx)
	b.log.Info("telegram bot stopped")
	return nil
}

// HandleUpdate routes one update to a command or callback handler.
func (b *Bot) HandleUpdate(ctx context.Context, u *models.Update) {
	switch {
	case u.CallbackQuery != nil:
		q := u.CallbackQuery
		for prefix, h := range b.callbacks {
			if strings.HasPrefix(q.Data, prefix) {
				h(ctx, q)
				return
			}
		}
		b.AnswerCallback(ctx, q, textTryAgain, true)
	case u.Message != nil && u.Message.Chat.Type == models.ChatTypePrivate:
		name, args, ok := parseCommand(u.Message.Text)
		if !ok {
			return
		}
		if h, found := b.commands[name]; found {
			h(ctx, u.Message, args)
			return
		}
		b.Reply(ctx, u.Message.Chat.ID, textUnknownCommand, nil)
	default:
		b.log.DebugContext(ctx, "unhandled telegram update", slog.Int64("update_id", u.ID))
	}
}

// parseCommand splits "/name@bot args" into name and args.
func parseCommand(text string) (name, args string, ok bool) {
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	head, rest, _ := strings.Cut(text[1:], " ")
	head, _, _ = strings.Cut(head, "@")
	if head == "" {
		return "", "", false
	}
	return strings.ToLower(head), strings.TrimSpace(rest), true
}

// Reply sends a message synchronously (inside an update handler); errors are logged.
func (b *Bot) Reply(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) {
	params := &tgbot.SendMessageParams{ChatID: chatID, Text: text}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	if _, err := b.send.SendMessage(ctx, params); err != nil {
		b.log.WarnContext(ctx, "bot reply", slog.Int64("chat_id", chatID), slog.Any("err", err))
	}
}

// AnswerCallback stops the button spinner, optionally as an alert.
func (b *Bot) AnswerCallback(ctx context.Context, q *models.CallbackQuery, text string, alert bool) {
	_, err := b.send.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{CallbackQueryID: q.ID, Text: text, ShowAlert: alert})
	if err != nil {
		b.log.WarnContext(ctx, "answer callback", slog.Any("err", err))
	}
}

// EditCallbackMessage replaces the text of the message carrying the pressed button and drops its buttons.
func (b *Bot) EditCallbackMessage(ctx context.Context, q *models.CallbackQuery, text string) {
	msg := q.Message.Message
	if msg == nil || text == "" {
		return
	}
	if _, err := b.send.EditMessageText(ctx, &tgbot.EditMessageTextParams{ChatID: msg.Chat.ID, MessageID: msg.ID, Text: text}); err != nil {
		b.log.WarnContext(ctx, "edit callback message", slog.Any("err", err))
	}
}

// CallbackMessageText is the text of the message carrying the pressed button, or "".
func CallbackMessageText(q *models.CallbackQuery) string {
	if q.Message.Message == nil {
		return ""
	}
	return q.Message.Message.Text
}

func (b *Bot) handleStart(ctx context.Context, msg *models.Message, args string) {
	b.Reply(ctx, msg.Chat.ID, textWelcome, keyboard([]*models.InlineKeyboardButton{b.links.AppButton(textOpenApp, "/")}))
}

func (b *Bot) handleHelp(ctx context.Context, msg *models.Message, _ string) {
	b.Reply(ctx, msg.Chat.ID, textHelp, keyboard([]*models.InlineKeyboardButton{b.links.AppButton(textOpenApp, "/")}))
}
