// UI texts in Russian (default), Kazakh and English. The student-facing screens are translated;
// teacher and admin screens are Russian for now. `locale` is reactive: templates re-render on change.
import { reactive, ref } from 'vue'

const dict = {
  ru: {
    appName: 'TutorHub',
    tabCatalogue: 'Каталог',
    tabLessons: 'Мои уроки',
    tabFavourites: 'Избранное',
    tabTeach: 'Я учитель',
    tabAdmin: 'Админ',
    loading: 'Загрузка…',
    retry: 'Повторить',
    back: 'Назад',
    save: 'Сохранить',
    saved: 'Сохранено',
    cancel: 'Отмена',
    notFound: 'Страница не найдена',
    comingSoon: 'Скоро здесь появится этот раздел.',
    authFailed: 'Не удалось войти. Откройте приложение из Telegram.',
    searchPlaceholder: 'Предмет, имя или экзамен',
    filters: 'Фильтры',
    resetFilters: 'Сбросить',
    subject: 'Предмет',
    anySubject: 'Любой предмет',
    level: 'Уровень',
    lessonLanguage: 'Язык занятий',
    format: 'Формат',
    district: 'Район',
    anyDistrict: 'Любой район',
    rating: 'Рейтинг',
    priceFrom: 'Цена от',
    priceTo: 'до',
    verifiedOnly: 'Только проверенные',
    freeThisWeek: 'Есть свободное время на неделе',
    nothingFound: 'Никого не нашли. Попробуйте изменить фильтры.',
    loadMore: 'Показать ещё',
    found: 'Найдено',
    from: 'от',
    years: 'лет опыта',
    verified: 'Проверен',
    featured: 'Топ',
    trialFree: 'Бесплатный пробный урок',
    trialPaid: 'Пробный урок',
    aboutMe: 'О себе',
    education: 'Образование',
    subjectsAndPrices: 'Предметы и цены',
    address: 'Адрес',
    notListed: 'Профиль сейчас не показывается в поиске.',
    lessonLength: 'Длительность урока',
    min: 'мин',
    cancelRule: 'отмена не позже чем за {h} ч',
    contact: 'Написать в Telegram',
    favouriteAdd: 'В избранное',
    favouriteOn: 'В избранном',
    share: 'Поделиться',
    report: 'Пожаловаться',
    reportSend: 'Отправить',
    reportConfirm: 'Отправить жалобу администраторам?',
    reportThanks: 'Спасибо! Мы проверим профиль.',
    reportDetails: 'Подробности (необязательно)',
    book: 'Записаться на урок',
    bookAt: 'Записаться на {time}',
    scheduleLoading: 'Загрузка расписания…',
    noSlots: 'Нет свободного времени в ближайшие две недели.',
    trialLesson: 'Пробный урок',
    free: 'бесплатно',
    noteToTeacher: 'Сообщение преподавателю: класс, цель, уровень',
    priceNote: 'Стоимость: {price}. Оплата напрямую преподавателю.',
    requestSent: 'Заявка отправлена! Преподаватель получит её в Telegram и ответит.',
    upcoming: 'Предстоящие',
    past: 'Прошедшие',
    lessonsEmpty: 'Здесь будут ваши уроки.',
    findTeacher: 'Найти преподавателя',
    cancelLesson: 'Отменить',
    cancelLessonConfirm: 'Отменить урок?',
    trialShort: 'пробный',
    cancelReason: 'Причина отмены',
    favouritesEmpty: 'Сохраняйте понравившихся преподавателей кнопкой ☆ в профиле.',
    reviews: 'Отзывы',
    writeReview: '✍️ Оставить отзыв',
    editReview: 'Изменить мой отзыв',
    reviewHidden: 'Ваш отзыв скрыт модератором.',
    reviewPlaceholder: 'Что понравилось? Чего удалось достичь?',
    noReviews: 'Пока нет отзывов. Отзыв можно оставить после проведённого урока.',
    student: 'Ученик',
    language: 'Язык',
  },
  kk: {
    appName: 'TutorHub',
    tabCatalogue: 'Каталог',
    tabLessons: 'Сабақтарым',
    tabFavourites: 'Таңдаулы',
    tabTeach: 'Мен мұғаліммін',
    tabAdmin: 'Әкімші',
    loading: 'Жүктелуде…',
    retry: 'Қайталау',
    back: 'Артқа',
    save: 'Сақтау',
    saved: 'Сақталды',
    cancel: 'Бас тарту',
    notFound: 'Бет табылмады',
    comingSoon: 'Бұл бөлім жақында пайда болады.',
    authFailed: 'Кіру мүмкін болмады. Қосымшаны Telegram-нан ашыңыз.',
    searchPlaceholder: 'Пән, аты немесе емтихан',
    filters: 'Сүзгілер',
    resetFilters: 'Тазарту',
    subject: 'Пән',
    anySubject: 'Кез келген пән',
    level: 'Деңгей',
    lessonLanguage: 'Сабақ тілі',
    format: 'Формат',
    district: 'Аудан',
    anyDistrict: 'Кез келген аудан',
    rating: 'Рейтинг',
    priceFrom: 'Баға',
    priceTo: 'дейін',
    verifiedOnly: 'Тек тексерілгендер',
    freeThisWeek: 'Осы аптада бос уақыты бар',
    nothingFound: 'Ешкім табылмады. Сүзгілерді өзгертіп көріңіз.',
    loadMore: 'Тағы көрсету',
    found: 'Табылды',
    from: 'бастап',
    years: 'жыл тәжірибе',
    verified: 'Тексерілген',
    featured: 'Топ',
    trialFree: 'Тегін сынақ сабақ',
    trialPaid: 'Сынақ сабақ',
    aboutMe: 'Өзі туралы',
    education: 'Білімі',
    subjectsAndPrices: 'Пәндер мен бағалар',
    address: 'Мекенжай',
    notListed: 'Профиль қазір іздеуде көрсетілмейді.',
    lessonLength: 'Сабақ ұзақтығы',
    min: 'мин',
    cancelRule: 'кемінде {h} сағат бұрын бас тартуға болады',
    contact: 'Telegram-да жазу',
    favouriteAdd: 'Таңдаулыға',
    favouriteOn: 'Таңдаулыда',
    share: 'Бөлісу',
    report: 'Шағымдану',
    reportSend: 'Жіберу',
    reportConfirm: 'Әкімшілерге шағым жіберу керек пе?',
    reportThanks: 'Рақмет! Профильді тексереміз.',
    reportDetails: 'Толығырақ (міндетті емес)',
    book: 'Сабаққа жазылу',
    bookAt: '{time} уақытына жазылу',
    scheduleLoading: 'Кесте жүктелуде…',
    noSlots: 'Алдағы екі аптада бос уақыт жоқ.',
    trialLesson: 'Сынақ сабақ',
    free: 'тегін',
    noteToTeacher: 'Мұғалімге хабарлама: сынып, мақсат, деңгей',
    priceNote: 'Құны: {price}. Төлем тікелей мұғалімге.',
    requestSent: 'Өтінім жіберілді! Мұғалім оны Telegram-да алып, жауап береді.',
    upcoming: 'Алдағы',
    past: 'Өткен',
    lessonsEmpty: 'Мұнда сіздің сабақтарыңыз болады.',
    findTeacher: 'Мұғалім табу',
    cancelLesson: 'Бас тарту',
    cancelLessonConfirm: 'Сабақтан бас тарту керек пе?',
    trialShort: 'сынақ',
    cancelReason: 'Бас тарту себебі',
    favouritesEmpty: 'Ұнаған мұғалімдерді профильдегі ☆ батырмасымен сақтаңыз.',
    reviews: 'Пікірлер',
    writeReview: '✍️ Пікір қалдыру',
    editReview: 'Пікірімді өзгерту',
    reviewHidden: 'Пікіріңізді модератор жасырды.',
    reviewPlaceholder: 'Не ұнады? Қандай нәтижеге жеттіңіз?',
    noReviews: 'Әзірге пікір жоқ. Пікірді өткен сабақтан кейін қалдыруға болады.',
    student: 'Оқушы',
    language: 'Тіл',
  },
  en: {
    appName: 'TutorHub',
    tabCatalogue: 'Catalogue',
    tabLessons: 'My lessons',
    tabFavourites: 'Favourites',
    tabTeach: 'I teach',
    tabAdmin: 'Admin',
    loading: 'Loading…',
    retry: 'Retry',
    back: 'Back',
    save: 'Save',
    saved: 'Saved',
    cancel: 'Cancel',
    notFound: 'Page not found',
    comingSoon: 'This section is coming soon.',
    authFailed: 'Could not sign in. Open the app from Telegram.',
    searchPlaceholder: 'Subject, name or exam',
    filters: 'Filters',
    resetFilters: 'Reset',
    subject: 'Subject',
    anySubject: 'Any subject',
    level: 'Level',
    lessonLanguage: 'Lesson language',
    format: 'Format',
    district: 'District',
    anyDistrict: 'Any district',
    rating: 'Rating',
    priceFrom: 'Price from',
    priceTo: 'to',
    verifiedOnly: 'Verified only',
    freeThisWeek: 'Has free time this week',
    nothingFound: 'Nobody found. Try other filters.',
    loadMore: 'Show more',
    found: 'Found',
    from: 'from',
    years: 'years of experience',
    verified: 'Verified',
    featured: 'Top',
    trialFree: 'Free trial lesson',
    trialPaid: 'Trial lesson',
    aboutMe: 'About',
    education: 'Education',
    subjectsAndPrices: 'Subjects and prices',
    address: 'Address',
    notListed: 'This profile is not shown in search right now.',
    lessonLength: 'Lesson length',
    min: 'min',
    cancelRule: 'cancel at least {h} h before',
    contact: 'Message in Telegram',
    favouriteAdd: 'Save',
    favouriteOn: 'Saved',
    share: 'Share',
    report: 'Report',
    reportSend: 'Send',
    reportConfirm: 'Send a report to the admins?',
    reportThanks: 'Thank you! We will check this profile.',
    reportDetails: 'Details (optional)',
    book: 'Book a lesson',
    bookAt: 'Book {time}',
    scheduleLoading: 'Loading the schedule…',
    noSlots: 'No free time in the next two weeks.',
    trialLesson: 'Trial lesson',
    free: 'free',
    noteToTeacher: 'Message to the teacher: grade, goal, level',
    priceNote: 'Price: {price}. Pay the teacher directly.',
    requestSent: 'Request sent! The teacher gets it in Telegram and will answer.',
    upcoming: 'Upcoming',
    past: 'Past',
    lessonsEmpty: 'Your lessons will appear here.',
    findTeacher: 'Find a teacher',
    cancelLesson: 'Cancel',
    cancelLessonConfirm: 'Cancel this lesson?',
    trialShort: 'trial',
    cancelReason: 'Cancellation reason',
    favouritesEmpty: 'Save teachers you like with the ☆ button on their profile.',
    reviews: 'Reviews',
    writeReview: '✍️ Write a review',
    editReview: 'Edit my review',
    reviewHidden: 'Your review was hidden by a moderator.',
    reviewPlaceholder: 'What did you like? What did you achieve?',
    noReviews: 'No reviews yet. You can review after a completed lesson.',
    student: 'Student',
    language: 'Language',
  },
}

