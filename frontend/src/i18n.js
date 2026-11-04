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
  notFound: 'Страница не найдена',
  comingSoon: 'Скоро здесь появится этот раздел.',
  authFailed: 'Не удалось войти. Откройте приложение из Telegram.',
}

export function t(key) {
  return ru[key] ?? key
}

export function subjectName(s) {
  return s ? s.name_ru : ''
}
