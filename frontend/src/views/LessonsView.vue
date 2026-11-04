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
  if (!(await confirmDialog(t('cancelLessonConfirm')))) return
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
    <button class="chip" :class="{ on: scope === 'upcoming' }" @click="setScope('upcoming')">{{ t('upcoming') }}</button>
    <button class="chip" :class="{ on: scope === 'past' }" @click="setScope('past')">{{ t('past') }}</button>
  </div>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">
      <p>{{ t('lessonsEmpty') }}</p>
      <router-link to="/" class="btn">{{ t('findTeacher') }}</router-link>
    </div>
    <LessonCard v-for="l in items" :key="l.id" :l="l">
      <button v-if="l.can_cancel" class="btn secondary small" @click="cancel(l)">{{ t('cancelLesson') }}</button>
    </LessonCard>
  </AsyncState>
</template>
