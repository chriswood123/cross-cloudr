import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useThemeStore } from './theme'

describe('theme store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.removeAttribute('data-coreui-theme')
    // jsdom does not implement matchMedia; default to a light system
    // preference so tests can override it per case.
    window.matchMedia = vi.fn().mockReturnValue({ matches: false })
  })

  it('initializes from the stored preference', () => {
    localStorage.setItem('theme', 'dark')

    const store = useThemeStore()

    expect(store.theme).toBe('dark')
    expect(document.documentElement.getAttribute('data-coreui-theme')).toBe('dark')
  })

  it('falls back to a dark system preference when nothing is stored', () => {
    window.matchMedia = vi.fn().mockReturnValue({ matches: true })

    const store = useThemeStore()

    expect(store.theme).toBe('dark')
    expect(window.matchMedia).toHaveBeenCalledWith('(prefers-color-scheme: dark)')
    // The system preference must not be persisted as a user choice.
    expect(localStorage.getItem('theme')).toBeNull()
  })

  it('falls back to light when the system prefers light', () => {
    const store = useThemeStore()

    expect(store.theme).toBe('light')
    expect(document.documentElement.getAttribute('data-coreui-theme')).toBe('light')
  })

  it('ignores an invalid stored value', () => {
    localStorage.setItem('theme', 'blue')

    const store = useThemeStore()

    expect(store.theme).toBe('light')
  })

  it('toggles the theme, persists it, and updates the attribute', () => {
    const store = useThemeStore()

    store.toggleTheme()

    expect(store.theme).toBe('dark')
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(document.documentElement.getAttribute('data-coreui-theme')).toBe('dark')

    store.toggleTheme()

    expect(store.theme).toBe('light')
    expect(localStorage.getItem('theme')).toBe('light')
    expect(document.documentElement.getAttribute('data-coreui-theme')).toBe('light')
  })

  it('persists an explicit theme set programmatically', () => {
    const store = useThemeStore()

    store.setTheme('dark')

    expect(store.theme).toBe('dark')
    expect(localStorage.getItem('theme')).toBe('dark')
  })

  it('ignores invalid values when set programmatically', () => {
    const store = useThemeStore()

    store.setTheme('blue')

    expect(store.theme).toBe('light')
    expect(localStorage.getItem('theme')).toBeNull()
    expect(document.documentElement.getAttribute('data-coreui-theme')).toBe('light')
  })
})
