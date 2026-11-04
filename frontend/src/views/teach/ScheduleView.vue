<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { weekdaysLong, formatDate } from '@/i18n'
import { useBackButton, useMainButton, alertDialog, haptic } from '@/telegram'
import AsyncState from '@/components/AsyncState.vue'

const router = useRouter()
useBackButton(() => router.push('/teach'))

// Monday first.
const order = [1, 2, 3, 4, 5, 6, 0]
const rules = ref([])
const daysOff = ref([])
const newDayOff = ref('')
const loading = ref(true)
const error = ref(null)
const saving = ref(false)
const dirty = ref(false)

const byDay = computed(() => Object.fromEntries(order.map((wd) => [wd, rules.value.filter((r) => r.weekday === wd)])))

function apply(tpl) {
  rules.value = tpl.rules.map((r) => ({ ...r }))
  daysOff.value = tpl.days_off
  dirty.value = false
}

async function load() {
  loading.value = true
  error.value = null
  try {
    apply(await api.get('/teacher/availability'))
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

function add(wd) {
  const last = byDay.value[wd].at(-1)
  rules.value.push({ weekday: wd, starts_at: last ? last.ends_at : '16:00', ends_at: last ? '22:00' : '20:00' })
  dirty.value = true
}

function remove(r) {
  rules.value = rules.value.filter((x) => x !== r)
  dirty.value = true
}

function copyToWeekdays(wd) {
  const src = byDay.value[wd]
  rules.value = rules.value.filter((r) => r.weekday === wd || ![1, 2, 3, 4, 5].includes(r.weekday))
  for (const d of [1, 2, 3, 4, 5]) if (d !== wd) for (const r of src) rules.value.push({ ...r, weekday: d })
  dirty.value = true
}

async function save() {
  saving.value = true
  try {
    apply(await api.put('/teacher/availability/rules', { rules: rules.value }))
    haptic('success')
  } catch (e) {
    haptic('error')
    alertDialog(e.message)
  } finally {
    saving.value = false
  }
}

async function setDayOff(day, closed) {
  try {
    const tpl = closed ? await api.put(`/teacher/availability/days-off/${day}`) : await api.del(`/teacher/availability/days-off/${day}`)
    daysOff.value = tpl.days_off
    newDayOff.value = ''
  } catch (e) {
    alertDialog(e.message)
  }
}

const times = []
for (let m = 0; m <= 24 * 60; m += 15) times.push(`${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`)

const { native } = useMainButton(() => ({ visible: dirty.value, text: 'Сохранить расписание', loading: saving.value }), save)

onMounted(load)
</script>

<template>
  <h1>Расписание</h1>
  <p class="hint">Укажите, когда вы готовы проводить уроки. Ученики увидят свободное время с шагом 30 минут.</p>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-for="wd in order" :key="wd" class="card">
      <div class="row between">
        <strong>{{ weekdaysLong[wd] }}</strong>
        <div class="row">
          <button v-if="byDay[wd].length && wd >= 1 && wd <= 5" class="btn secondary small" title="Скопировать на будни" @click="copyToWeekdays(wd)">⇉ Пн–Пт</button>
          <button class="btn secondary small" @click="add(wd)">+</button>
        </div>
      </div>
      <p v-if="!byDay[wd].length" class="hint" style="margin: 4px 0 0">Выходной</p>
      <div v-for="(r, i) in byDay[wd]" :key="i" class="row" style="margin-top: 6px">
        <select v-model="r.starts_at" @change="dirty = true">
          <option v-for="tm in times.slice(0, -1)" :key="tm" :value="tm">{{ tm }}</option>
        </select>
        —
        <select v-model="r.ends_at" @change="dirty = true">
          <option v-for="tm in times.slice(1)" :key="tm" :value="tm">{{ tm }}</option>
        </select>
        <button class="btn secondary small" @click="remove(r)">✕</button>
      </div>
    </div>
    <button v-if="!native && dirty" class="btn block" :disabled="saving" @click="save">Сохранить расписание</button>

    <div class="card">
      <h3 style="margin-top: 0">Дни отдыха</h3>
      <div v-for="d in daysOff" :key="d" class="row between">
        <span>{{ formatDate(d) }}</span>
        <button class="btn secondary small" @click="setDayOff(d, false)">✕</button>
      </div>
      <div class="row" style="margin-top: 8px">
        <input v-model="newDayOff" type="date" />
        <button class="btn small" :disabled="!newDayOff" @click="setDayOff(newDayOff, true)">Добавить</button>
      </div>
    </div>
  </AsyncState>
</template>