const vocab = {
  ru: {
    levels: { primary: '1–4 класс', middle: '5–9 класс', high: '10–11 класс', ent: 'ЕНТ', university: 'Студенты', adult: 'Взрослые' },
    formats: { online: 'Онлайн', teacher_place: 'У преподавателя', student_home: 'На дому у ученика' },
    kinds: { teacher: 'Преподаватель', consultant: 'Консультант' },
    categories: { school: 'Школьные предметы', exam: 'Экзамены', language: 'Языки', it: 'IT и программирование', music: 'Музыка', arts: 'Творчество и хобби', consulting: 'Консультации', other: 'Другое' },
    lessonStatuses: { pending: 'Ждёт ответа', confirmed: 'Подтверждён', declined: 'Отклонён', cancelled: 'Отменён', expired: 'Истёк', completed: 'Проведён' },
    weekdaysShort: ['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'],
    reportReasons: { fake: 'Фейковый профиль', inappropriate: 'Неприемлемое содержание', spam: 'Спам', no_show: 'Не пришёл на урок', other: 'Другое' },
  },
  kk: {
    levels: { primary: '1–4 сынып', middle: '5–9 сынып', high: '10–11 сынып', ent: 'ҰБТ', university: 'Студенттер', adult: 'Ересектер' },
    formats: { online: 'Онлайн', teacher_place: 'Мұғалімде', student_home: 'Оқушының үйінде' },
    kinds: { teacher: 'Мұғалім', consultant: 'Кеңесші' },
    categories: { school: 'Мектеп пәндері', exam: 'Емтихандар', language: 'Тілдер', it: 'IT және бағдарламалау', music: 'Музыка', arts: 'Шығармашылық', consulting: 'Кеңестер', other: 'Басқа' },
    lessonStatuses: { pending: 'Жауап күтуде', confirmed: 'Расталды', declined: 'Қабылданбады', cancelled: 'Бас тартылды', expired: 'Мерзімі өтті', completed: 'Өтті' },
    weekdaysShort: ['Жс', 'Дс', 'Сс', 'Ср', 'Бс', 'Жм', 'Сн'],
    reportReasons: { fake: 'Жалған профиль', inappropriate: 'Орынсыз мазмұн', spam: 'Спам', no_show: 'Сабаққа келмеді', other: 'Басқа' },
  },
  en: {
    levels: { primary: 'Grades 1–4', middle: 'Grades 5–9', high: 'Grades 10–11', ent: 'UNT', university: 'University', adult: 'Adults' },
    formats: { online: 'Online', teacher_place: "At the teacher's", student_home: "At the student's home" },
    kinds: { teacher: 'Teacher', consultant: 'Consultant' },
    categories: { school: 'School subjects', exam: 'Exams', language: 'Languages', it: 'IT and programming', music: 'Music', arts: 'Arts and hobbies', consulting: 'Consulting', other: 'Other' },
    lessonStatuses: { pending: 'Awaiting answer', confirmed: 'Confirmed', declined: 'Declined', cancelled: 'Cancelled', expired: 'Expired', completed: 'Completed' },
    weekdaysShort: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'],
    reportReasons: { fake: 'Fake profile', inappropriate: 'Inappropriate content', spam: 'Spam', no_show: 'Did not show up', other: 'Other' },
  },
}

