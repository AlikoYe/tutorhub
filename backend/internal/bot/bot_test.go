package bot

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

var testLinks = Links{WebAppURL: "https://app.example.com/", BotUsername: "tutorhub_bot"}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		in, name, args string
		ok             bool
	}{
		{"/start", "start", "", true},
		{"/start t_42", "start", "t_42", true},
		{"/Teach@tutorhub_bot  now ", "teach", "now", true},
		{"hello", "", "", false},
		{"/", "", "", false},
	}
	for _, tt := range tests {
		name, args, ok := parseCommand(tt.in)
		if name != tt.name || args != tt.args || ok != tt.ok {
			t.Errorf("parseCommand(%q) = %q, %q, %v", tt.in, name, args, ok)
		}
	}
}

func TestStartRepliesWithAppButton(t *testing.T) {
	send := &fakeSender{}
	b := NewWithSender(send, testLinks, slog.New(slog.DiscardHandler))
	b.HandleUpdate(context.Background(), privateMessage(7, "/start"))

	if len(send.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(send.sent))
	}
	p := send.sent[0]
	kb, ok := p.ReplyMarkup.(*models.InlineKeyboardMarkup)
	if p.ChatID != int64(7) || !ok || kb.InlineKeyboard[0][0].WebApp.URL != "https://app.example.com/" {
		t.Fatalf("unexpected reply %+v", p)
	}
}

func TestStartWithoutWebAppHasNoButtons(t *testing.T) {
	send := &fakeSender{}
	b := NewWithSender(send, Links{}, slog.New(slog.DiscardHandler))
	b.HandleUpdate(context.Background(), privateMessage(7, "/start"))
	if len(send.sent) != 1 || send.sent[0].ReplyMarkup != nil {
		t.Fatalf("want one reply without markup, got %+v", send.sent)
	}
}

func TestUnknownCommandAndPlainText(t *testing.T) {
	send := &fakeSender{}
	b := NewWithSender(send, testLinks, slog.New(slog.DiscardHandler))
	b.HandleUpdate(context.Background(), privateMessage(7, "/nope"))
	b.HandleUpdate(context.Background(), privateMessage(7, "just text"))
	if len(send.sent) != 1 || send.sent[0].Text != textUnknownCommand {
		t.Fatalf("got %+v", send.sent)
	}
}

func TestCallbackDispatch(t *testing.T) {
	send := &fakeSender{}
	b := NewWithSender(send, testLinks, slog.New(slog.DiscardHandler))
	var got string
	b.OnCallback("x:", func(_ context.Context, q *models.CallbackQuery) { got = q.Data })

	b.HandleUpdate(context.Background(), &models.Update{CallbackQuery: &models.CallbackQuery{ID: "1", Data: "x:5"}})
	b.HandleUpdate(context.Background(), &models.Update{CallbackQuery: &models.CallbackQuery{ID: "2", Data: "zz"}})
	if got != "x:5" {
		t.Fatalf("handler got %q", got)
	}
	if len(send.answered) != 1 || !send.answered[0].ShowAlert {
		t.Fatalf("unknown callback should be answered with an alert, got %+v", send.answered)
	}
}

func TestLinks(t *testing.T) {
	if got := testLinks.App("/teach"); got != "https://app.example.com/teach" {
		t.Errorf("App = %q", got)
	}
	if got := testLinks.SpecialistLink(12); got != "https://t.me/tutorhub_bot?start=t_12" {
		t.Errorf("SpecialistLink = %q", got)
	}
	if (Links{}).AppButton("x", "/") != nil || (Links{}).SpecialistLink(1) != "" {
		t.Error("links without config must be empty")
	}
}

func TestNotifierSendsInBackgroundAndSwallowsErrors(t *testing.T) {
	send := &fakeSender{fail: true}
	n := NewNotifier(send, testLinks, time.UTC, slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	n.Send(ctx, 5, "test", "hello", nil)
	cancel() // the request is over; the send must still happen
	n.Send(ctx, 0, "test", "nobody", nil)
	n.Wait()
	if len(send.sent) != 1 || !strings.Contains(send.sent[0].Text, "hello") {
		t.Fatalf("sent %+v", send.sent)
	}
}
