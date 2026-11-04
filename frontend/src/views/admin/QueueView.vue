<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { loadMeta, districtName } from '@/store'
import { statuses, levels, formats, formatPrice, plans, lastDay } from '@/i18n'
import { useBackButton, alertDialog, haptic } from '@/telegram'
import Avatar from '@/components/Avatar.vue'
import AsyncState from '@/components/AsyncState.vue'
import ChipSelect from '@/components/ChipSelect.vue'
import AdminSubscriptionPanel from '@/components/AdminSubscriptionPanel.vue'

const router = useRouter()
useBackButton(() => router.push('/admin'))

const status = ref('pending')
const items = ref([])
const loading = ref(true)
const error = ref(null)
const open = ref(null)
const decision = reactive({ plan: 'trial', ends_on: '', reason: '' })

async function load() {
  loading.value = true
  error.value = null
  try {
    await loadMeta().catch(() => {})
    items.value = await api.get('/admin/specialists', { status: status.value })
  } catch (e) {
    error.value = e
  } finally {
    loading.value = false
  }
}

async function approve(p) {
  try {
    const body = { plan: decision.plan }
    if (decision.ends_on) body.ends_on = decision.ends_on
    await api.post(`/admin/specialists/${p.id}/approve`, body)
    haptic('success')
    open.value = null
    load()
  } catch (e) {
    alertDialog(e.message)
  }
}

async function reject(p) {
  try {
    await api.post(`/admin/specialists/${p.id}/reject`, { reason: decision.reason })
    haptic('success')
    open.value = null
    decision.reason = ''
    load()
  } catch (e) {
    alertDialog(e.message)
  }
}

onMounted(load)
</script>

<template>
  <h1>Анкеты</h1>
  <ChipSelect v-model="status" :options="statuses" @update:model-value="(v) => { status = v || 'pending'; load() }" />
  <div style="height: 10px" />
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="!items.length" class="card empty">Пусто</div>
    <div v-for="p in items" :key="p.id" class="card">
      <div class="row" @click="open = open === p.id ? null : p.id">
        <Avatar :url="p.photo_url" :name="p.display_name" />
        <div class="grow">
          <strong>{{ p.display_name }}</strong>
          <div class="hint">
            {{ p.owner.name }}<span v-if="p.owner.username"> · @{{ p.owner.username }}</span>
          </div>
          <div class="hint">{{ p.subjects.map((o) => o.name_ru).join(', ') }}</div>
        </div>
        <span class="muted">{{ open === p.id ? '▴' : '▾' }}</span>
      </div>

      <div v-if="open === p.id" class="stack" style="margin-top: 10px">
        <p class="pre">{{ p.bio }}</p>
        <p v-if="p.education" class="hint pre">🎓 {{ p.education }}</p>
        <div v-for="o in p.subjects" :key="o.subject_id" class="hint">
          {{ o.name_ru }}: {{ o.levels.map((l) => levels[l]).join(', ') }} — {{ formatPrice(o.price_kzt) }}
        </div>
        <div class="hint">
          {{ p.formats.map((f) => formats[f]).join(', ') }} · {{ districtName(p.city, p.district) }} {{ p.address }}
        </div>
        <div v-if="p.subscription" class="hint">Тариф: {{ plans[p.subscription.plan] }} до {{ lastDay(p.subscription.ends_at) }}</div>

        <template v-if="p.status === 'pending'">
          <div class="row">
            <select v-model="decision.plan" style="max-width: 50%">
              <option value="trial">Пробный период</option>
              <option value="founding">Основатель</option>
            </select>
            <input v-model="decision.ends_on" type="date" :required="decision.plan === 'founding'" />
          </div>
          <button class="btn block" @click="approve(p)">✅ Одобрить</button>
          <textarea v-model="decision.reason" rows="2" placeholder="Причина отказа (увидит преподаватель)" />
          <button class="btn danger block" :disabled="!decision.reason.trim()" @click="reject(p)">❌ Вернуть на доработку</button>
        </template>
        <AdminSubscriptionPanel v-else :specialist-id="p.id" />
      </div>
    </div>
  </AsyncState>
</template>

<style scoped>
.pre { white-space: pre-wrap; margin: 0; }
</style>
