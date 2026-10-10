import { ref } from 'vue'
import { defineStore } from 'pinia'

const STORAGE_KEY = 'theme'

function storedPreference() {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'light' || stored === 'dark') {
    return stored
  }
  return null
}

function systemPreference() {
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return 'light'
}

function applyTheme(theme) {
  document.documentElement.setAttribute('data-coreui-theme', theme)
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref(storedPreference() || systemPreference())
  applyTheme(theme.value)

  function setTheme(value) {
    if (value !== 'light' && value !== 'dark') {
      return
    }
    theme.value = value
    localStorage.setItem(STORAGE_KEY, value)
    applyTheme(value)
  }

  function toggleTheme() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  return { theme, setTheme, toggleTheme }
})
