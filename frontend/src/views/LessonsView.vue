<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import { t } from '@/i18n'
import { confirmDialog, alertDialog, haptic } from '@/telegram'
import LessonCard from '@/components/LessonCard.vue'
import AsyncState from '@/components/AsyncState.vue'

const scope = ref('upcoming')
const items = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await api.get('/lessons/my', { scope: scope.value })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function cancel(l) {
  if (!(await confirmDialog('Отменить урок?'))) return
  try {
    await api.post(`/lessons/${l.id}/cancel`, {})
    haptic('success')
    load()
  } catch (e) {
    alertDialog(e.message)
  }
}

function setScope(s) {
  scope.value = s
  load()
}

onMounted(load)
</script>

<template>
  <h1>{{ t('tabLessons') }}</h1>
  <div class="row" style="margin-bottom: 10px">
    <button class="chip" :class="{ on: scope === 'upcoming' }" @click="setScope('upcoming')">Предстоящие</button>
    <button class="chip" :class="{ on: scope === 'past' }" @click="setScope('past')">Прошедшие</button>
  </div>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">
      <p>Здесь будут ваши уроки.</p>
      <router-link to="/" class="btn">Найти преподавателя</router-link>
    </div>
    <LessonCard v-for="l in items" :key="l.id" :l="l">
      <button v-if="l.can_cancel" class="btn secondary small" @click="cancel(l)">Отменить</button>
    </LessonCard>
  </AsyncState>
</template>
