<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { useBackButton } from '@/telegram'
import { loadMeta, districtName, cityName } from '@/store'
import { t, levels, formats, languages, kinds, formatPrice } from '@/i18n'
import Avatar from '@/components/Avatar.vue'
import AsyncState from '@/components/AsyncState.vue'

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
        <p v-if="!p.listed" class="hint">{{ t('notListed') }}</p>
      </div>

      <div class="card">
        <h3 style="margin-top: 0">{{ t('subjectsAndPrices') }}</h3>
        <div v-for="o in p.subjects" :key="o.subject_id" class="row between offer">
          <div>
            <div>{{ o.name_ru }}</div>
            <div class="hint">{{ o.levels.map((l) => levels[l]).join(', ') }}</div>
          </div>
          <strong>{{ formatPrice(o.price_kzt) }}</strong>
        </div>
        <p class="hint" style="margin-bottom: 0">
          {{ t('lessonLength') }}: {{ p.lesson_minutes }} {{ t('min') }}
          <template v-if="p.trial_kind === 'free'"> · {{ t('trialFree') }}</template>
          <template v-else-if="p.trial_kind === 'paid'"> · {{ t('trialPaid') }}: {{ formatPrice(p.trial_price) }}</template>
        </p>
      </div>

      <div class="card">
        <div class="hint">Формат</div>
        <div>{{ p.formats.map((f) => formats[f]).join(', ') }}</div>
        <div class="hint" style="margin-top: 8px">Язык занятий</div>
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
    </template>
  </AsyncState>
</template>

<style scoped>
.offer { padding: 6px 0; border-bottom: 1px solid color-mix(in srgb, var(--hint) 15%, transparent); }
.offer:last-of-type { border-bottom: 0; }
.pre { white-space: pre-wrap; margin: 0; }
</style>
