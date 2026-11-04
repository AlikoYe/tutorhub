<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { useBackButton } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/teach'))

const data = ref(null)
const loading = ref(true)
const error = ref(null)

const metrics = [
  { key: 'search_impressions', label: 'Показы в поиске' },
  { key: 'views', label: 'Просмотры профиля' },
  { key: 'contacts', label: 'Написали в Telegram' },
  { key: 'requests', label: 'Заявки на урок' },
  { key: 'lessons', label: 'Уроки' },
]

const current = computed(() => data.value?.weeks.at(-1))
const previous = computed(() => data.value?.weeks.at(-2))
const max = computed(() => Object.fromEntries(metrics.map((m) => [m.key, Math.max(1, ...(data.value?.weeks || []).map((w) => w[m.key]))])))

function delta(key) {
  if (!previous.value) return ''
  const d = current.value[key] - previous.value[key]
  return d === 0 ? '' : d > 0 ? `+${d}` : String(d)
}

function weekLabel(iso) {
  return new Date(`${iso}T12:00:00+05:00`).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

async function load() {
  loading.value = true
  error.value = null
  try {
    data.value = await api.get('/teacher/stats', { weeks: 8 })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>Статистика</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <template v-if="data">
      <div class="card">
        <div class="hint">Эта неделя (с {{ weekLabel(current.week_start) }})</div>
        <div v-for="m in metrics" :key="m.key" class="row between metric">
          <span>{{ m.label }}</span>
          <span><strong>{{ current[m.key] }}</strong> <span class="hint">{{ delta(m.key) }}</span></span>
        </div>
      </div>
      <div v-for="m in metrics" :key="m.key" class="card">
        <div class="row between"><strong>{{ m.label }}</strong><span class="hint">за 8 недель: {{ data.totals[m.key] }}</span></div>
        <div class="bars">
          <div v-for="w in data.weeks" :key="w.week_start" class="bar-col" :title="`${weekLabel(w.week_start)}: ${w[m.key]}`">
            <div class="bar" :style="{ height: `${(w[m.key] / max[m.key]) * 100}%` }" />
            <span class="bar-label">{{ weekLabel(w.week_start).split(' ')[0] }}</span>
          </div>
        </div>
      </div>
    </template>
  </AsyncState>
</template>

<style scoped>
.metric { padding: 6px 0; }
.bars { display: flex; align-items: flex-end; gap: 6px; height: 90px; margin-top: 8px; }
.bar-col { flex: 1; height: 100%; display: flex; flex-direction: column; justify-content: flex-end; align-items: center; }
.bar { width: 100%; min-height: 2px; background: var(--accent); border-radius: 4px 4px 0 0; }
.bar-label { font-size: 10px; color: var(--hint); margin-top: 2px; }
</style>
