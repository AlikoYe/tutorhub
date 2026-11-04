package site

import (
	"fmt"

	"github.com/AlikoYe/tutorhub/backend/internal/geo"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
	"github.com/AlikoYe/tutorhub/backend/internal/subject"
)

// Langs are the public site languages; the first is the default.
var Langs = []string{"ru", "kk", "en"}

var texts = map[string]map[string]string{
	"ru": {
		"tagline":       "Репетиторы, учителя и консультанты Казахстана",
		"tutorsIn":      "Репетиторы в городе %s",
		"tutorsFor":     "Репетиторы: %s — %s",
		"tutorsForDesc": "%d преподавателей: %s в городе %s. Цены от %s за урок, отзывы, свободное время. Запись в Telegram.",
		"cityDesc":      "Найдите репетитора в городе %s: школьные предметы, ЕНТ, IELTS, языки, программирование, музыка. Запись в Telegram.",
		"teacherDesc":   "%s — %s. %s. Цены от %s за урок. Запись на урок в Telegram.",
		"from":          "от",
		"perLesson":     "за урок",
		"book":          "Записаться в Telegram",
		"openApp":       "Открыть в TutorHub",
		"share":         "Поделиться",
		"subjects":      "Предметы и цены",
		"about":         "О себе",
		"education":     "Образование",
		"reviews":       "Отзывы",
		"verified":      "Проверен",
		"years":         "лет опыта",
		"formats":       "Формат",
		"languages":     "Язык занятий",
		"nobody":        "Пока нет преподавателей по этому предмету. Загляните позже или выберите другой предмет.",
		"allSubjects":   "Все предметы",
		"found":         "Найдено преподавателей",
		"trialFree":     "Бесплатный пробный урок",
		"notListed":     "Этот преподаватель сейчас не принимает новых учеников через каталог.",
		"footer":        "TutorHub — бесплатно для учеников, без комиссий.",
	},
	"kk": {
		"tagline":       "Қазақстанның репетиторлары, мұғалімдері және кеңесшілері",
		"tutorsIn":      "%s қаласындағы репетиторлар",
		"tutorsFor":     "Репетиторлар: %s — %s",
		"tutorsForDesc": "%d оқытушы: %s, %s қаласы. Сабақ бағасы %s бастап, пікірлер, бос уақыт. Telegram арқылы жазылу.",
		"cityDesc":      "%s қаласынан репетитор табыңыз: мектеп пәндері, ҰБТ, IELTS, тілдер, бағдарламалау, музыка. Telegram арқылы жазылу.",
		"teacherDesc":   "%s — %s. %s. Сабақ бағасы %s бастап. Telegram арқылы сабаққа жазылу.",
		"from":          "бастап",
		"perLesson":     "сабақ",
		"book":          "Telegram арқылы жазылу",
		"openApp":       "TutorHub-та ашу",
		"share":         "Бөлісу",
		"subjects":      "Пәндер мен бағалар",
		"about":         "Өзі туралы",
		"education":     "Білімі",
		"reviews":       "Пікірлер",
		"verified":      "Тексерілген",
		"years":         "жыл тәжірибе",
		"formats":       "Формат",
		"languages":     "Сабақ тілі",
		"nobody":        "Бұл пән бойынша әзірге оқытушылар жоқ. Кейінірек кіріңіз немесе басқа пәнді таңдаңыз.",
		"allSubjects":   "Барлық пәндер",
		"found":         "Табылған оқытушылар",
		"trialFree":     "Тегін сынақ сабақ",
		"notListed":     "Бұл оқытушы қазір каталог арқылы жаңа оқушылар қабылдамайды.",
		"footer":        "TutorHub — оқушыларға тегін, комиссиясыз.",
	},
	"en": {
		"tagline":       "Tutors, teachers and consultants in Kazakhstan",
		"tutorsIn":      "Tutors in %s",
		"tutorsFor":     "Tutors: %s — %s",
		"tutorsForDesc": "%d teachers for %s in %s. Prices from %s per lesson, reviews, free time slots. Book in Telegram.",
		"cityDesc":      "Find a tutor in %s: school subjects, UNT, IELTS, languages, programming, music. Book in Telegram.",
		"teacherDesc":   "%s — %s. %s. Prices from %s per lesson. Book a lesson in Telegram.",
		"from":          "from",
		"perLesson":     "per lesson",
		"book":          "Book in Telegram",
		"openApp":       "Open in TutorHub",
		"share":         "Share",
		"subjects":      "Subjects and prices",
		"about":         "About",
		"education":     "Education",
		"reviews":       "Reviews",
		"verified":      "Verified",
		"years":         "years of experience",
		"formats":       "Format",
		"languages":     "Lesson language",
		"nobody":        "No teachers for this subject yet. Check back later or pick another subject.",
		"allSubjects":   "All subjects",
		"found":         "Teachers found",
		"trialFree":     "Free trial lesson",
		"notListed":     "This teacher is not taking new students through the catalogue right now.",
		"footer":        "TutorHub — free for students, no commissions.",
	},
}

var formatNames = map[string]map[string]string{
	"ru": {"online": "Онлайн", "teacher_place": "У преподавателя", "student_home": "На дому у ученика"},
	"kk": {"online": "Онлайн", "teacher_place": "Оқытушыда", "student_home": "Оқушының үйінде"},
	"en": {"online": "Online", "teacher_place": "At the teacher's place", "student_home": "At the student's home"},
}

var languageNames = map[string]string{"kk": "Қазақша", "ru": "Русский", "en": "English"}

var levelNames = map[string]map[string]string{
	"ru": {"primary": "1–4 класс", "middle": "5–9 класс", "high": "10–11 класс", "ent": "ЕНТ", "university": "Студенты", "adult": "Взрослые"},
	"kk": {"primary": "1–4 сынып", "middle": "5–9 сынып", "high": "10–11 сынып", "ent": "ҰБТ", "university": "Студенттер", "adult": "Ересектер"},
	"en": {"primary": "Grades 1–4", "middle": "Grades 5–9", "high": "Grades 10–11", "ent": "UNT", "university": "University", "adult": "Adults"},
}

func t(lang, key string, args ...any) string {
	s := texts[lang][key]
	if s == "" {
		s = texts["ru"][key]
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

func placeName(p geo.Place, lang string) string {
	switch lang {
	case "kk":
		return p.NameKK
	case "en":
		return p.NameEN
	}
	return p.NameRU
}

func subjectName(s subject.Subject, lang string) string {
	switch lang {
	case "kk":
		return s.NameKK
	case "en":
		return s.NameEN
	}
	return s.NameRU
}

func offerName(o specialist.Offer, lang string) string {
	switch lang {
	case "kk":
		return o.NameKK
	case "en":
		return o.NameEN
	}
	return o.NameRU
}

func districtName(city, district, lang string) string {
	c, ok := geo.FindCity(city)
	if !ok {
		return ""
	}
	for _, d := range c.Districts {
		if d.Slug == district {
			return placeName(d, lang)
		}
	}
	return ""
}
