<template>
  <div>
    <h2 class="mb-4">Cloud Credentials</h2>

    <CAlert v-if="error" color="danger">{{ error }}</CAlert>
    <CAlert v-if="success" color="success">{{ success }}</CAlert>

    <CRow>
      <CCol md="6">
        <CCard class="mb-4">
          <CCardHeader>AWS Credentials</CCardHeader>
          <CCardBody>
            <CForm @submit.prevent="saveAWS">
              <div class="mb-3">
                <CFormLabel>Access Key ID</CFormLabel>
                <CFormInput v-model="aws.access_key_id" required />
              </div>
              <div class="mb-3">
                <CFormLabel>Secret Access Key</CFormLabel>
                <CFormInput v-model="aws.secret_access_key" type="password" required />
              </div>
              <div class="mb-3">
                <CFormLabel>Region</CFormLabel>
                <CFormInput v-model="aws.region" placeholder="e.g. us-east-1" required />
              </div>
              <CButton color="primary" type="submit" :disabled="saving">Save AWS Credentials</CButton>
              <CButton
                v-if="hasAWS"
                color="danger"
                variant="outline"
                class="ms-2"
                @click="remove('aws')"
                :disabled="saving"
              >
                Remove
              </CButton>
            </CForm>
          </CCardBody>
        </CCard>
      </CCol>

      <CCol md="6">
        <CCard class="mb-4">
          <CCardHeader>GCP Credentials</CCardHeader>
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
                  placeholder="Paste service account JSON here"
                  required
                />
              </div>
              <CButton color="primary" type="submit" :disabled="saving">Save GCP Credentials</CButton>
              <CButton
                v-if="hasGCP"
                color="danger"
                variant="outline"
                class="ms-2"
                @click="remove('gcp')"
                :disabled="saving"
              >
                Remove
              </CButton>
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

    const hasAWS = ref(false)
    const hasGCP = ref(false)
    const saving = ref(false)
    const error = ref(null)
    const success = ref(null)

    async function fetchCredentials() {
      try {
        const response = await api.get('/api/credentials')
        hasAWS.value = response.data.some((c) => c.provider === 'aws')
        hasGCP.value = response.data.some((c) => c.provider === 'gcp')
      } catch (err) {
        error.value = err.response?.data?.error || 'Failed to load credentials'
      }
    }

    async function saveAWS() {
      saving.value = true
      error.value = null
      success.value = null
      try {
        await api.post('/api/credentials/aws', aws)
        success.value = 'AWS credentials saved successfully'
        hasAWS.value = true
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
        await api.post('/api/credentials/gcp', gcp)
        success.value = 'GCP credentials saved successfully'
        hasGCP.value = true
      } catch (err) {
        error.value = err.response?.data?.error || 'Failed to save GCP credentials'
      } finally {
        saving.value = false
      }
    }

    async function remove(provider) {
      saving.value = true
      error.value = null
      success.value = null
      try {
        await api.delete(`/api/credentials/${provider}`)
        success.value = `${provider.toUpperCase()} credentials removed`
        if (provider === 'aws') {
          hasAWS.value = false
          aws.access_key_id = ''
          aws.secret_access_key = ''
          aws.region = ''
        } else {
          hasGCP.value = false
          gcp.project_id = ''
          gcp.service_account_json = ''
        }
      } catch (err) {
        error.value = err.response?.data?.error || 'Failed to remove credentials'
      } finally {
        saving.value = false
      }
    }

    onMounted(() => {
      fetchCredentials()
    })

    return {
      aws,
      gcp,
      hasAWS,
      hasGCP,
      saving,
      error,
      success,
      saveAWS,
      saveGCP,
      remove,
    }
  },
}
</script>
