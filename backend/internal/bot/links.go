package bot

import (
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
)

// Links builds Mini App URLs and personal bot links.
type Links struct {
	WebAppURL   string // https://..., may be empty in dev
	BotUsername string // without @, may be empty
}

// App returns the Mini App URL for an in-app path such as "/teach", or "" without WEBAPP_URL.
func (l Links) App(path string) string {
	if l.WebAppURL == "" {
		return ""
	}
	return strings.TrimRight(l.WebAppURL, "/") + path
}

// AppButton is an inline button opening the Mini App at path, or nil without WEBAPP_URL.
func (l Links) AppButton(text, path string) *models.InlineKeyboardButton {
	url := l.App(path)
	if url == "" {
		return nil
	}
	return &models.InlineKeyboardButton{Text: text, WebApp: &models.WebAppInfo{URL: url}}
}

// SpecialistLink is the personal link t.me/<bot>?start=t_<id>, or "" without BOT_USERNAME.
func (l Links) SpecialistLink(id int64) string {
	if l.BotUsername == "" {
		return ""
	}
	return "https://t.me/" + l.BotUsername + "?start=" + StartSpecialistPrefix + strconv.FormatInt(id, 10)
}

// keyboard builds an inline keyboard from rows, skipping nil buttons and empty rows.
func keyboard(rows ...[]*models.InlineKeyboardButton) models.ReplyMarkup {
	var out [][]models.InlineKeyboardButton
	for _, row := range rows {
		var r []models.InlineKeyboardButton
		for _, b := range row {
			if b != nil {
				r = append(r, *b)
			}
		}
		if len(r) > 0 {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: out}
}
