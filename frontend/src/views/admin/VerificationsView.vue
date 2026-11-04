<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, fetchBlob } from '@/api/client'
import { useBackButton, alertDialog, haptic } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/admin'))

const items = ref([])
const loading = ref(true)
const error = ref(null)
const notes = reactive({})

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await api.get('/admin/verifications')
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function open(d) {
  try {
    const blob = await fetchBlob(`/admin/documents/${d.id}/file`)
    window.open(URL.createObjectURL(blob), '_blank')
  } catch (e) {
    alertDialog(e.message)
  }
}

async function decide(x, approve) {
  try {
    await api.post(`/admin/specialists/${x.specialist.id}/verification`, { approve, note: notes[x.specialist.id] || '' })
    haptic('success')
    load()
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(load)
</script>

<template>
  <h1>Проверка документов</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">Нет запросов</div>
    <div v-for="x in items" :key="x.specialist.id" class="card stack">
      <router-link :to="`/specialists/${x.specialist.id}`"><strong>{{ x.specialist.display_name }}</strong></router-link>
      <div class="hint">{{ x.specialist.education }}</div>
      <button v-for="d in x.documents" :key="d.id" class="btn secondary small" @click="open(d)">📄 {{ d.original_name }}</button>
      <input v-model="notes[x.specialist.id]" maxlength="500" placeholder="Комментарий (при отказе увидит преподаватель)" />
      <div class="row">
        <button class="btn small" @click="decide(x, true)">✅ Подтвердить</button>
        <button class="btn danger small" @click="decide(x, false)">❌ Отказать</button>
      </div>
    </div>
  </AsyncState>
</template>
