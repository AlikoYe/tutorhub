package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
)

var reportReasons = map[string]string{
	"fake": "фейковый профиль", "inappropriate": "неприемлемое содержание", "spam": "спам",
	"no_show": "не пришёл на урок", "other": "другое",
}

// ReviewPosted tells the teacher about a new review.
func (n *Notifier) ReviewPosted(ctx context.Context, teacherID int64, specialistName string, rating int, text string) {
	msg := "⭐ Новый отзыв: " + strings.Repeat("★", rating) + strings.Repeat("☆", 5-rating)
	if text != "" {
		msg += "\n«" + text + "»"
	}
	n.Send(ctx, teacherID, "review_posted", msg, keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenProfile, "/teach")}))
}

// ProfileReported asks admins to look at a report.
func (n *Notifier) ProfileReported(ctx context.Context, adminIDs []int64, specialistName, reason, text string) {
	msg := fmt.Sprintf("🚩 Жалоба на профиль «%s»: %s", specialistName, reportReasons[reason])
	if text != "" {
		msg += "\n💬 " + text
	}
	kb := keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenQueue, "/admin/reports")})
	for _, id := range adminIDs {
		n.Send(ctx, id, "profile_reported", msg, kb)
	}
}

// VerificationRequested asks admins to check documents.
func (n *Notifier) VerificationRequested(ctx context.Context, adminIDs []int64, specialistName string) {
	kb := keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenQueue, "/admin/verifications")})
	for _, id := range adminIDs {
		n.Send(ctx, id, "verification_requested", "📄 "+specialistName+" просит проверить документы для значка «Проверен».", kb)
	}
}

// VerificationDecided tells the teacher the outcome.
func (n *Notifier) VerificationDecided(ctx context.Context, teacherID int64, approved bool, note string) {
	msg := "✅ Документы проверены — в вашем профиле теперь значок «Проверен»."
	if !approved {
		msg = "❌ Не удалось подтвердить документы."
		if note != "" {
			msg += "\n" + note
		}
		msg += "\nЗагрузите другие документы и отправьте запрос снова."
	}
	n.Send(ctx, teacherID, "verification_decided", msg, keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenEditor, "/teach/verification")}))
}