export const LOCALES = { ru: 'Рус', kk: 'Қаз', en: 'Eng' }
const DATE_LOCALES = { ru: 'ru-RU', kk: 'kk-KZ', en: 'en-GB' }

function initialLocale() {
  try {
    const saved = localStorage.getItem('lang')
    if (saved && dict[saved]) return saved
  } catch {
    // Storage may be unavailable.
  }
  const tgLang = window.Telegram?.WebApp?.initDataUnsafe?.user?.language_code
  return tgLang === 'kk' || tgLang === 'en' ? tgLang : 'ru'
}

export const locale = ref(initialLocale())

// Vocabulary maps are reactive objects so `levels[x]` in templates follows the locale.
export const levels = reactive({})
export const formats = reactive({})
export const kinds = reactive({})
export const categories = reactive({})
export const lessonStatuses = reactive({})
export const reportReasons = reactive({})
export const weekdaysShort = reactive([])

function applyVocab() {
  const v = vocab[locale.value]
  Object.assign(levels, v.levels)
  Object.assign(formats, v.formats)
  Object.assign(kinds, v.kinds)
  Object.assign(categories, v.categories)
  Object.assign(lessonStatuses, v.lessonStatuses)
  Object.assign(reportReasons, v.reportReasons)
  weekdaysShort.splice(0, weekdaysShort.length, ...v.weekdaysShort)
  document.documentElement.lang = locale.value
}
applyVocab()

