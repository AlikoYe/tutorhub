<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { useBackButton, confirmDialog, alertDialog, haptic } from '@/telegram'
import LessonCard from '@/components/LessonCard.vue'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/teach'))

const scopes = { requests: 'Заявки', upcoming: 'Предстоящие', past: 'Прошедшие' }
const scope = ref('requests')
const items = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await api.get('/teacher/lessons', { scope: scope.value })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function act(l, action) {
  if (action === 'cancel' && !(await confirmDialog('Отменить урок? Ученик получит уведомление.'))) return
  try {
    await api.post(`/teacher/lessons/${l.id}/${action}`, action === 'cancel' ? {} : undefined)
    haptic('success')
    load()
  } catch (e) {
    alertDialog(e.message)
    load()
  }
}

onMounted(load)
</script>

<template>
  <h1>Уроки и заявки</h1>
  <div class="row" style="margin-bottom: 10px">
    <button v-for="(label, s) in scopes" :key="s" class="chip" :class="{ on: scope === s }" @click="scope = s; load()">{{ label }}</button>
  </div>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">Пока пусто</div>
    <LessonCard v-for="l in items" :key="l.id" :l="l" teacher>
      <template v-if="l.status === 'pending'">
        <button class="btn small" @click="act(l, 'confirm')">✅ Подтвердить</button>
        <button class="btn danger small" @click="act(l, 'decline')">❌ Отклонить</button>
      </template>
      <button v-else-if="l.can_cancel" class="btn secondary small" @click="act(l, 'cancel')">Отменить</button>
    </LessonCard>
  </AsyncState>
</template>
