<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import { formatDate } from '@/i18n'
import { alertDialog, haptic } from '@/telegram'
import Stars from './Stars.vue'

const props = defineProps({ specialistId: { type: Number, required: true }, canAct: Boolean })
const emit = defineEmits(['changed'])

const reviews = ref([])
const mine = ref(null)
const editing = ref(false)
const rating = ref(5)
const text = ref('')

async function load() {
  reviews.value = await api.get(`/specialists/${props.specialistId}/reviews`)
  if (props.canAct) mine.value = await api.get(`/specialists/${props.specialistId}/review`)
}

function edit() {
  rating.value = mine.value.review?.rating || 5
  text.value = mine.value.review?.text || ''
  editing.value = true
}

async function save() {
  try {
    await api.put(`/specialists/${props.specialistId}/review`, { rating: rating.value, text: text.value })
    haptic('success')
    editing.value = false
    await load()
    emit('changed')
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(() => load().catch(() => {}))
</script>

<template>
  <div class="card">
    <h3 style="margin-top: 0">Отзывы</h3>
    <template v-if="mine?.can_review && !editing">
      <p v-if="mine.hidden" class="hint">Ваш отзыв скрыт модератором.</p>
      <button class="btn secondary small" @click="edit">{{ mine.review ? 'Изменить мой отзыв' : '✍️ Оставить отзыв' }}</button>
    </template>
    <div v-if="editing" class="stack">
      <div class="pick">
        <button v-for="i in 5" :key="i" class="star" :class="{ on: i <= rating }" @click="rating = i">★</button>
      </div>
      <textarea v-model="text" maxlength="2000" rows="3" placeholder="Что понравилось? Чего удалось достичь?" />
      <div class="row">
        <button class="btn small" @click="save">Сохранить</button>
        <button class="btn secondary small" @click="editing = false">Отмена</button>
      </div>
    </div>
    <p v-if="!reviews.length" class="hint">Пока нет отзывов. Отзыв можно оставить после проведённого урока.</p>
    <div v-for="r in reviews" :key="r.id" class="review">
      <div class="row between">
        <strong>{{ r.author_name || 'Ученик' }}</strong>
        <Stars :value="r.rating" />
      </div>
      <p v-if="r.text" class="pre">{{ r.text }}</p>
      <div class="hint">{{ formatDate(r.created_at) }}</div>
    </div>
  </div>
</template>

<style scoped>
.review { padding: 8px 0; border-top: 1px solid color-mix(in srgb, var(--hint) 15%, transparent); margin-top: 8px; }
.pre { white-space: pre-wrap; margin: 4px 0; }
.pick { display: flex; gap: 4px; }
.star { border: 0; background: none; font-size: 28px; color: color-mix(in srgb, var(--hint) 40%, transparent); cursor: pointer; padding: 0; }
.star.on { color: #f5a524; }
</style>
