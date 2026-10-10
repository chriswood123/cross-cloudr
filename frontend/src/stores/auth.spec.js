import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('../services/api', () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
  },
}))

import api from '../services/api'
import { useAuthStore } from './auth'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('logs in successfully and persists the token', async () => {
    api.post.mockResolvedValueOnce({ data: { token: 'test-token' } })

    const store = useAuthStore()
    const result = await store.login({ username: 'admin', password: 'secret' })

    expect(result).toBe(true)
    expect(store.token).toBe('test-token')
    expect(store.username).toBe('admin')
    expect(store.error).toBeNull()
    expect(store.loading).toBe(false)
    expect(localStorage.getItem('token')).toBe('test-token')
    expect(localStorage.getItem('username')).toBe('admin')
    expect(api.post).toHaveBeenCalledWith('/api/login', {
      username: 'admin',
      password: 'secret',
    })
  })

  it('sets an error on failed login and does not persist a token', async () => {
    api.post.mockRejectedValueOnce({
      response: { data: { error: 'invalid credentials' } },
    })

    const store = useAuthStore()
    const result = await store.login({ username: 'admin', password: 'wrong' })

    expect(result).toBe(false)
    expect(store.token).toBeNull()
    expect(store.error).toBe('invalid credentials')
    expect(store.loading).toBe(false)
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('uses a default error message when the API provides none', async () => {
    api.post.mockRejectedValueOnce({})

    const store = useAuthStore()
    await store.login({ username: 'admin', password: 'wrong' })

    expect(store.error).toBe('Login failed')
  })

  it('restores the session from localStorage on init', () => {
    localStorage.setItem('token', 'persisted-token')
    localStorage.setItem('username', 'admin')

    const store = useAuthStore()

    expect(store.token).toBe('persisted-token')
    expect(store.username).toBe('admin')
  })

  it('starts with no session when localStorage is empty', () => {
    const store = useAuthStore()

    expect(store.token).toBeNull()
    expect(store.username).toBeNull()
  })

  it('logs out and clears stored state', async () => {
    api.post.mockResolvedValueOnce({ data: { token: 'test-token' } })

    const store = useAuthStore()
    await store.login({ username: 'admin', password: 'secret' })
    expect(store.token).toBe('test-token')

    store.logout()

    expect(store.token).toBeNull()
    expect(store.username).toBeNull()
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('username')).toBeNull()
  })

  it('toggles loading during the login request', async () => {
    let resolveLogin
    api.post.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveLogin = resolve
      })
    )

    const store = useAuthStore()
    const loginPromise = store.login({ username: 'admin', password: 'secret' })

    expect(store.loading).toBe(true)

    resolveLogin({ data: { token: 'test-token' } })
    await loginPromise

    expect(store.loading).toBe(false)
  })
})
