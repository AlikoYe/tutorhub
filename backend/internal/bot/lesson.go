package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/AlikoYe/tutorhub/backend/internal/lesson"
	"github.com/AlikoYe/tutorhub/backend/internal/lessonstatus"
)

// Callback data for the teacher's buttons: "ls:c:<id>" confirms, "ls:d:<id>" declines.
const (
	callbackLesson  = "ls:"
	callbackConfirm = "ls:c:"
	callbackDecline = "ls:d:"
)

func lessonAttr(l lesson.Lesson) slog.Attr { return slog.Int64("lesson_id", l.ID) }

// LessonRequested asks the teacher to confirm or decline.
func (n *Notifier) LessonRequested(ctx context.Context, l lesson.Lesson) {
	id := strconv.FormatInt(l.ID, 10)
	kb := keyboard(
		[]*models.InlineKeyboardButton{
			{Text: textConfirm, CallbackData: callbackConfirm + id},
			{Text: textDecline, CallbackData: callbackDecline + id},
		},
		[]*models.InlineKeyboardButton{n.links.AppButton(textOpenRequests, "/teach/lessons")},
	)
	n.Send(ctx, l.TeacherUserID, "lesson_requested", textLessonRequested(l, n.loc), kb, lessonAttr(l))
}

// LessonDecided tells the student the teacher's answer.
func (n *Notifier) LessonDecided(ctx context.Context, l lesson.Lesson) {
	student := l.ForStudent(n.now())
	text := textLessonDeclined(student, n.loc)
	if l.Status == lessonstatus.Confirmed {
		text = textLessonConfirmed(student, n.loc)
	}
	n.Send(ctx, l.StudentInfo.TelegramID, "lesson_decided", text,
		keyboard([]*models.InlineKeyboardButton{n.links.AppButton(textOpenLessons, "/lessons")}), lessonAttr(l))
}

// LessonCancelled tells the other side.
func (n *Notifier) LessonCancelled(ctx context.Context, l lesson.Lesson) {
	if l.CancelledBy != nil && *l.CancelledBy == lesson.ByTeacher {
		n.Send(ctx, l.StudentInfo.TelegramID, "lesson_cancelled", textLessonCancelled(l, n.loc, false), nil, lessonAttr(l))
		return
	}
	n.Send(ctx, l.TeacherUserID, "lesson_cancelled", textLessonCancelled(l, n.loc, true), nil, lessonAttr(l))
}

// LessonExpired tells the student the teacher did not answer in time.
func (n *Notifier) LessonExpired(ctx context.Context, l lesson.Lesson) {
	n.Send(ctx, l.StudentInfo.TelegramID, "lesson_expired", textLessonExpired(l, n.loc), nil, lessonAttr(l))
}

// LessonReminder reminds both sides of an upcoming confirmed lesson.
func (n *Notifier) LessonReminder(ctx context.Context, l lesson.Lesson) {
	n.Send(ctx, l.StudentInfo.TelegramID, "lesson_reminder", textLessonReminder(l.ForStudent(n.now()), n.loc, false), nil, lessonAttr(l))
	n.Send(ctx, l.TeacherUserID, "lesson_reminder", textLessonReminder(l, n.loc, true), nil, lessonAttr(l))
}

// LessonDecider confirms or declines a request as the acting Telegram user (lesson.Service).
type LessonDecider interface {
	Decide(ctx context.Context, actorID, lessonID int64, confirm bool) (lesson.Lesson, error)
}

// RegisterLessons adds the teacher's confirm/decline buttons.
func (b *Bot) RegisterLessons(decider LessonDecider, loc *time.Location) {
	b.OnCallback(callbackLesson, func(ctx context.Context, q *models.CallbackQuery) {
		confirm := strings.HasPrefix(q.Data, callbackConfirm)
		raw := strings.TrimPrefix(strings.TrimPrefix(q.Data, callbackConfirm), callbackDecline)
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || (!confirm && !strings.HasPrefix(q.Data, callbackDecline)) {
			b.AnswerCallback(ctx, q, textTryAgain, true)
			return
		}
		l, err := decider.Decide(ctx, q.From.ID, id, confirm)
		switch {
		case err == nil:
			outcome := textOutcomeNo
			if confirm {
				outcome = textOutcomeOK
			}
			b.AnswerCallback(ctx, q, outcome, false)
			b.EditCallbackMessage(ctx, q, textLessonDecidedForTeacher(l, loc, confirm))
		case errors.Is(err, lesson.ErrNotPending):
			b.AnswerCallback(ctx, q, textNotPending, true)
			b.EditCallbackMessage(ctx, q, CallbackMessageText(q)+"\n\n"+textNoLongerOpen)
		case errors.Is(err, lesson.ErrForbidden):
			b.AnswerCallback(ctx, q, textForbidden, true)
		case errors.Is(err, lesson.ErrNotFound):
			b.AnswerCallback(ctx, q, textLessonMissing, true)
			b.EditCallbackMessage(ctx, q, CallbackMessageText(q))
		default:
			b.log.ErrorContext(ctx, "decide from bot", slog.Int64("lesson_id", id), slog.Any("err", err))
			b.AnswerCallback(ctx, q, textTryAgain, true)
		}
	})
}
