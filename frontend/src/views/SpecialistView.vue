<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { useBackButton, openTelegramLink, haptic, alertDialog, shareLink } from '@/telegram'
import { loadMeta, districtName, cityName } from '@/store'
import { t, levels, formats, languages, kinds, formatPrice, subjectName, reportReasons } from '@/i18n'
import Avatar from '@/components/Avatar.vue'
import AsyncState from '@/components/AsyncState.vue'
import BookingPanel from '@/components/BookingPanel.vue'
import ReviewsSection from '@/components/ReviewsSection.vue'
import Stars from '@/components/Stars.vue'
import { confirmDialog } from '@/telegram'
import { store } from '@/store'

const props = defineProps({ id: { type: String, required: true } })
const router = useRouter()
useBackButton(() => (window.history.length > 1 ? router.back() : router.push('/')))

const p = ref(null)
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    await loadMeta().catch(() => {})
    p.value = await api.get(`/specialists/${props.id}`)
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

// The public page lives on the same domain as the Mini App.
const origin = window.location.origin

async function toggleFavourite() {
  const on = !p.value.favourite
  try {
    if (on) await api.put(`/favourites/${p.value.id}`)
    else await api.del(`/favourites/${p.value.id}`)
    p.value.favourite = on
    haptic('light')
  } catch (e) {
    alertDialog(e.message)
  }
}

const reporting = ref(false)
const reportReason = ref('fake')
const reportText = ref('')

async function report() {
  if (!(await confirmDialog(t('reportConfirm')))) return
  try {
    await api.post(`/specialists/${p.value.id}/report`, { reason: reportReason.value, text: reportText.value })
    reporting.value = false
    alertDialog(t('reportThanks'))
  } catch (e) {
    alertDialog(e.message)
  }
}

async function contact() {
  try {
    const { telegram_url } = await api.post(`/specialists/${p.value.id}/contact`)
    openTelegramLink(telegram_url)
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(load)
watch(() => props.id, load)
</script>

<template>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <template v-if="p">
      <div class="card" style="text-align: center">
        <Avatar :url="p.photo_url" :name="p.display_name" big style="margin: 0 auto 8px" />
        <h1 style="margin-bottom: 4px">{{ p.display_name }}</h1>
        <div class="hint">{{ kinds[p.kind] }} · {{ cityName(p.city) }}<span v-if="p.district">, {{ districtName(p.city, p.district) }}</span></div>
        <div class="row wrap" style="justify-content: center; gap: 4px; margin-top: 6px">
          <span v-if="p.featured" class="badge accent">⭐ {{ t('featured') }}</span>
          <span v-if="p.verified" class="badge ok">✓ {{ t('verified') }}</span>
          <span v-if="p.experience_years" class="badge">{{ p.experience_years }} {{ t('years') }}</span>
        </div>
        <div v-if="p.rating" style="margin-top: 6px"><Stars :value="p.rating" :count="p.reviews_count" /></div>
        <p v-if="!p.listed" class="hint">{{ t('notListed') }}</p>
        <div v-if="store.me && p.id !== store.me.specialist?.id" class="row wrap" style="justify-content: center; margin-top: 10px">
          <button class="btn secondary small" @click="contact">💬 {{ t('contact') }}</button>
          <button class="btn secondary small" @click="shareLink(`${origin}/t/${p.id}`, p.display_name)">↗ {{ t('share') }}</button>
          <button class="btn secondary small" @click="toggleFavourite">{{ p.favourite ? '★ ' + t('favouriteOn') : '☆ ' + t('favouriteAdd') }}</button>
        </div>
      </div>

      <div class="card">
        <h3 style="margin-top: 0">{{ t('subjectsAndPrices') }}</h3>
        <div v-for="o in p.subjects" :key="o.subject_id" class="row between offer">
          <div>
            <div>{{ subjectName(o) }}</div>
            <div class="hint">{{ o.levels.map((l) => levels[l]).join(', ') }}</div>
          </div>
          <strong>{{ formatPrice(o.price_kzt) }}</strong>
        </div>
        <p class="hint" style="margin-bottom: 0">
          {{ t('lessonLength') }}: {{ p.lesson_minutes }} {{ t('min') }} · {{ t('cancelRule', { h: p.cancel_hours }) }}
          <template v-if="p.trial_kind === 'free'"> · {{ t('trialFree') }}</template>
          <template v-else-if="p.trial_kind === 'paid'"> · {{ t('trialPaid') }}: {{ formatPrice(p.trial_price) }}</template>
        </p>
      </div>

      <BookingPanel v-if="p.subjects.length && store.me && p.id !== store.me.specialist?.id" :specialist="p" />

      <div class="card">
        <div class="hint">{{ t('format') }}</div>
        <div>{{ p.formats.map((f) => formats[f]).join(', ') }}</div>
        <div class="hint" style="margin-top: 8px">{{ t('lessonLanguage') }}</div>
        <div>{{ p.languages.map((l) => languages[l]).join(', ') }}</div>
        <template v-if="p.address">
          <div class="hint" style="margin-top: 8px">{{ t('address') }}</div>
          <div>{{ p.address }}</div>
        </template>
      </div>

      <div v-if="p.bio" class="card">
        <h3 style="margin-top: 0">{{ t('aboutMe') }}</h3>
        <p class="pre">{{ p.bio }}</p>
      </div>
      <div v-if="p.education" class="card">
        <h3 style="margin-top: 0">{{ t('education') }}</h3>
        <p class="pre">{{ p.education }}</p>
      </div>

      <ReviewsSection :specialist-id="p.id" :can-act="Boolean(store.me && p.id !== store.me.specialist?.id)" @changed="load" />

      <div v-if="store.me && p.id !== store.me.specialist?.id" style="text-align: center">
        <button v-if="!reporting" class="btn secondary small" @click="reporting = true">🚩 {{ t('report') }}</button>
        <div v-else class="card stack" style="text-align: left">
          <select v-model="reportReason">
            <option v-for="(label, r) in reportReasons" :key="r" :value="r">{{ label }}</option>
          </select>
          <textarea v-model="reportText" maxlength="1000" rows="2" :placeholder="t('reportDetails')" />
          <div class="row">
            <button class="btn danger small" @click="report">{{ t('reportSend') }}</button>
            <button class="btn secondary small" @click="reporting = false">{{ t('cancel') }}</button>
          </div>
        </div>
      </div>
    </template>
  </AsyncState>
</template>

<style scoped>
.offer { padding: 6px 0; border-bottom: 1px solid color-mix(in srgb, var(--hint) 15%, transparent); }
.offer:last-of-type { border-bottom: 0; }
.pre { white-space: pre-wrap; margin: 0; }
</style>
