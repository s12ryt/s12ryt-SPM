<script setup lang="ts">
import type { Rules } from "./types";
defineProps<{ rules: Rules }>();
const fields: [keyof Rules, string, number, number][] = [
  ["cpu", "CPU 門檻（%）", 1, 100],
  ["memory", "記憶體門檻（%）", 1, 100],
  ["disk", "磁碟門檻（%）", 1, 100],
  ["holdSeconds", "持續秒數", 0, 86400],
  ["offlineSeconds", "離線秒數", 5, 86400],
  ["intervalSeconds", "採樣間隔（秒）", 1, 3600],
];
</script>
<template>
  <div class="form-grid">
    <label v-for="[key, label, min, max] in fields" :key="key"
      >{{ label
      }}<input
        v-model.number="rules[key]"
        type="number"
        :min="min"
        :max="max"
        required
    /></label>
  </div>
</template>
