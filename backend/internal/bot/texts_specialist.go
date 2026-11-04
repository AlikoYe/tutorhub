package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
)

const (
	textOpenEditor   = "✏️ Открыть анкету"
	textOpenQueue    = "🛠 Открыть очередь"
	textOpenProfile  = "👤 Открыть профиль"
	textTeachNew     = "🎓 Станьте преподавателем TutorHub!\nЗаполните анкету: предметы, цены, формат занятий. После проверки администратором профиль появится в каталоге."
	textTeachDraft   = "📝 Ваша анкета — черновик. Заполните её и отправьте на проверку."
	textTeachPending = "⏳ Ваша анкета на проверке. Мы напишем, когда её одобрят."
	textTeachLive    = "✅ Ваш профиль опубликован."
	textTeachHidden  = "🙈 Ваш профиль скрыт из каталога. Его можно вернуть в анкете."
	textTeachBlocked = "⛔ Ваш профиль заблокирован. Напишите в поддержку."
)

var statusNames = map[string]string{
	"draft": "черновик", "pending": "на проверке", "live": "опубликован", "hidden": "скрыт", "blocked": "заблокирован",
}

func textTeach(s *specialist.Summary, personalLink string) string {
	if s == nil {
		return textTeachNew
	}
	var t string
	switch s.Status {
	case "draft":
		t = textTeachDraft
	case "pending":
		t = textTeachPending
	case "live":
		t = textTeachLive
		if !s.Listed {
			t += "\n⚠️ Сейчас профиль не виден в поиске: нет активной подписки."
		}
	case "hidden":
		t = textTeachHidden
	default:
		t = textTeachBlocked
	}
	if personalLink != "" && (s.Status == "live" || s.Status == "hidden") {
		t += "\n\n🔗 Ваша личная ссылка для учеников:\n" + personalLink
	}
	return t
}

func textSubmittedForAdmin(p specialist.Private) string {
	subjects := make([]string, 0, len(p.Subjects))
	for _, o := range p.Subjects {
		subjects = append(subjects, o.NameRU)
	}
	return strings.Join([]string{
		"🆕 Новая анкета на проверку",
		"👤 " + p.DisplayName + ownerHandle(p.Owner),
		"📚 " + strings.Join(subjects, ", "),
	}, "\n")
}

func ownerHandle(o specialist.Owner) string {
	if o.Username != nil && *o.Username != "" {
		return " (@" + *o.Username + ")"
	}
	return ""
}

func textApproved(p specialist.Private, loc *time.Location) string {
	t := "🎉 Ваш профиль одобрен и опубликован в каталоге TutorHub!"
	if s := p.Subscription; s != nil {
		t += fmt.Sprintf("\n%s до %s.", planName(s.Plan), lastDay(s.EndsAt, loc))
	}
	return t
}

func textRejected(reason string) string {
	return "✏️ Анкету нужно доработать:\n" + reason + "\n\nИсправьте и отправьте её снова."
}

func textSpecialistLink(name string) string {
	return "👤 Профиль преподавателя: " + name + "\nОткройте, чтобы посмотреть предметы, цены и свободное время."
}

var planNames = map[string]string{
	"founding": "Тариф «Основатель» (бесплатно)",
	"trial":    "Пробный период",
	"basic":    "Тариф «Базовый»",
	"featured": "Тариф «Премиум»",
}

func planName(plan string) string {
	if n, ok := planNames[plan]; ok {
		return n
	}
	return plan
}

// lastDay formats the last included day of a period ending at local midnight.
func lastDay(endsAt time.Time, loc *time.Location) string {
	return endsAt.In(loc).Add(-time.Second).Format("02.01.2006")
}
