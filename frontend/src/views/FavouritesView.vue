<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import { loadMeta } from '@/store'
import { t } from '@/i18n'
import SpecialistCard from '@/components/SpecialistCard.vue'
import AsyncState from '@/components/AsyncState.vue'

const items = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    await loadMeta().catch(() => {})
    items.value = await api.get('/favourites')
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>{{ t('tabFavourites') }}</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">
      <p>Сохраняйте понравившихся преподавателей кнопкой ☆ в профиле.</p>
      <router-link to="/" class="btn">{{ t('tabCatalogue') }}</router-link>
    </div>
    <SpecialistCard v-for="s in items" :key="s.id" :s="s" />
  </AsyncState>
</template>
