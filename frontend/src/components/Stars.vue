<script setup>
defineProps({ value: { type: Number, default: 0 }, count: { type: Number, default: null } })
</script>

<template>
  <span class="stars" :title="value ? value.toFixed(1) : ''">
    <span v-for="i in 5" :key="i" :class="{ on: i <= Math.round(value) }">★</span>
    <span v-if="value" class="num">{{ value.toFixed(1) }}</span>
    <span v-if="count !== null" class="hint"> ({{ count }})</span>
  </span>
</template>

<style scoped>
.stars { color: color-mix(in srgb, var(--hint) 50%, transparent); white-space: nowrap; font-size: 13px; }
.stars .on { color: #f5a524; }
.num { color: var(--text); margin-left: 4px; font-weight: 600; }
</style>
