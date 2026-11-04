package bot

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
)

// SpecialistSubmitted asks every admin to verify a new profile.
func (n *Notifier) SpecialistSubmitted(ctx context.Context, adminIDs []int64, p specialist.Private) {
	kb := keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenQueue, "/admin")})
	for _, id := range adminIDs {
		n.Send(ctx, id, "specialist_submitted", textSubmittedForAdmin(p), kb, slog.Int64("specialist_id", p.ID))
	}
}

// SpecialistApproved tells the teacher their profile is live.
func (n *Notifier) SpecialistApproved(ctx context.Context, p specialist.Private) {
	kb := keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenEditor, "/teach")})
	n.Send(ctx, p.UserID, "specialist_approved", textApproved(p, n.loc), kb, slog.Int64("specialist_id", p.ID))
}

// SpecialistRejected tells the teacher what to fix.
func (n *Notifier) SpecialistRejected(ctx context.Context, p specialist.Private, reason string) {
	kb := keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenEditor, "/teach")})
	n.Send(ctx, p.UserID, "specialist_rejected", textRejected(reason), kb, slog.Int64("specialist_id", p.ID))
}

// Specialists is what the bot needs from the specialist module.
type Specialists interface {
	Profile(ctx context.Context, v specialist.Viewer, id int64) (specialist.Public, error)
	MySpecialist(ctx context.Context, userID int64) (any, error)
}

// RegisterSpecialists adds /teach and the /start t_<id> personal links.
func (b *Bot) RegisterSpecialists(svc Specialists) {
	b.OnCommand("teach", func(ctx context.Context, msg *models.Message, _ string) {
		var sum *specialist.Summary
		if msg.From != nil {
			v, err := svc.MySpecialist(ctx, msg.From.ID)
			if err != nil {
				b.log.ErrorContext(ctx, "teach: load profile", slog.Any("err", err))
				b.Reply(ctx, msg.Chat.ID, textTryAgain, nil)
				return
			}
			sum, _ = v.(*specialist.Summary)
		}
		link := ""
		if sum != nil {
			link = b.links.SpecialistLink(sum.ID)
		}
		b.Reply(ctx, msg.Chat.ID, textTeach(sum, link), keyboard([]*models.InlineKeyboardButton{b.links.AppButton(textOpenEditor, "/teach")}))
	})

	b.OnCommand("start", func(ctx context.Context, msg *models.Message, args string) {
		if id, ok := parseSpecialistPayload(args); ok {
			var viewerID int64
			if msg.From != nil {
				viewerID = msg.From.ID
			}
			p, err := svc.Profile(ctx, specialist.Viewer{UserID: viewerID}, id)
			if err == nil {
				path := "/specialists/" + strconv.FormatInt(id, 10)
				b.Reply(ctx, msg.Chat.ID, textSpecialistLink(p.DisplayName), keyboard([]*models.InlineKeyboardButton{b.links.AppButton(textOpenProfile, path)}))
				return
			}
		}
		b.handleStart(ctx, msg, args)
	})
}

func parseSpecialistPayload(args string) (int64, bool) {
	raw, ok := strings.CutPrefix(args, StartSpecialistPrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}
