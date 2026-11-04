// App-wide state: the current user and the subject taxonomy.
import { reactive } from 'vue'
import { api } from '@/api/client'

export const store = reactive({
  me: null,
  meError: null,
  subjects: [],
  subjectsById: {},
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