export function setLocale(l) {
  if (!dict[l]) return
  locale.value = l
  try {
    localStorage.setItem('lang', l)
  } catch {
    // Storage may be unavailable.
  }
  applyVocab()
}

// Russian-only vocabularies (teacher and admin screens).
export const languages = { kk: 'Қазақша', ru: 'Русский', en: 'English' }
export const statuses = { draft: 'Черновик', pending: 'На проверке', live: 'Опубликован', hidden: 'Скрыт', blocked: 'Заблокирован' }
export const plans = { founding: 'Основатель', trial: 'Пробный период', basic: 'Базовый', featured: 'Премиум' }
export const weekdaysLong = ['Воскресенье', 'Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота']

// t returns the text for key in the current locale; {name} placeholders are filled from vars.
export function t(key, vars) {
  let s = dict[locale.value][key] ?? dict.ru[key] ?? key
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.replace(`{${k}}`, v)
  return s
}

export function subjectName(s) {
  if (!s) return ''
  return s[`name_${locale.value}`] || s.name_ru
}

export const placeName = subjectName

export function formatPrice(n) {
  if (n === null || n === undefined) return ''
  return `${Number(n).toLocaleString('ru-RU')} ₸`
}

const TZ = 'Asia/Almaty'

export function formatDate(iso) {
  return new Date(iso).toLocaleDateString(DATE_LOCALES[locale.value], { day: 'numeric', month: 'long', year: 'numeric', timeZone: TZ })
}

// lastDay formats the last included day of a period whose end is exclusive (local midnight).
export function lastDay(iso) {
  return formatDate(new Date(new Date(iso).getTime() - 1000).toISOString())
}

export function formatTime(iso) {
  return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', timeZone: TZ })
}

export function formatDay(iso) {
  return new Date(iso).toLocaleDateString(DATE_LOCALES[locale.value], { weekday: 'short', day: 'numeric', month: 'long', timeZone: TZ })
}

export function formatLessonTime(l) {
  return `${formatDay(l.starts_at)}, ${formatTime(l.starts_at)}–${formatTime(l.ends_at)}`
}

// dayLabel formats a YYYY-MM-DD date as "Пн 9".
export function dayLabel(date) {
  const d = new Date(`${date}T12:00:00+05:00`)
  return `${weekdaysShort[d.getUTCDay()]} ${d.getUTCDate()}`
}
