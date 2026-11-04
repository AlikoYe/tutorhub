<script setup>
import { computed } from 'vue'
import { t, formats, formatPrice } from '@/i18n'
import { districtName } from '@/store'
import Avatar from './Avatar.vue'
import Stars from './Stars.vue'

const props = defineProps({ s: { type: Object, required: true }, subjectId: { type: Number, default: null } })

const shownSubjects = computed(() => {
  const list = [...props.s.subjects]
  if (props.subjectId) list.sort((a, b) => (b.subject_id === props.subjectId) - (a.subject_id === props.subjectId))
  return list.slice(0, 3)
})
const price = computed(() => {
  const o = props.subjectId && props.s.subjects.find((x) => x.subject_id === props.subjectId)
  return o ? o.price_kzt : props.s.min_price_kzt
})
</script>

<template>
  <router-link :to="`/specialists/${s.id}`" class="card spec-card">
    <div class="row" style="align-items: flex-start">
      <Avatar :url="s.photo_url" :name="s.display_name" />
      <div class="grow">
        <div class="row between">
          <strong class="name">{{ s.display_name }}</strong>
          <span v-if="price !== null" class="price">{{ t('from') }} {{ formatPrice(price) }}</span>
        </div>
        <div class="row wrap badges">
          <span v-if="s.featured" class="badge accent">⭐ {{ t('featured') }}</span>
          <span v-if="s.verified" class="badge ok">✓ {{ t('verified') }}</span>
          <span v-if="s.trial_kind === 'free'" class="badge warn">{{ t('trialFree') }}</span>
          <Stars v-if="s.rating" :value="s.rating" :count="s.reviews_count" />
        </div>
        <div class="subjects">{{ shownSubjects.map((o) => o.name_ru).join(' · ') }}</div>
        <div class="hint">
          <span v-if="s.district">{{ districtName(s.city, s.district) }} · </span>
          {{ s.formats.map((f) => formats[f]).join(', ') }}
          <span v-if="s.experience_years"> · {{ s.experience_years }} {{ t('years') }}</span>
        </div>
      </div>
    </div>
  </router-link>
</template>

<style scoped>
.spec-card { display: block; color: var(--text); }
.name { font-size: 16px; }
.price { font-weight: 600; white-space: nowrap; }
.badges { gap: 4px; margin: 4px 0; }
.subjects { margin: 2px 0; font-size: 14px; }
</style>
