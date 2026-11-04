<script setup>
import { t } from '@/i18n'

const sections = [
  { to: '/admin/queue', icon: '🗂', label: 'Проверка анкет', hint: 'Новые профили преподавателей' },
]
</script>

<template>
  <h1>{{ t('tabAdmin') }}</h1>
  <router-link v-for="s in sections" :key="s.to" :to="s.to" class="card row" style="color: var(--text)">
    <span style="font-size: 22px">{{ s.icon }}</span>
    <div class="grow">
      <div>{{ s.label }}</div>
      <div class="hint">{{ s.hint }}</div>
    </div>
    <span class="muted">›</span>
  </router-link>
</template>
