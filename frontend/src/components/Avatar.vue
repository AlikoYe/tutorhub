<script setup>
import { computed } from 'vue'

const props = defineProps({ url: { type: String, default: null }, name: { type: String, default: '' }, big: Boolean })
const initials = computed(() =>
  props.name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join(''),
)
</script>

<template>
  <img v-if="url" :src="url" :alt="name" class="avatar" :class="{ big }" loading="lazy" />
  <div v-else class="avatar" :class="{ big }">{{ initials || '👤' }}</div>
</template>
