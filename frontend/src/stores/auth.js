import { ref } from 'vue'
import { defineStore } from 'pinia'
import api from '../services/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || null)
  const username = ref(localStorage.getItem('username') || null)
  const error = ref(null)
  const loading = ref(false)

  async function login(credentials) {
    loading.value = true
    error.value = null
    try {
      const response = await api.post('/api/login', credentials)
      token.value = response.data.token
      username.value = credentials.username
      localStorage.setItem('token', response.data.token)
      localStorage.setItem('username', credentials.username)
      return true
    } catch (err) {
      error.value = err.response?.data?.error || 'Login failed'
      return false
    } finally {
      loading.value = false
    }
  }

  function logout() {
    token.value = null
    username.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('username')
  }

  return { token, username, error, loading, login, logout }
})
