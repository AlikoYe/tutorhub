<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api/client'
import { store, loadMeta, loadSubjects } from '@/store'
import { t, levels, formats, languages, categories } from '@/i18n'
import SpecialistCard from '@/components/SpecialistCard.vue'
import ChipSelect from '@/components/ChipSelect.vue'
import AsyncState from '@/components/AsyncState.vue'

const route = useRoute()
const router = useRouter()

const FILTER_KEYS = ['q', 'subject_id', 'level', 'language', 'format', 'district', 'price_min', 'price_max', 'verified', 'free_this_week']
const f = reactive(Object.fromEntries(FILTER_KEYS.map((k) => [k, route.query[k] ?? ''])))
const showFilters = ref(false)
const items = ref([])
const total = ref(0)
const loading = ref(false)
const error = ref(null)
const PAGE = 20

const subjectGroups = computed(() => {
  const groups = {}
  for (const s of store.subjects) (groups[s.category] ||= []).push(s)
  return groups
})
const districts = computed(() => store.meta?.cities[0]?.districts || [])
const activeFilters = computed(() => FILTER_KEYS.filter((k) => k !== 'q' && f[k] !== '' && f[k] !== false).length)

function query(offset) {
  const q = { limit: PAGE, offset }
  for (const k of FILTER_KEYS) if (f[k] !== '' && f[k] !== false) q[k] = f[k]
  return q
}

async function search(append = false) {
  loading.value = !append
  error.value = null
  try {
    const page = await api.get('/specialists', query(append ? items.value.length : 0))
    items.value = append ? [...items.value, ...page.items] : page.items
    total.value = page.total
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

let timer
watch(
  () => ({ ...f }),
  () => {
    clearTimeout(timer)
    timer = setTimeout(() => {
      const q = {}
      for (const k of FILTER_KEYS) if (f[k] !== '' && f[k] !== false) q[k] = String(f[k])
      router.replace({ query: q })
      search()
    }, 300)
  },
)

function reset() {
  for (const k of FILTER_KEYS) if (k !== 'q') f[k] = ''
}

onMounted(async () => {
  await Promise.all([loadSubjects().catch(() => {}), loadMeta().catch(() => {})])
  search()
})
</script>

<template>
  <h1>{{ t('appName') }}</h1>
  <div class="row" style="margin-bottom: 10px">
    <input v-model="f.q" type="search" :placeholder="t('searchPlaceholder')" class="grow" />
    <button class="btn secondary" @click="showFilters = !showFilters">
      ⚙️ <span v-if="activeFilters">{{ activeFilters }}</span>
    </button>
  </div>

  <div v-if="showFilters" class="card">
    <label class="field">
      <span>Предмет</span>
      <select v-model="f.subject_id">
        <option value="">{{ t('anySubject') }}</option>
        <optgroup v-for="(list, cat) in subjectGroups" :key="cat" :label="categories[cat]">
          <option v-for="s in list" :key="s.id" :value="String(s.id)">{{ s.name_ru }}</option>
        </optgroup>
      </select>
    </label>
    <div class="field"><span class="hint">Уровень</span><ChipSelect v-model="f.level" :options="levels" /></div>
    <div class="field"><span class="hint">Язык занятий</span><ChipSelect v-model="f.language" :options="languages" /></div>
    <div class="field"><span class="hint">Формат</span><ChipSelect v-model="f.format" :options="formats" /></div>
    <label class="field">
      <span>Район</span>
      <select v-model="f.district">
        <option value="">{{ t('anyDistrict') }}</option>
        <option v-for="d in districts" :key="d.slug" :value="d.slug">{{ d.name_ru }}</option>
      </select>
    </label>
    <div class="row field">
      <input v-model="f.price_min" type="number" min="0" inputmode="numeric" :placeholder="t('priceFrom')" />
      <input v-model="f.price_max" type="number" min="0" inputmode="numeric" :placeholder="t('priceTo')" />
    </div>
    <label class="row field"><input v-model="f.verified" type="checkbox" style="width: auto" true-value="true" false-value="" /> {{ t('verifiedOnly') }}</label>
    <label class="row field"><input v-model="f.free_this_week" type="checkbox" style="width: auto" true-value="true" false-value="" /> Есть свободное время на неделе</label>
    <button class="btn secondary small" @click="reset">{{ t('resetFilters') }}</button>
  </div>

  <AsyncState :loading="loading" :error="error" @retry="search()">
    <p class="hint">{{ t('found') }}: {{ total }}</p>
    <div v-if="!items.length" class="card empty">{{ t('nothingFound') }}</div>
    <SpecialistCard v-for="s in items" :key="s.id" :s="s" :subject-id="f.subject_id ? Number(f.subject_id) : null" />
    <button v-if="items.length < total" class="btn secondary block" @click="search(true)">{{ t('loadMore') }}</button>
  </AsyncState>
</template>
