<script setup lang="ts">
import type { InputHTMLAttributes } from "vue"
import { cn } from "@/lib/utils"

const model = defineModel<string | number>()

const props = defineProps<{
  class?: InputHTMLAttributes["class"]
}>()
</script>

<template>
  <input
    v-model="model"
    data-slot="input"
    :class="
      cn(
        'border-input bg-background placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs outline-none transition-[color,box-shadow] focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50',
        props.class,
      )
    "
  >
</template>
