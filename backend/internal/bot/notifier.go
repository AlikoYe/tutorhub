package bot

import (
	"context"
	"log/slog"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// SendTimeout bounds one background Telegram send.
const SendTimeout = 10 * time.Second

// Notifier sends messages through a Sender in the background, so Telegram latency never
// slows a request. Send errors are logged and swallowed.
type Notifier struct {
	send    Sender
	links   Links
	log     *slog.Logger
	now     func() time.Time
	loc     *time.Location
	timeout time.Duration
	wg      sync.WaitGroup
}

func NewNotifier(send Sender, links Links, loc *time.Location, log *slog.Logger) *Notifier {
	if loc == nil {
		loc = time.UTC
	}
	return &Notifier{send: send, links: links, log: log, now: time.Now, loc: loc, timeout: SendTimeout}
}

// Wait blocks until every background send has finished (shutdown, tests).
func (n *Notifier) Wait() {
	n.wg.Wait()
}

// Send delivers text to chatID in the background. event and attrs only go to the logs.
func (n *Notifier) Send(ctx context.Context, chatID int64, event, text string, markup models.ReplyMarkup, attrs ...slog.Attr) {
	if chatID == 0 {
		n.log.InfoContext(ctx, "no recipient for notification", slog.String("event", event))
		return
	}
	params := &tgbot.SendMessageParams{ChatID: chatID, Text: text}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	// Detach from the request: it may finish (and cancel ctx) before Telegram answers.
	ctx = context.WithoutCancel(ctx)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		ctx, cancel := context.WithTimeout(ctx, n.timeout)
		defer cancel()
		if _, err := n.send.SendMessage(ctx, params); err != nil {
			args := []any{slog.String("event", event), slog.Int64("chat_id", chatID), slog.Any("err", err)}
			for _, a := range attrs {
				args = append(args, a)
			}
			n.log.WarnContext(ctx, "telegram send failed", args...)
		}
	}()
}
