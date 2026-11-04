<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { plans as planNames, lastDay, formatDate, formatPrice } from '@/i18n'
import { useBackButton } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/teach'))

const state = ref(null)
const price = ref(null)
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    ;[state.value, price.value] = await Promise.all([api.get('/teacher/subscription'), api.get('/plans')])
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

const perks = {
  basic: 'Профиль в каталоге, заявки и расписание',
  featured: 'Всё из «Базового» + показ выше других в вашем предмете и районе + значок «Топ»',
}

onMounted(load)
</script>

<template>
  <h1>Подписка</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <template v-if="state">
      <div class="card">
        <div class="row between">
          <strong>{{ state.current ? planNames[state.current.plan] : 'Нет активного тарифа' }}</strong>
          <span class="badge" :class="state.listed ? 'ok' : 'bad'">{{ state.listed ? 'В каталоге' : 'Не в каталоге' }}</span>
        </div>
        <p v-if="state.coverage_ends_at" class="hint">Оплачено до {{ lastDay(state.coverage_ends_at) }} включительно.</p>
        <p v-else class="hint">Профиль не показывается в поиске. Ваша личная ссылка продолжает работать.</p>
      </div>

      <div v-for="p in price.plans" :key="p.plan" class="card">
        <div class="row between">
          <strong>{{ planNames[p.plan] }}</strong>
          <strong>{{ formatPrice(p.price_kzt) }} / мес</strong>
        </div>
        <p class="hint" style="margin-bottom: 0">{{ perks[p.plan] }}</p>
      </div>

      <div class="card">
        <h3 style="margin-top: 0">Как оплатить</h3>
        <p class="pre">{{ price.payment_instructions }}</p>
        <p class="hint">Без комиссий с уроков: ученики платят вам напрямую.</p>
      </div>

      <div v-if="state.history.length" class="card">
        <h3 style="margin-top: 0">История</h3>
        <div v-for="s in state.history" :key="s.id" class="row between hint">
          <span>{{ planNames[s.plan] }}</span>
          <span>{{ formatDate(s.starts_at) }} — {{ lastDay(s.ends_at) }}</span>
        </div>
      </div>
    </template>
  </AsyncState>
</template>

<style scoped>
.pre { white-space: pre-wrap; }
</style>
