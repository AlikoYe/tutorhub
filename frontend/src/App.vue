<script setup>
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { store, loadMe } from '@/store'
import { t } from '@/i18n'

const route = useRoute()

const tabs = computed(() => {
  const list = [
    { name: 'catalogue', to: '/', label: t('tabCatalogue'), icon: '🔎' },
    { name: 'lessons', to: '/lessons', label: t('tabLessons'), icon: '📅' },
    { name: 'favourites', to: '/favourites', label: t('tabFavourites'), icon: '⭐' },
    { name: 'teach', to: '/teach', label: t('tabTeach'), icon: '🎓' },
  ]
  if (store.me?.is_admin) list.push({ name: 'admin', to: '/admin', label: t('tabAdmin'), icon: '🛠' })
  return list
})

onMounted(loadMe)
</script>

<template>
  <div class="app">
    <div v-if="store.meError" class="banner error">
      {{ t('authFailed') }} <small>({{ store.meError.message }})</small>
    </div>
    <main class="content">
      <router-view />
    </main>
    <nav v-if="route.meta.tab" class="tabbar">
      <router-link
        v-for="tab in tabs"
        :key="tab.name"
        :to="tab.to"
        class="tab"
        :class="{ active: route.meta.tab === tab.name }"
      >
        <span class="tab-icon">{{ tab.icon }}</span>
        <span class="tab-label">{{ tab.label }}</span>
      </router-link>
    </nav>
  </div>
</template>
