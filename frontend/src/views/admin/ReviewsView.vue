<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { formatDate } from '@/i18n'
import { useBackButton, alertDialog } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'
import ChipSelect from '@/components/ChipSelect.vue'
import Stars from '@/components/Stars.vue'

const router = useRouter()
useBackButton(() => router.push('/admin'))

const status = ref('published')
const items = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await api.get('/admin/reviews', { status: status.value })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function toggle(r) {
  try {
    await api.post(`/admin/reviews/${r.id}/${r.status === 'published' ? 'hide' : 'unhide'}`)
    load()
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(load)
</script>

<template>
  <h1>Отзывы</h1>
  <ChipSelect v-model="status" :options="{ published: 'Опубликованные', hidden: 'Скрытые' }" @update:model-value="(v) => { status = v || 'published'; load() }" />
  <div style="height: 10px" />
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">Нет отзывов</div>
    <div v-for="r in items" :key="r.id" class="card">
      <div class="row between">
        <router-link :to="`/specialists/${r.specialist.id}`">{{ r.specialist.display_name }}</router-link>
        <Stars :value="r.rating" />
      </div>
      <p v-if="r.text" style="margin: 6px 0">{{ r.text }}</p>
      <div class="row between">
        <span class="hint">{{ r.author_name }} · {{ formatDate(r.created_at) }}</span>
        <button class="btn secondary small" @click="toggle(r)">{{ r.status === 'published' ? 'Скрыть' : 'Опубликовать' }}</button>
      </div>
    </div>
  </AsyncState>
</template>
