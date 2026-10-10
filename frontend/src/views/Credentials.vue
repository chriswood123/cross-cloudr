<template>
  <div>
    <h2 class="mb-4">Cloud Credentials</h2>

    <CAlert v-if="error" color="danger" dismissible @close="error = null">{{ error }}</CAlert>
    <CAlert v-if="success" color="success" dismissible @close="success = null">{{ success }}</CAlert>

    <CCard class="mb-4">
      <CCardHeader>Saved Credentials</CCardHeader>
      <CCardBody>
        <CTable hover responsive v-if="savedCredentials.length > 0">
          <CTableHead>
            <CTableRow>
              <CTableHeaderCell>Provider</CTableHeaderCell>
              <CTableHeaderCell>Details</CTableHeaderCell>
              <CTableHeaderCell>Last Updated</CTableHeaderCell>
              <CTableHeaderCell class="text-end">Actions</CTableHeaderCell>
            </CTableRow>
          </CTableHead>
          <CTableBody>
            <CTableRow v-for="cred in savedCredentials" :key="cred.provider">
              <CTableDataCell>
                <CBadge :color="cred.provider === 'aws' ? 'warning' : 'info'">
                  {{ cred.provider.toUpperCase() }}
                </CBadge>
              </CTableDataCell>
              <CTableDataCell>
                <div v-if="cred.provider === 'aws'">
                  Region: {{ cred.region || '-' }}
                </div>
                <div v-else>
                  Project: {{ cred.project_id || '-' }}
                </div>
              </CTableDataCell>
              <CTableDataCell>{{ formatDate(cred.updated_at) }}</CTableDataCell>
              <CTableDataCell class="text-end">
                <CButton color="primary" size="sm" class="me-2" @click="edit(cred)">Edit</CButton>
                <CButton color="info" size="sm" class="me-2" @click="test(cred.provider)" :disabled="testing === cred.provider">
                  {{ testing === cred.provider ? 'Testing...' : 'Test' }}
                </CButton>
                <CButton color="danger" size="sm" variant="outline" @click="remove(cred.provider)">Remove</CButton>
              </CTableDataCell>
            </CTableRow>
          </CTableBody>
        </CTable>
        <p v-else class="text-medium-emphasis mb-0">No credentials configured yet.</p>
      </CCardBody>
    </CCard>

    <CRow>
      <CCol md="6">
        <CCard class="mb-4">
          <CCardHeader>{{ editingProvider === 'aws' ? 'Edit' : 'Add' }} AWS Credentials</CCardHeader>
          <CCardBody>
            <CForm @submit.prevent="saveAWS">
              <div class="mb-3">
                <CFormLabel>Access Key ID</CFormLabel>
                <CFormInput v-model="aws.access_key_id" :placeholder="awsPlaceholder" :required="editingProvider !== 'aws'" />
              </div>
              <div class="mb-3">
                <CFormLabel>Secret Access Key</CFormLabel>
                <CFormInput v-model="aws.secret_access_key" type="password" placeholder="Leave blank to keep unchanged" :required="editingProvider !== 'aws'" />
              </div>
              <div class="mb-3">
                <CFormLabel>Region</CFormLabel>
                <CFormInput v-model="aws.region" placeholder="e.g. us-east-1" required />
              </div>
              <CButton color="primary" type="submit" :disabled="saving">{{ editingProvider === 'aws' ? 'Update' : 'Save' }} AWS Credentials</CButton>
              <CButton v-if="editingProvider === 'aws'" color="secondary" variant="outline" class="ms-2" @click="cancelEdit">Cancel</CButton>
            </CForm>
          </CCardBody>
        </CCard>
      </CCol>

      <CCol md="6">
        <CCard class="mb-4">
          <CCardHeader>{{ editingProvider === 'gcp' ? 'Edit' : 'Add' }} GCP Credentials</CCardHeader>
          <CCardBody>
            <CForm @submit.prevent="saveGCP">
              <div class="mb-3">
                <CFormLabel>Project ID</CFormLabel>
                <CFormInput v-model="gcp.project_id" required />
              </div>
              <div class="mb-3">
                <CFormLabel>Service Account JSON</CFormLabel>
                <CFormTextarea
                  v-model="gcp.service_account_json"
                  rows="8"
                  :placeholder="gcpPlaceholder"
                  :required="editingProvider !== 'gcp'"
                />
              </div>
              <CButton color="primary" type="submit" :disabled="saving">{{ editingProvider === 'gcp' ? 'Update' : 'Save' }} GCP Credentials</CButton>
              <CButton v-if="editingProvider === 'gcp'" color="secondary" variant="outline" class="ms-2" @click="cancelEdit">Cancel</CButton>
            </CForm>
          </CCardBody>
        </CCard>
      </CCol>
    </CRow>
  </div>
