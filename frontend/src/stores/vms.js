import { ref } from 'vue'
import { defineStore } from 'pinia'
import api from '../services/api'

export const useVMStore = defineStore('vms', () => {
  const vms = ref([])
  const warnings = ref('')
  const loading = ref(false)
  const error = ref(null)

  async function fetchVMs() {
    loading.value = true
    error.value = null
    warnings.value = ''
    try {
      const response = await api.get('/api/vms')
      if (Array.isArray(response.data)) {
        vms.value = response.data
      } else if (response.data && Array.isArray(response.data.vms)) {
        vms.value = response.data.vms
        warnings.value = response.data.warnings || ''
      } else {
        vms.value = []
      }
    } catch (err) {
      error.value = err.response?.data?.error || 'Failed to fetch VMs'
      vms.value = []
    } finally {
      loading.value = false
    }
  }

  return { vms, warnings, loading, error, fetchVMs }
})
