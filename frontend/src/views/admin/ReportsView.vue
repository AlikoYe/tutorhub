<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { formatDate } from '@/i18n'
import { useBackButton, alertDialog } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'
import ChipSelect from '@/components/ChipSelect.vue'

const router = useRouter()
useBackButton(() => router.push('/admin'))

const reasons = { fake: 'Фейк', inappropriate: 'Неприемлемо', spam: 'Спам', no_show: 'Не пришёл', other: 'Другое' }
const status = ref('open')
const items = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await api.get('/admin/reports', { status: status.value })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function resolve(r, s) {
  try {
    await api.post(`/admin/reports/${r.id}/resolve`, { status: s })
    load()
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(load)
</script>

<template>
  <h1>Жалобы</h1>
  <ChipSelect v-model="status" :options="{ open: 'Открытые', resolved: 'Решённые', dismissed: 'Отклонённые' }" @update:model-value="(v) => { status = v || 'open'; load() }" />
  <div style="height: 10px" />
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">Нет жалоб</div>
    <div v-for="r in items" :key="r.id" class="card">
      <div class="row between">
        <router-link :to="`/specialists/${r.specialist.id}`"><strong>{{ r.specialist.display_name }}</strong></router-link>
        <span class="badge bad">{{ reasons[r.reason] }}</span>
      </div>
      <p v-if="r.text" style="margin: 6px 0">{{ r.text }}</p>
      <div class="hint">от {{ r.reporter.name }}<span v-if="r.reporter.username"> (@{{ r.reporter.username }})</span> · {{ formatDate(r.created_at) }}</div>
      <div v-if="r.status === 'open'" class="row" style="margin-top: 8px">
        <button class="btn small" @click="resolve(r, 'resolved')">Меры приняты</button>
        <button class="btn secondary small" @click="resolve(r, 'dismissed')">Отклонить</button>
      </div>
    </div>
  </AsyncState>
</template>
