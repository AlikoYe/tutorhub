<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { formatDate } from '@/i18n'
import { useBackButton, alertDialog, haptic, confirmDialog } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/teach'))

const profile = ref(null)
const docs = ref([])
const loading = ref(true)
const error = ref(null)
const busy = ref(false)

const statusText = {
  none: 'Значок «Проверен» показывает ученикам, что администратор проверил ваш диплом или сертификаты.',
  requested: 'Документы на проверке. Мы напишем в Telegram, когда проверим.',
  verified: '✅ Вы проверены — значок показывается в профиле и в поиске.',
  rejected: 'Проверка не пройдена. Загрузите другие документы и отправьте запрос снова.',
}

async function load() {
  loading.value = true
  error.value = null
  try {
    ;[profile.value, docs.value] = await Promise.all([api.get('/my/specialist'), api.get('/my/specialist/documents')])
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function upload(ev) {
  const file = ev.target.files?.[0]
  ev.target.value = ''
  if (!file) return
  busy.value = true
  try {
    await api.upload('/my/specialist/documents', file)
    docs.value = await api.get('/my/specialist/documents')
    haptic('success')
  } catch (e) {
    alertDialog(e.message)
  } finally {
    busy.value = false
  }
}

async function remove(d) {
  if (!(await confirmDialog(`Удалить «${d.original_name}»?`))) return
  await api.del(`/my/specialist/documents/${d.id}`)
  docs.value = docs.value.filter((x) => x.id !== d.id)
}

async function request() {
  busy.value = true
  try {
    profile.value = await api.post('/my/specialist/verification')
    haptic('success')
  } catch (e) {
    alertDialog(e.message)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>Проверка документов</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <template v-if="profile">
      <div class="card">
        <p style="margin-top: 0">{{ statusText[profile.verification_status] }}</p>
        <p v-if="profile.verification_status === 'rejected' && profile.verification_note" class="error-text">{{ profile.verification_note }}</p>
        <p class="hint">Документы видят только администраторы TutorHub. Форматы: PDF, JPG, PNG, до 10 МБ, не больше 5 файлов.</p>
      </div>
      <div class="card">
        <div v-for="d in docs" :key="d.id" class="row between doc">
          <span>📄 {{ d.original_name }} <span class="hint">· {{ formatDate(d.created_at) }}</span></span>
          <button class="btn secondary small" @click="remove(d)">✕</button>
        </div>
        <p v-if="!docs.length" class="hint">Документов пока нет.</p>
        <label class="btn secondary block" :class="{ disabled: busy }" style="margin-top: 8px">
          + Загрузить документ
          <input type="file" accept="application/pdf,image/jpeg,image/png" hidden :disabled="busy" @change="upload" />
        </label>
      </div>
      <button
        v-if="profile.verification_status === 'none' || profile.verification_status === 'rejected'"
        class="btn block"
        :disabled="busy || !docs.length"
        @click="request"
      >
        Отправить на проверку
      </button>
    </template>
  </AsyncState>
</template>

<style scoped>
.doc { padding: 6px 0; }
</style>
