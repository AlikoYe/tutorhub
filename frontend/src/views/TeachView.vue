<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import { loadMe } from '@/store'
import { t, statuses, plans, lastDay } from '@/i18n'
import { alertDialog, confirmDialog, haptic } from '@/telegram'
import Avatar from '@/components/Avatar.vue'
import AsyncState from '@/components/AsyncState.vue'

const p = ref(null)
const none = ref(false)
const loading = ref(true)
const error = ref(null)
const busy = ref(false)

async function load() {
  loading.value = true
  error.value = null
  try {
    p.value = await api.get('/my/specialist')
    none.value = false
  } catch (e) {
    if (e.status === 404) none.value = true
    else error.value = e
  } finally {
    loading.value = false
  }
}

async function act(path, confirmText) {
  if (confirmText && !(await confirmDialog(confirmText))) return
  busy.value = true
  try {
    p.value = await api.post(`/my/specialist/${path}`)
    haptic('success')
    loadMe()
  } catch (e) {
    haptic('error')
    alertDialog(e.message)
  } finally {
    busy.value = false
  }
}

function copyLink() {
  navigator.clipboard?.writeText(p.value.personal_link)
  haptic('light')
}

const links = [
  { to: '/teach/lessons', icon: '📋', label: 'Заявки и уроки', hint: 'Подтверждайте заявки учеников' },
  { to: '/teach/schedule', icon: '🗓', label: 'Расписание', hint: 'Рабочие часы и дни отдыха' },
  { to: '/teach/students', icon: '👥', label: 'Ученики', hint: 'Все, кто записывался, и заметки' },
  { to: '/teach/profile', icon: '✏️', label: 'Анкета', hint: 'Предметы, цены, фото' },
]

onMounted(load)
</script>

<template>
  <h1>{{ t('tabTeach') }}</h1>
  <AsyncState :loading="loading" :error="error" @retry="load">
    <div v-if="none" class="card">
      <h2 style="margin-top: 0">Станьте преподавателем TutorHub</h2>
      <p>Ученики Алматы ищут репетиторов по предмету, району и цене. Заполните анкету — после проверки профиль появится в каталоге.</p>
      <p class="hint">Первые преподаватели получают бесплатное размещение (тариф «Основатель»).</p>
      <router-link to="/teach/profile" class="btn block">Заполнить анкету</router-link>
    </div>

    <template v-else-if="p">
      <div class="card">
        <div class="row">
          <Avatar :url="p.photo_url" :name="p.display_name" />
          <div class="grow">
            <strong>{{ p.display_name || 'Без имени' }}</strong>
            <div class="row wrap" style="gap: 4px; margin-top: 4px">
              <span class="badge" :class="{ ok: p.status === 'live', warn: p.status === 'pending' || p.status === 'draft', bad: p.status === 'blocked' }">
                {{ statuses[p.status] }}
              </span>
              <span v-if="p.status === 'live'" class="badge" :class="p.listed ? 'ok' : 'bad'">{{ p.listed ? 'В поиске' : 'Не в поиске' }}</span>
            </div>
          </div>
          <router-link v-if="p.status !== 'draft'" :to="`/specialists/${p.id}`" class="btn secondary small">👁</router-link>
        </div>

        <p v-if="p.rejection_reason && p.status === 'draft'" class="banner error" style="border-radius: 8px; margin: 10px 0 0">
          Анкету вернули на доработку: {{ p.rejection_reason }}
        </p>
        <p v-if="p.status === 'draft'" class="hint">Заполните анкету и отправьте её на проверку.</p>
        <p v-if="p.status === 'pending'" class="hint">Анкета на проверке. Мы напишем в Telegram, когда её одобрят.</p>
        <p v-if="p.status === 'live' && !p.listed" class="hint">Профиль не виден в поиске: нет активной подписки. Ссылка на профиль продолжает работать.</p>

        <div v-if="p.subscription" class="hint" style="margin-top: 8px">
          Тариф: <strong>{{ plans[p.subscription.plan] }}</strong> до {{ lastDay(p.subscription.ends_at) }}
        </div>

        <div class="row wrap" style="margin-top: 10px">
          <button v-if="p.status === 'draft'" class="btn" :disabled="busy" @click="act('submit')">Отправить на проверку</button>
          <button v-if="p.status === 'live'" class="btn secondary small" :disabled="busy" @click="act('hide', 'Скрыть профиль из каталога?')">Скрыть из каталога</button>
          <button v-if="p.status === 'hidden'" class="btn small" :disabled="busy" @click="act('unhide')">Вернуть в каталог</button>
        </div>
      </div>

      <div v-if="p.personal_link && (p.status === 'live' || p.status === 'hidden')" class="card">
        <div class="hint">Личная ссылка для ваших учеников</div>
        <div class="row">
          <code class="grow" style="overflow-wrap: anywhere">{{ p.personal_link }}</code>
          <button class="btn secondary small" @click="copyLink">📋</button>
        </div>
      </div>

      <router-link v-for="l in links" :key="l.to" :to="l.to" class="card row nav-item">
        <span style="font-size: 22px">{{ l.icon }}</span>
        <div class="grow">
          <div>{{ l.label }}</div>
          <div class="hint">{{ l.hint }}</div>
        </div>
        <span class="muted">›</span>
      </router-link>
    </template>
  </AsyncState>
</template>

<style scoped>
.nav-item { color: var(--text); }
</style>