</template>

<script>
import { reactive, ref, onMounted } from 'vue'
import api from '../services/api'

export default {
  name: 'Credentials',
  setup() {
    const aws = reactive({
      access_key_id: '',
      secret_access_key: '',
      region: '',
    })

    const gcp = reactive({
      project_id: '',
      service_account_json: '',
    })

    const savedCredentials = ref([])
    const editingProvider = ref(null)
    const saving = ref(false)
    const testing = ref(null)
    const error = ref(null)
    const success = ref(null)

    const awsPlaceholder = ref('')
    const gcpPlaceholder = ref('')

    async function fetchCredentials() {
      try {
        const response = await api.get('/api/credentials')
        savedCredentials.value = response.data || []
      } catch (err) {
        error.value = err.response?.data?.error || 'Failed to load credentials'
      }
    }

    async function fetchCredentialDetail(provider) {
      try {
        const response = await api.get(`/api/credentials/${provider}`)
        return response.data
      } catch (err) {
        return null
      }
    }

    async function edit(cred) {
      editingProvider.value = cred.provider
      error.value = null
      success.value = null

      const detail = await fetchCredentialDetail(cred.provider)
      if (!detail) {
        error.value = `Failed to load ${cred.provider.toUpperCase()} credential details`
        return
      }

      if (cred.provider === 'aws') {
        aws.access_key_id = ''
        aws.secret_access_key = ''
        aws.region = detail.region || ''
        awsPlaceholder.value = 'Leave blank to keep unchanged'
      } else {
        gcp.project_id = detail.project_id || ''
        gcp.service_account_json = ''
        gcpPlaceholder.value = 'Paste new service account JSON to update, or leave blank to keep unchanged'
      }
    }

    function cancelEdit() {
      editingProvider.value = null
      resetForms()
    }

    function resetForms() {
      aws.access_key_id = ''
      aws.secret_access_key = ''
      aws.region = ''
      gcp.project_id = ''
      gcp.service_account_json = ''
      awsPlaceholder.value = ''
      gcpPlaceholder.value = 'Paste service account JSON here'
    }

    async function saveAWS() {
      saving.value = true
      error.value = null
      success.value = null
      try {
        const payload = { ...aws }
        if (editingProvider.value === 'aws' && !payload.access_key_id) {
          delete payload.access_key_id
        }
        if (editingProvider.value === 'aws' && !payload.secret_access_key) {
          delete payload.secret_access_key
        }
        await api.post('/api/credentials/aws', payload)
        success.value = 'AWS credentials saved successfully'
        editingProvider.value = null
        resetForms()
        await fetchCredentials()
      } catch (err) {
        error.value = err.response?.data?.error || 'Failed to save AWS credentials'
      } finally {
        saving.value = false
      }
    }

    async function saveGCP() {
      saving.value = true
      error.value = null
      success.value = null
      try {
        const payload = { ...gcp }
        if (editingProvider.value === 'gcp' && !payload.service_account_json) {
          delete payload.service_account_json
        }
        await api.post('/api/credentials/gcp', payload)
        success.value = 'GCP credentials saved successfully'
        editingProvider.value = null
        resetForms()
        await fetchCredentials()
      } catch (err) {
        error.value = err.response?.data?.error || 'Failed to save GCP credentials'
      } finally {
        saving.value = false
      }
    }

    async function test(provider) {
      testing.value = provider
      error.value = null
      success.value = null
      try {
        const response = await api.post(`/api/credentials/${provider}/test`)
        success.value = response.data.message
      } catch (err) {
        error.value = err.response?.data?.error || `Failed to test ${provider.toUpperCase()} credentials`
      } finally {
        testing.value = null
      }
    }

    async function remove(provider) {
      saving.value = true
      error.value = null
      success.value = null
      try {
        await api.delete(`/api/credentials/${provider}`)
        success.value = `${provider.toUpperCase()} credentials removed`
        if (editingProvider.value === provider) {
          editingProvider.value = null
          resetForms()
        }
        await fetchCredentials()
      } catch (err) {
        error.value = err.response?.data?.error || 'Failed to remove credentials'
      } finally {
        saving.value = false
      }
    }

    function formatDate(value) {
      if (!value) return '-'
      return new Date(value).toLocaleString()
    }

    onMounted(() => {
      fetchCredentials()
    })

    return {
      aws,
      gcp,
      savedCredentials,
      editingProvider,
      saving,
      testing,
      error,
      success,
      awsPlaceholder,
      gcpPlaceholder,
      saveAWS,
      saveGCP,
      edit,
      cancelEdit,
      test,
      remove,
      formatDate,
    }
  },
}
</script>
