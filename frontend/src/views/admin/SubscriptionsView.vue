<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { plans as planNames, lastDay } from '@/i18n'
import { useBackButton, alertDialog, haptic } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'
import AdminSubscriptionPanel from '@/components/AdminSubscriptionPanel.vue'

const router = useRouter()
useBackButton(() => router.push('/admin'))

const items = ref([])
const loading = ref(true)
const error = ref(null)
const open = ref(null)
const foundingUntil = ref('')

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await api.get('/admin/subscriptions/expiring', { days: 14 })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function setFounding() {
  try {
    const { updated } = await api.post('/admin/founding-period', { ends_on: foundingUntil.value })
    haptic('success')
    alertDialog(`Продлено тарифов «Основатель»: ${updated}`)
    load()
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(load)
</script>

<template>
  <h1>Подписки</h1>
  <div class="card">
    <h3 style="margin-top: 0">Период «Основатель»</h3>
    <p class="hint">Перенести дату окончания всех действующих тарифов «Основатель».</p>
    <div class="row">
      <input v-model="foundingUntil" type="date" />
      <button class="btn small" :disabled="!foundingUntil" @click="setFounding">Применить</button>
    </div>
  </div>
  <h2>Заканчиваются в ближайшие 14 дней</h2>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">Никого</div>
    <div v-for="x in items" :key="x.specialist_id" class="card">
      <div class="row between" @click="open = open === x.specialist_id ? null : x.specialist_id">
        <div>
          <strong>{{ x.display_name }}</strong>
          <div class="hint">{{ planNames[x.plan] }} до {{ lastDay(x.ends_at) }}<span v-if="x.owner.username"> · @{{ x.owner.username }}</span></div>
        </div>
        <span class="muted">{{ open === x.specialist_id ? '▴' : '▾' }}</span>
      </div>
      <AdminSubscriptionPanel v-if="open === x.specialist_id" :specialist-id="x.specialist_id" />
    </div>
  </AsyncState>
</template>
