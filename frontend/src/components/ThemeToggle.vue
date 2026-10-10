<template>
  <CButton
    color="light"
    size="sm"
    :title="label"
    :aria-label="label"
    @click="themeStore.toggleTheme()"
  >
    <svg
      v-if="themeStore.theme === 'dark'"
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      fill="currentColor"
      viewBox="0 0 16 16"
      aria-hidden="true"
    >
      <circle cx="8" cy="8" r="4" />
      <g stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
        <line x1="8" y1="0.5" x2="8" y2="2.5" />
        <line x1="8" y1="13.5" x2="8" y2="15.5" />
        <line x1="0.5" y1="8" x2="2.5" y2="8" />
        <line x1="13.5" y1="8" x2="15.5" y2="8" />
        <line x1="2.7" y1="2.7" x2="4.1" y2="4.1" />
        <line x1="11.9" y1="11.9" x2="13.3" y2="13.3" />
        <line x1="2.7" y1="13.3" x2="4.1" y2="11.9" />
        <line x1="11.9" y1="4.1" x2="13.3" y2="2.7" />
      </g>
    </svg>
    <svg
      v-else
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      fill="currentColor"
      viewBox="0 0 16 16"
      aria-hidden="true"
    >
      <path
        d="M6 .278a.768.768 0 0 1 .08.858 7.208 7.208 0 0 0-.878 3.46c0 4.021 3.278 7.277 7.318 7.277.527 0 1.04-.055 1.533-.16a.787.787 0 0 1 .81.316.733.733 0 0 1-.031.893A8.349 8.349 0 0 1 8.344 16C3.734 16 0 12.286 0 7.71 0 4.266 2.114 1.312 5.124.06A.752.752 0 0 1 6 .278z"
      />
    </svg>
  </CButton>
</template>

<script>
import { computed } from 'vue'
import { useThemeStore } from '../stores/theme'

export default {
  name: 'ThemeToggle',
  setup() {
    const themeStore = useThemeStore()

    const label = computed(() =>
      themeStore.theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'
    )

    return { themeStore, label }
  },
}
</script>
