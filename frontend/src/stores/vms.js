import { ref } from 'vue'
import { defineStore } from 'pinia'
import api from '../services/api'

export const useVMStore = defineStore('vms', () => {
  const vms = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function fetchVMs() {
    loading.value = true
    error.value = null
    try {
      const response = await api.get('/api/vms')
      vms.value = response.data
    } catch (err) {
      error.value = err.response?.data?.error || 'Failed to fetch VMs'
    } finally {
      loading.value = false
    }
  }

  return { vms, loading, error, fetchVMs }
})
