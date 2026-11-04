<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '@/api/client'
import { plans as planNames, lastDay } from '@/i18n'
import { alertDialog, haptic, confirmDialog } from '@/telegram'

const props = defineProps({ specialistId: { type: Number, required: true } })
const state = ref(null)
const form = reactive({ plan: 'basic', mode: 'months', months: 1, ends_on: '', note: '' })

async function load() {
  state.value = await api.get(`/admin/specialists/${props.specialistId}/subscriptions`)
}

async function activate() {
  const body = { plan: form.plan, note: form.note }
  if (form.mode === 'months') body.months = Number(form.months)
  else body.ends_on = form.ends_on
  try {
    state.value = await api.post(`/admin/specialists/${props.specialistId}/subscriptions`, body)
    form.note = ''
    haptic('success')
  } catch (e) {
    alertDialog(e.message)
  }
}

async function end(s) {
  if (!(await confirmDialog('Завершить эту подписку сейчас?'))) return
  try {
    state.value = await api.post(`/admin/subscriptions/${s.id}/end`)
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(() => load().catch((e) => alertDialog(e.message)))
</script>

<template>
  <div v-if="state" class="stack sub-panel">
    <div class="row between">
      <strong>Подписка</strong>
      <span class="badge" :class="state.listed ? 'ok' : 'bad'">{{ state.listed ? 'в каталоге' : 'не в каталоге' }}</span>
    </div>
    <div v-for="s in state.history.slice(0, 5)" :key="s.id" class="row between hint">
      <span>{{ planNames[s.plan] }} до {{ lastDay(s.ends_at) }}<span v-if="s.note"> · {{ s.note }}</span></span>
      <button v-if="new Date(s.ends_at) > new Date()" class="btn secondary small" @click="end(s)">Завершить</button>
    </div>
    <div class="row">
      <select v-model="form.plan">
        <option v-for="(label, p) in planNames" :key="p" :value="p">{{ label }}</option>
      </select>
      <select v-model="form.mode" style="max-width: 40%">
        <option value="months">на месяцы</option>
        <option value="date">до даты</option>
      </select>
    </div>
    <input v-if="form.mode === 'months'" v-model="form.months" type="number" min="1" max="24" />
    <input v-else v-model="form.ends_on" type="date" />
    <input v-model="form.note" maxlength="300" placeholder="Заметка (например, номер платежа Kaspi)" />
    <button class="btn block" @click="activate">Активировать / продлить</button>
  </div>
</template>

<style scoped>
.sub-panel { border-top: 1px solid color-mix(in srgb, var(--hint) 20%, transparent); padding-top: 10px; margin-top: 10px; }
</style>
