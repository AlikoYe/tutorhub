<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { statuses, plans } from '@/i18n'
import { useBackButton } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/admin'))

const d = ref(null)
const loading = ref(true)
const error = ref(null)

const series = computed(() =>
  d.value
    ? [
        { label: 'Поиски в день', data: d.value.searches_per_day },
        { label: 'Заявки в день', data: d.value.requests_per_day },
      ]
    : [],
)

function maxOf(list) {
  return Math.max(1, ...list.map((x) => x.count))
}

async function load() {
  loading.value = true
  error.value = null
  try {
    d.value = await api.get('/admin/dashboard', { days: 14 })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>Дашборд</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <template v-if="d">
      <div class="tiles">
        <div class="card tile"><div class="num">{{ d.listed }}</div><div class="hint">в каталоге</div></div>
        <div class="card tile"><div class="num">{{ d.specialists_by_status.pending }}</div><div class="hint">ждут проверки</div></div>
        <div class="card tile"><div class="num">{{ d.expiring_7d }}</div><div class="hint">тариф кончается за 7 дней</div></div>
        <div class="card tile"><div class="num">{{ d.users }}</div><div class="hint">пользователей</div></div>
      </div>

      <div class="card">
        <h3 style="margin-top: 0">Преподаватели по статусам</h3>
        <div v-for="(n, s) in d.specialists_by_status" :key="s" class="row between"><span>{{ statuses[s] }}</span><strong>{{ n }}</strong></div>
        <h3>Действующие тарифы</h3>
        <div v-for="(n, p) in d.plans_running" :key="p" class="row between"><span>{{ plans[p] }}</span><strong>{{ n }}</strong></div>
      </div>

      <div v-for="s in series" :key="s.label" class="card">
        <div class="row between"><strong>{{ s.label }}</strong><span class="hint">всего {{ s.data.reduce((a, x) => a + x.count, 0) }}</span></div>
        <div class="bars">
          <div v-for="x in s.data" :key="x.date" class="bar-col" :title="`${x.date}: ${x.count}`">
            <div class="bar" :style="{ height: `${(x.count / maxOf(s.data)) * 100}%` }" />
          </div>
        </div>
      </div>

      <div class="card">
        <h3 style="margin-top: 0">Спрос без предложения (30 дней)</h3>
        <p class="hint">Предметы, которые ищут, а преподавателей в каталоге мало или нет.</p>
        <div v-if="!d.unmet_demand.length" class="hint">Пока нет данных</div>
        <div v-for="x in d.unmet_demand" :key="x.subject_id" class="row between">
          <span>{{ x.name_ru }}</span>
          <span><strong>{{ x.searches }}</strong> поисков · {{ x.listed_specialists }} преп.</span>
        </div>
      </div>
    </template>
  </AsyncState>
</template>

<style scoped>
.tiles { display: grid; grid-template-columns: 1fr 1fr; gap: var(--gap); margin-bottom: var(--gap); }
.tile { margin: 0; }
.num { font-size: 26px; font-weight: 700; }
.bars { display: flex; align-items: flex-end; gap: 3px; height: 80px; margin-top: 8px; }
.bar-col { flex: 1; height: 100%; display: flex; align-items: flex-end; }
.bar { width: 100%; min-height: 2px; background: var(--accent); border-radius: 3px 3px 0 0; }
</style>
