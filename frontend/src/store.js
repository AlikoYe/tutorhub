// App-wide state: the current user, the subject taxonomy and vocabularies.
import { reactive } from 'vue'
import { api } from '@/api/client'

export const store = reactive({
  me: null,
  meError: null,
  subjects: [],
  subjectsById: {},
  meta: null,
})

export async function loadMe() {
  try {
    store.me = await api.get('/me')
    store.meError = null
  } catch (e) {
    store.meError = e
  }
  return store.me
}

export async function loadSubjects() {
  if (store.subjects.length) return store.subjects
  const list = await api.get('/subjects')
  store.subjects = list
  store.subjectsById = Object.fromEntries(list.map((s) => [s.id, s]))
  return list
}

export async function loadMeta() {
  if (!store.meta) store.meta = await api.get('/meta')
  return store.meta
}

export function districtName(city, slug) {
  const c = store.meta?.cities.find((x) => x.slug === city)
  return c?.districts.find((d) => d.slug === slug)?.name_ru || ''
}

export function cityName(slug) {
  return store.meta?.cities.find((x) => x.slug === slug)?.name_ru || slug
}
