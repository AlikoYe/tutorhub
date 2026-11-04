<script setup>
import { t } from '@/i18n'

defineProps({ loading: Boolean, error: { type: Object, default: null } })
defineEmits(['retry'])
</script>

<template>
  <div v-if="loading" class="empty">{{ t('loading') }}</div>
  <div v-else-if="error" class="card empty">
    <p class="error-text">{{ error.message }}</p>
    <button class="btn secondary small" @click="$emit('retry')">{{ t('retry') }}</button>
  </div>
  <slot v-else />
</template>
