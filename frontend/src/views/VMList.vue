<template>
  <div>
    <CRow class="mb-3">
      <CCol>
        <h2>Virtual Machines</h2>
      </CCol>
      <CCol class="text-end">
        <CButton color="primary" @click="refresh" :disabled="vmStore.loading">
          Refresh
        </CButton>
      </CCol>
    </CRow>

    <CRow class="mb-3">
      <CCol md="4">
        <CFormInput
          v-model="filter"
          placeholder="Search by name, ID, or state..."
        />
      </CCol>
      <CCol md="3">
        <CFormSelect v-model="providerFilter" :options="providerOptions" />
      </CCol>
    </CRow>

    <CAlert v-if="vmStore.error" color="danger">{{ vmStore.error }}</CAlert>
    <CAlert v-if="vmStore.actionError" color="danger">{{ vmStore.actionError }}</CAlert>
    <CAlert v-if="vmStore.warnings" color="warning">{{ vmStore.warnings }}</CAlert>

    <CTable hover responsive>
      <CTableHead>
        <CTableRow>
          <CTableHeaderCell>Provider</CTableHeaderCell>
          <CTableHeaderCell>Name</CTableHeaderCell>
          <CTableHeaderCell>ID</CTableHeaderCell>
          <CTableHeaderCell>State</CTableHeaderCell>
          <CTableHeaderCell>Type</CTableHeaderCell>
          <CTableHeaderCell>Region/Zone</CTableHeaderCell>
          <CTableHeaderCell>Public IP</CTableHeaderCell>
          <CTableHeaderCell>Private IP</CTableHeaderCell>
          <CTableHeaderCell>Actions</CTableHeaderCell>
        </CTableRow>
      </CTableHead>
      <CTableBody>
        <CTableRow v-for="vm in filteredVMs" :key="`${vm.provider}-${vm.id}`">
          <CTableDataCell>
            <CBadge :color="providerColor(vm.provider)">
              {{ vm.provider.toUpperCase() }}
            </CBadge>
          </CTableDataCell>
          <CTableDataCell>{{ vm.name || '-' }}</CTableDataCell>
          <CTableDataCell>{{ vm.id }}</CTableDataCell>
          <CTableDataCell>
            <CBadge :color="stateColor(vm.state)">{{ vm.state }}</CBadge>
          </CTableDataCell>
          <CTableDataCell>{{ vm.instance_type }}</CTableDataCell>
          <CTableDataCell>{{ vm.zone || vm.region || '-' }}</CTableDataCell>
          <CTableDataCell>{{ vm.public_ip || '-' }}</CTableDataCell>
          <CTableDataCell>{{ vm.private_ip || '-' }}</CTableDataCell>
          <CTableDataCell>
            <CButton
              v-if="canStart(vm)"
              size="sm"
              color="success"
              class="me-1"
              :disabled="vmStore.isActionPending(vm)"
              @click="startVM(vm)"
            >
              {{ vmStore.isActionPending(vm) ? 'Starting...' : 'Start' }}
            </CButton>
            <CButton
              v-if="canStop(vm)"
              size="sm"
              color="danger"
              :disabled="vmStore.isActionPending(vm)"
              @click="stopVM(vm)"
            >
              {{ vmStore.isActionPending(vm) ? 'Stopping...' : 'Stop' }}
            </CButton>
          </CTableDataCell>
        </CTableRow>
        <CTableRow v-if="filteredVMs.length === 0">
          <CTableDataCell colspan="9" class="text-center">
            {{ vmStore.loading ? 'Loading...' : 'No VMs found' }}
          </CTableDataCell>
        </CTableRow>
      </CTableBody>
    </CTable>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue'
import { useVMStore } from '../stores/vms'

export default {
  name: 'VMList',
  setup() {
    const vmStore = useVMStore()
    const filter = ref('')
    const providerFilter = ref('')

    const providerOptions = [
      { label: 'All Providers', value: '' },
      { label: 'AWS', value: 'aws' },
      { label: 'GCP', value: 'gcp' },
      { label: 'DigitalOcean', value: 'digitalocean' },
    ]

    const filteredVMs = computed(() => {
      return vmStore.vms.filter((vm) => {
        const matchesProvider = !providerFilter.value || vm.provider === providerFilter.value
        const term = filter.value.toLowerCase()
        const matchesFilter =
          !term ||
          (vm.name && vm.name.toLowerCase().includes(term)) ||
          vm.id.toLowerCase().includes(term) ||
          vm.state.toLowerCase().includes(term)
        return matchesProvider && matchesFilter
      })
    })

    function stateColor(state) {
      const s = (state || '').toLowerCase()
      if (s === 'running') return 'success'
      if (s === 'stopped' || s === 'terminated') return 'danger'
      return 'secondary'
    }

    function providerColor(provider) {
      if (provider === 'aws') return 'warning'
      if (provider === 'digitalocean') return 'primary'
      return 'info'
    }

    // AWS "stopped" instances can be started. GCP reports a stopped instance
    // as "TERMINATED" (and unlike AWS, it can be started again). DigitalOcean
    // droplet states are normalized to "stopped" by the backend.
    function canStart(vm) {
      const s = (vm.state || '').toLowerCase()
      if (vm.provider === 'aws') return s === 'stopped'
      if (vm.provider === 'gcp') return s === 'terminated'
      return s === 'stopped'
    }

    function canStop(vm) {
      return (vm.state || '').toLowerCase() === 'running'
    }

    function startVM(vm) {
      vmStore.startVM(vm)
    }

    function stopVM(vm) {
      if (window.confirm(`Stop ${vm.name || vm.id}?`)) {
        vmStore.stopVM(vm)
      }
    }

    function refresh() {
      vmStore.fetchVMs()
    }

    onMounted(() => {
      vmStore.fetchVMs()
    })

    return {
      vmStore,
      filter,
      providerFilter,
      providerOptions,
      filteredVMs,
      stateColor,
      providerColor,
      canStart,
      canStop,
      startVM,
      stopVM,
      refresh,
    }
  },
}
</script>
