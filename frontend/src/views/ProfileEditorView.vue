<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { store, loadMeta, loadSubjects, loadMe } from '@/store'
import { t, levels, formats, languages, kinds, categories } from '@/i18n'
import { useBackButton, useMainButton, haptic, alertDialog } from '@/telegram'
import ChipSelect from '@/components/ChipSelect.vue'
import Avatar from '@/components/Avatar.vue'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/teach'))

const form = reactive({
  kind: 'teacher',
  display_name: '',
  bio: '',
  education: '',
  experience_years: 0,
  city: 'almaty',
  district: '',
  address: '',
  formats: [],
  languages: [],
  trial_kind: 'none',
  trial_price: null,
  lesson_minutes: 60,
  subjects: [],
})
const profile = ref(null)
const loading = ref(true)
const error = ref(null)
const saving = ref(false)
const message = ref('')
const addSubjectId = ref('')

const districts = computed(() => store.meta?.cities.find((c) => c.slug === form.city)?.districts || [])
const subjectGroups = computed(() => {
  const used = new Set(form.subjects.map((s) => s.subject_id))
  const groups = {}
  for (const s of store.subjects) if (!used.has(s.id)) (groups[s.category] ||= []).push(s)
  return groups
})
const trialKinds = { none: 'Нет', free: 'Бесплатный', paid: 'Платный' }

function fill(p) {
  profile.value = p
  for (const k of Object.keys(form)) {
    if (k === 'subjects') form.subjects = p.subjects.map((o) => ({ subject_id: o.subject_id, levels: [...o.levels], price_kzt: o.price_kzt }))
    else if (p[k] !== undefined && p[k] !== null) form[k] = Array.isArray(p[k]) ? [...p[k]] : p[k]
  }
  if (p.address === null) form.address = ''
}

async function load() {
  loading.value = true
  error.value = null
  try {
    await Promise.all([loadMeta(), loadSubjects()])
    fill(await api.get('/my/specialist'))
  } catch (e) {
    if (e.status !== 404) error.value = e
  } finally {
    loading.value = false
  }
}

function addSubject() {
  if (!addSubjectId.value) return
  form.subjects.push({ subject_id: Number(addSubjectId.value), levels: [], price_kzt: 5000 })
  addSubjectId.value = ''
}

function payload() {
  return {
    ...form,
    experience_years: Number(form.experience_years) || 0,
    lesson_minutes: Number(form.lesson_minutes) || 60,
    trial_price: form.trial_kind === 'paid' ? Number(form.trial_price) || 0 : null,
    subjects: form.subjects.map((s) => ({ ...s, price_kzt: Number(s.price_kzt) || 0 })),
  }
}

async function save() {
  saving.value = true
  message.value = ''
  try {
    fill(await api.put('/my/specialist', payload()))
    message.value = t('saved')
    haptic('success')
    loadMe()
    return true
  } catch (e) {
    haptic('error')
    alertDialog(e.message)
    return false
  } finally {
    saving.value = false
  }
}

async function saveAndSubmit() {
  if (!(await save())) return
  try {
    fill(await api.post('/my/specialist/submit'))
    haptic('success')
    router.push('/teach')
  } catch (e) {
    alertDialog(e.message)
  }
}

async function onPhoto(ev) {
  const file = ev.target.files?.[0]
  if (!file) return
  if (!profile.value && !(await save())) return
  try {
    fill(await api.upload('/my/specialist/photo', file))
    haptic('success')
  } catch (e) {
    alertDialog(e.message)
  }
}

const { native } = useMainButton(() => ({ visible: !loading.value, text: t('save'), loading: saving.value }), save)

onMounted(load)
</script>

<template>
  <h1>Анкета</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div class="card row">
      <Avatar :url="profile?.photo_url" :name="form.display_name" big />
      <label class="btn secondary small">
        📷 Загрузить фото
        <input type="file" accept="image/jpeg,image/png,image/webp" hidden @change="onPhoto" />
      </label>
    </div>

    <div class="card">
      <div class="field"><span class="hint">Кто вы</span><ChipSelect v-model="form.kind" :options="kinds" /></div>
      <label class="field"><span>Имя для учеников</span><input v-model="form.display_name" maxlength="80" /></label>
      <label class="field"><span>О себе (минимум 30 символов)</span><textarea v-model="form.bio" maxlength="2000" rows="5" /></label>
      <label class="field"><span>Образование</span><textarea v-model="form.education" maxlength="1000" rows="2" /></label>
      <label class="field"><span>Опыт, лет</span><input v-model="form.experience_years" type="number" min="0" max="70" inputmode="numeric" /></label>
    </div>

    <div class="card">
      <h3 style="margin-top: 0">Предметы и цены</h3>
      <div v-for="(s, i) in form.subjects" :key="s.subject_id" class="subject">
        <div class="row between">
          <strong>{{ store.subjectsById[s.subject_id]?.name_ru }}</strong>
          <button class="btn secondary small" @click="form.subjects.splice(i, 1)">✕</button>
        </div>
        <ChipSelect v-model="s.levels" :options="levels" multiple />
        <label class="row" style="margin-top: 6px">
          <span class="hint">Цена за урок, ₸</span>
          <input v-model="s.price_kzt" type="number" min="0" inputmode="numeric" style="max-width: 140px" />
        </label>
      </div>
      <div class="row">
        <select v-model="addSubjectId" class="grow">
          <option value="">+ Добавить предмет</option>
          <optgroup v-for="(list, cat) in subjectGroups" :key="cat" :label="categories[cat]">
            <option v-for="s in list" :key="s.id" :value="String(s.id)">{{ s.name_ru }}</option>
          </optgroup>
        </select>
        <button class="btn small" :disabled="!addSubjectId" @click="addSubject">Добавить</button>
      </div>
    </div>

    <div class="card">
      <div class="field"><span class="hint">Формат занятий</span><ChipSelect v-model="form.formats" :options="formats" multiple /></div>
      <div class="field"><span class="hint">Язык занятий</span><ChipSelect v-model="form.languages" :options="languages" multiple /></div>
      <label class="field">
        <span>Район</span>
        <select v-model="form.district">
          <option value="">—</option>
          <option v-for="d in districts" :key="d.slug" :value="d.slug">{{ d.name_ru }}</option>
        </select>
      </label>
      <label class="field"><span>Адрес (видят только ученики с подтверждённым уроком)</span><input v-model="form.address" maxlength="200" /></label>
      <label class="field">
        <span>Длительность урока</span>
        <select v-model.number="form.lesson_minutes">
          <option v-for="m in [30, 45, 60, 75, 90, 120]" :key="m" :value="m">{{ m }} мин</option>
        </select>
      </label>
      <div class="field"><span class="hint">Пробный урок</span><ChipSelect v-model="form.trial_kind" :options="trialKinds" /></div>
      <label v-if="form.trial_kind === 'paid'" class="field"><span>Цена пробного урока, ₸</span><input v-model="form.trial_price" type="number" min="0" /></label>
    </div>

    <p v-if="message" class="hint" style="text-align: center">{{ message }}</p>
    <div class="stack">
      <button v-if="!native" class="btn block" :disabled="saving" @click="save">{{ t('save') }}</button>
      <button v-if="!profile || profile.status === 'draft'" class="btn secondary block" :disabled="saving" @click="saveAndSubmit">
        Сохранить и отправить на проверку
      </button>
    </div>
  </AsyncState>
</template>

<style scoped>
.subject { padding: 10px 0; border-bottom: 1px solid color-mix(in srgb, var(--hint) 15%, transparent); margin-bottom: 10px; }
</style>
