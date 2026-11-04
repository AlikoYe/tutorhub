<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { formatPrice, formatTime, dayLabel } from '@/i18n'
import { haptic, alertDialog, useMainButton } from '@/telegram'

const props = defineProps({ specialist: { type: Object, required: true } })
const router = useRouter()

const subjectId = ref(props.specialist.subjects[0]?.subject_id || null)
const slots = ref(null)
const dayIndex = ref(0)
const picked = ref(null)
const trial = ref(false)
const note = ref('')
const loading = ref(false)
const sending = ref(false)
const error = ref('')

const offer = computed(() => props.specialist.subjects.find((o) => o.subject_id === subjectId.value))
const day = computed(() => slots.value?.days[dayIndex.value])
const price = computed(() => {
  if (trial.value) return props.specialist.trial_kind === 'paid' ? props.specialist.trial_price : 0
  return offer.value?.price_kzt
})

async function load() {
  if (!subjectId.value) return
  loading.value = true
  picked.value = null
  try {
    slots.value = await api.get(`/specialists/${props.specialist.id}/slots`, { subject_id: subjectId.value, days: 14 })
    const first = slots.value.days.findIndex((d) => d.slots.length)
    dayIndex.value = first >= 0 ? first : 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function book() {
  if (!picked.value || sending.value) return
  sending.value = true
  error.value = ''
  try {
    await api.post('/lessons', {
      specialist_id: props.specialist.id,
      subject_id: subjectId.value,
      starts_at: picked.value.starts_at,
      is_trial: trial.value,
      note: note.value,
    })
    haptic('success')
    alertDialog('Заявка отправлена! Преподаватель получит её в Telegram и ответит.')
    router.push('/lessons')
  } catch (e) {
    haptic('error')
    error.value = e.message
    if (e.code === 'slot_taken') load()
  } finally {
    sending.value = false
  }
}

const { native } = useMainButton(
  () => ({
    visible: Boolean(picked.value),
    text: picked.value ? `Записаться на ${formatTime(picked.value.starts_at)}` : '',
    loading: sending.value,
  }),
  book,
)

watch(subjectId, load)
onMounted(load)
</script>

<template>
  <div class="card">
    <h3 style="margin-top: 0">Записаться на урок</h3>
    <select v-if="specialist.subjects.length > 1" v-model.number="subjectId" style="margin-bottom: 10px">
      <option v-for="o in specialist.subjects" :key="o.subject_id" :value="o.subject_id">
        {{ o.name_ru }} — {{ formatPrice(o.price_kzt) }}, {{ o.lesson_minutes }} мин
      </option>
    </select>
    <div v-if="loading" class="hint">Загрузка расписания…</div>
    <template v-else-if="slots">
      <div class="days">
        <button
          v-for="(d, i) in slots.days"
          :key="d.date"
          class="chip"
          :class="{ on: i === dayIndex }"
          :disabled="!d.slots.length"
          @click="dayIndex = i; picked = null"
        >
          {{ dayLabel(d.date) }}
        </button>
      </div>
      <div v-if="day && day.slots.length" class="row wrap" style="gap: 6px; margin-top: 8px">
        <button
          v-for="s in day.slots"
          :key="s.starts_at"
          class="chip"
          :class="{ on: picked?.starts_at === s.starts_at }"
          @click="picked = s"
        >
          {{ formatTime(s.starts_at) }}
        </button>
      </div>
      <p v-else class="hint">Нет свободного времени в ближайшие две недели.</p>
    </template>

    <template v-if="picked">
      <label v-if="specialist.trial_kind !== 'none'" class="row" style="margin-top: 10px">
        <input v-model="trial" type="checkbox" style="width: auto" />
        Пробный урок ({{ specialist.trial_kind === 'free' ? 'бесплатно' : formatPrice(specialist.trial_price) }})
      </label>
      <textarea v-model="note" maxlength="500" rows="2" placeholder="Сообщение преподавателю: класс, цель, уровень" style="margin-top: 8px" />
      <p class="hint">Стоимость: {{ price ? formatPrice(price) : 'бесплатно' }}. Оплата напрямую преподавателю.</p>
      <button v-if="!native" class="btn block" :disabled="sending" @click="book">Записаться на {{ formatTime(picked.starts_at) }}</button>
    </template>
    <p v-if="error" class="error-text">{{ error }}</p>
  </div>
</template>

<style scoped>
.days { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 4px; }
.days .chip { flex: none; }
.days .chip:disabled { opacity: 0.4; }
</style>
