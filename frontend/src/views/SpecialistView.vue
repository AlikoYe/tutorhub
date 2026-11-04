<script setup>
import { useRouter } from 'vue-router'
import { useBackButton } from '@/telegram'
import { t } from '@/i18n'

defineProps({ id: { type: String, required: true } })
const router = useRouter()
useBackButton(() => router.back())
</script>

<template>
  <div class="card empty">{{ t('comingSoon') }}</div>
</template>
