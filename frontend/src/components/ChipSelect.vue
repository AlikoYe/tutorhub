<script setup>
// Chips for choosing values from options ({value: label}); multiple -> array model.
const props = defineProps({
  options: { type: Object, required: true },
  modelValue: { type: [Array, String], default: null },
  multiple: Boolean,
})
const emit = defineEmits(['update:modelValue'])

function isOn(v) {
  return props.multiple ? (props.modelValue || []).includes(v) : props.modelValue === v
}
function toggle(v) {
  if (props.multiple) {
    const cur = props.modelValue || []
    emit('update:modelValue', cur.includes(v) ? cur.filter((x) => x !== v) : [...cur, v])
  } else {
    emit('update:modelValue', props.modelValue === v ? '' : v)
  }
}
</script>

<template>
  <div class="row wrap chips">
    <button v-for="(label, v) in options" :key="v" type="button" class="chip" :class="{ on: isOn(v) }" @click="toggle(v)">
      {{ label }}
    </button>
  </div>
</template>

<style scoped>
.chips { gap: 6px; }
</style>
