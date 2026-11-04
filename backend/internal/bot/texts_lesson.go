package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlikoYe/tutorhub/backend/internal/lesson"
)

const (
	textConfirm       = "✅ Подтвердить"
	textDecline       = "❌ Отклонить"
	textOutcomeOK     = "✅ Подтверждено"
	textOutcomeNo     = "❌ Отклонено"
	textNotPending    = "⚠️ Эта заявка уже не ожидает ответа."
	textNoLongerOpen  = "⚠️ Заявка больше не активна"
	textForbidden     = "⛔ Это может сделать только преподаватель урока."
	textLessonMissing = "⚠️ Урок не найден."
	textOpenRequests  = "📋 Все заявки"
	textOpenLessons   = "📅 Мои уроки"
)

var weekdaysRU = [...]string{"вс", "пн", "вт", "ср", "чт", "пт", "сб"}
var monthsRU = [...]string{"", "января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}

// when formats a lesson's time, e.g. "пн, 9 ноября, 16:00–17:00".
func when(l lesson.Lesson, loc *time.Location) string {
	s, e := l.StartsAt.In(loc), l.EndsAt.In(loc)
	return fmt.Sprintf("%s, %d %s, %s–%s", weekdaysRU[s.Weekday()], s.Day(), monthsRU[s.Month()], s.Format("15:04"), e.Format("15:04"))
}

func subjectLine(l lesson.Lesson) string {
	t := "📚 " + l.Subject.NameRU
	if l.IsTrial {
		t += " (пробный урок)"
	}
	return t
}

func studentLine(l lesson.Lesson) string {
	name := l.StudentInfo.Name
	if name == "" {
		name = "Ученик"
	}
	if l.StudentInfo.Username != nil && *l.StudentInfo.Username != "" {
		name += " (@" + *l.StudentInfo.Username + ")"
	}
	return "🙋 " + name
}

func priceLine(l lesson.Lesson) string {
	if l.PriceKZT == 0 {
		return "💰 бесплатно"
	}
	return fmt.Sprintf("💰 %d ₸", l.PriceKZT)
}

func textLessonRequested(l lesson.Lesson, loc *time.Location) string {
	lines := []string{"🆕 Заявка на урок", subjectLine(l), "🕖 " + when(l, loc), studentLine(l), priceLine(l)}
	if l.Note != "" {
		lines = append(lines, "💬 "+l.Note)
	}
	if l.ExpiresAt != nil {
		lines = append(lines, "⏳ Ответьте до "+l.ExpiresAt.In(loc).Format("02.01 15:04"))
	}
	return strings.Join(lines, "\n")
}

func textLessonDecidedForTeacher(l lesson.Lesson, loc *time.Location, confirmed bool) string {
	outcome := textOutcomeNo
	if confirmed {
		outcome = textOutcomeOK
	}
	return strings.Join([]string{"📋 Заявка на урок", subjectLine(l), "🕖 " + when(l, loc), studentLine(l), "", outcome}, "\n")
}

func textLessonConfirmed(l lesson.Lesson, loc *time.Location) string {
	lines := []string{"✅ Урок подтверждён!", "👤 " + l.Specialist.DisplayName, subjectLine(l), "🕖 " + when(l, loc), priceLine(l)}
	if l.Specialist.Address != nil {
		lines = append(lines, "📍 "+*l.Specialist.Address)
	}
	return strings.Join(lines, "\n")
}

func textLessonDeclined(l lesson.Lesson, loc *time.Location) string {
	return fmt.Sprintf("❌ %s не сможет провести урок %s.\nВыберите другое время.", l.Specialist.DisplayName, when(l, loc))
}

func textLessonCancelled(l lesson.Lesson, loc *time.Location, forTeacher bool) string {
	who := "Преподаватель " + l.Specialist.DisplayName + " отменил(а)"
	if forTeacher {
		who = strings.TrimPrefix(studentLine(l), "🙋 ") + " отменил(а)"
	}
	t := fmt.Sprintf("🚫 %s урок\n%s\n🕖 %s", who, subjectLine(l), when(l, loc))
	if l.CancelReason != "" {
		t += "\n💬 " + l.CancelReason
	}
	return t
}

func textLessonExpired(l lesson.Lesson, loc *time.Location) string {
	return fmt.Sprintf("⌛ %s не ответил(а) вовремя, заявка на %s истекла.\nПопробуйте выбрать другое время или другого преподавателя.", l.Specialist.DisplayName, when(l, loc))
}

func textLessonReminder(l lesson.Lesson, loc *time.Location, forTeacher bool) string {
	lines := []string{"⏰ Напоминание: скоро урок", subjectLine(l), "🕖 " + when(l, loc)}
	if forTeacher {
		lines = append(lines, studentLine(l))
	} else {
		lines = append(lines, "👤 "+l.Specialist.DisplayName)
		if l.Specialist.Address != nil {
			lines = append(lines, "📍 "+*l.Specialist.Address)
		}
	}
	return strings.Join(lines, "\n")
}
