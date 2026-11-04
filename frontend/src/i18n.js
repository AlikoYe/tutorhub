// UI texts. Russian is the default; kk/en come with the M5 UI polish.
const ru = {
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
  anySubject: 'Любой предмет',
  anyDistrict: 'Любой район',
  any: 'Любой',
  priceFrom: 'Цена от',
  priceTo: 'до',
  verifiedOnly: 'Только проверенные',
  nothingFound: 'Никого не нашли. Попробуйте изменить фильтры.',
  loadMore: 'Показать ещё',
  found: 'Найдено',
  perLesson: 'за урок',
  from: 'от',
  kzt: '₸',
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
}

export const levels = {
  primary: '1–4 класс',
  middle: '5–9 класс',
  high: '10–11 класс',
  ent: 'ЕНТ',
  university: 'Студенты',
  adult: 'Взрослые',
}

export const formats = {
  online: 'Онлайн',
  teacher_place: 'У преподавателя',
  student_home: 'На дому у ученика',
}

export const languages = { kk: 'Қазақша', ru: 'Русский', en: 'English' }

export const kinds = { teacher: 'Преподаватель', consultant: 'Консультант' }

export const statuses = {
  draft: 'Черновик',
  pending: 'На проверке',
  live: 'Опубликован',
  hidden: 'Скрыт',
  blocked: 'Заблокирован',
}

export const plans = {
  founding: 'Основатель',
  trial: 'Пробный период',
  basic: 'Базовый',
  featured: 'Премиум',
}

export const categories = {
  school: 'Школьные предметы',
  exam: 'Экзамены',
  language: 'Языки',
  it: 'IT и программирование',
  music: 'Музыка',
  arts: 'Творчество и хобби',
  consulting: 'Консультации',
  other: 'Другое',
}

export function t(key) {
  return ru[key] ?? key
}

export function subjectName(s) {
  return s ? s.name_ru : ''
}

export function placeName(p) {
  return p ? p.name_ru : ''
}

export function formatPrice(n) {
  if (n === null || n === undefined) return ''
  return `${Number(n).toLocaleString('ru-RU')} ₸`
}

export function formatDate(iso) {
  return new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
}

// lastDay formats the last included day of a period whose end is exclusive (local midnight).
export function lastDay(iso) {
  return formatDate(new Date(new Date(iso).getTime() - 1000).toISOString())
}
