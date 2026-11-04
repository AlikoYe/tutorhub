package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot/models"

	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
)

const textOpenSubscription = "💳 Подписка"

func subAttr(s specialist.Subscription) slog.Attr { return slog.Int64("subscription_id", s.ID) }

func (n *Notifier) subKeyboard() models.ReplyMarkup {
	return keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenSubscription, "/teach/subscription")})
}

// SubscriptionActivated tells the teacher an admin activated a plan.
func (n *Notifier) SubscriptionActivated(ctx context.Context, teacherID int64, s specialist.Subscription) {
	text := fmt.Sprintf("💳 Подписка активирована: %s до %s.\nВаш профиль показывается в каталоге.", planName(s.Plan), lastDay(s.EndsAt, n.loc))
	if s.StartsAt.After(n.now()) {
		text = fmt.Sprintf("💳 Подписка продлена: %s с %s до %s.", planName(s.Plan), s.StartsAt.In(n.loc).Format("02.01.2006"), lastDay(s.EndsAt, n.loc))
	}
	n.Send(ctx, teacherID, "subscription_activated", text, n.subKeyboard(), subAttr(s))
}

// SubscriptionEnding reminds the teacher 7 days and 1 day before the plan ends.
func (n *Notifier) SubscriptionEnding(ctx context.Context, teacherID int64, s specialist.Subscription) {
	text := fmt.Sprintf("⏳ %s заканчивается %s.\nПродлите подписку, чтобы профиль остался в каталоге и ученики продолжали вас находить.",
		planName(s.Plan), lastDay(s.EndsAt, n.loc))
	n.Send(ctx, teacherID, "subscription_ending", text, n.subKeyboard(), subAttr(s))
}

// SubscriptionExpired tells the teacher they left search.
func (n *Notifier) SubscriptionExpired(ctx context.Context, teacherID int64, s specialist.Subscription) {
	text := "🔕 Подписка закончилась, профиль скрыт из поиска.\nВаши данные и личная ссылка сохранены. Продлите подписку — профиль сразу вернётся в каталог."
	n.Send(ctx, teacherID, "subscription_expired", text, n.subKeyboard(), subAttr(s))
}
