import { ref } from 'vue'
import { defineStore } from 'pinia'
import api from '../services/api'

export const useVMStore = defineStore('vms', () => {
  const vms = ref([])
  const warnings = ref('')
  const loading = ref(false)
  const error = ref(null)
  const actionError = ref(null)
  const pendingActions = ref({})

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

  function vmKey(vm) {
    return `${vm.provider}-${vm.id}`
  }

  function isActionPending(vm) {
    return !!pendingActions.value[vmKey(vm)]
  }

  async function vmAction(vm, action) {
    const key = vmKey(vm)
    actionError.value = null
    pendingActions.value = { ...pendingActions.value, [key]: true }
    try {
      // AWS addresses instances by instance ID; GCP addresses them by name
      // within a zone.
      const body =
        vm.provider === 'aws' ? { instance_id: vm.id } : { name: vm.name, zone: vm.zone }
      await api.post(`/api/vms/${vm.provider}/${action}`, body)
      await fetchVMs()
      return true
    } catch (err) {
      actionError.value = err.response?.data?.error || `Failed to ${action} VM`
      return false
    } finally {
      pendingActions.value = { ...pendingActions.value, [key]: false }
    }
  }

  function startVM(vm) {
    return vmAction(vm, 'start')
  }

  function stopVM(vm) {
    return vmAction(vm, 'stop')
  }

  return {
    vms,
    warnings,
    loading,
    error,
    actionError,
    pendingActions,
    fetchVMs,
    startVM,
    stopVM,
    isActionPending,
  }
})
