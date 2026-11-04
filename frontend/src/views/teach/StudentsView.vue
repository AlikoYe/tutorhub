<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { formatDay, formatTime } from '@/i18n'
import { useBackButton, alertDialog, haptic } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/teach'))

const items = ref([])
const loading = ref(true)
const error = ref(null)
const editing = ref(null)
const draft = ref('')

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await api.get('/teacher/students')
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

function edit(s) {
  editing.value = s.telegram_id
  draft.value = s.note
}

async function saveNote(s) {
  try {
    await api.put(`/teacher/students/${s.telegram_id}/note`, { note: draft.value })
    s.note = draft.value.trim()
    editing.value = null
    haptic('success')
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(load)
</script>

<template>
  <h1>Ученики</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">Ученики появятся после первых заявок.</div>
    <div v-for="s in items" :key="s.telegram_id" class="card">
      <div class="row between">
        <strong>{{ s.name || 'Ученик' }}</strong>
        <a v-if="s.username" :href="`https://t.me/${s.username}`">@{{ s.username }}</a>
      </div>
      <div class="hint">Уроков: {{ s.lessons }} · заявок: {{ s.requests }}</div>
      <div v-if="s.next_lesson_at" class="hint">Следующий урок: {{ formatDay(s.next_lesson_at) }}, {{ formatTime(s.next_lesson_at) }}</div>
      <div v-if="editing === s.telegram_id" class="stack" style="margin-top: 8px">
        <textarea v-model="draft" maxlength="2000" rows="3" placeholder="Заметка видна только вам" />
        <div class="row">
          <button class="btn small" @click="saveNote(s)">Сохранить</button>
          <button class="btn secondary small" @click="editing = null">Отмена</button>
        </div>
      </div>
      <p v-else class="hint" style="margin: 6px 0 0; cursor: pointer" @click="edit(s)">📝 {{ s.note || 'Добавить заметку' }}</p>
    </div>
  </AsyncState>
</template>
