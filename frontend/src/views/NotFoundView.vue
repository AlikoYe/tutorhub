<script setup>
import { t } from '@/i18n'
</script>

<template>
  <div class="card empty">
    <p>{{ t('notFound') }}</p>
    <router-link to="/" class="btn">{{ t('tabCatalogue') }}</router-link>
  </div>
</template>
