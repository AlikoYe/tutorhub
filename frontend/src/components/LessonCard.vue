<script setup>
import { lessonStatuses, formatLessonTime, formatPrice } from '@/i18n'

defineProps({ l: { type: Object, required: true }, teacher: Boolean })

const badge = { pending: 'warn', confirmed: 'ok', completed: 'accent', declined: 'bad', cancelled: 'bad', expired: '' }
</script>

<template>
  <div class="card">
    <div class="row between">
      <strong>{{ l.subject.name_ru }}<span v-if="l.is_trial" class="hint"> · пробный</span></strong>
      <span class="badge" :class="badge[l.status]">{{ lessonStatuses[l.status] }}</span>
    </div>
    <div>🕖 {{ formatLessonTime(l) }}</div>
    <div v-if="teacher && l.student">
      🙋 {{ l.student.name || 'Ученик' }}<a v-if="l.student.username" :href="`https://t.me/${l.student.username}`"> @{{ l.student.username }}</a>
    </div>
    <div v-else>
      👤 <router-link :to="`/specialists/${l.specialist.id}`">{{ l.specialist.display_name }}</router-link>
    </div>
    <div v-if="l.specialist.address" class="hint">📍 {{ l.specialist.address }}</div>
    <div class="hint">💰 {{ l.price_kzt ? formatPrice(l.price_kzt) : 'бесплатно' }}</div>
    <div v-if="l.note" class="hint">💬 {{ l.note }}</div>
    <div v-if="l.cancel_reason" class="hint">Причина отмены: {{ l.cancel_reason }}</div>
    <div v-if="$slots.default" class="row wrap" style="margin-top: 8px">
      <slot />
    </div>
  </div>
</template>
