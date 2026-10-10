<template>
  <div class="min-vh-100 d-flex flex-row align-items-center bg-light">
    <CContainer>
      <CRow class="justify-content-center">
        <CCol md="6">
          <CCard class="p-4">
            <CCardBody>
              <h1 class="text-center mb-4">Cross Cloudr</h1>
              <p class="text-medium-emphasis text-center">Sign in to manage your cloud VMs</p>
              <CForm @submit.prevent="handleLogin">
                <CInputGroup class="mb-3">
                  <CInputGroupText>@</CInputGroupText>
                  <CFormInput
                    v-model="form.username"
                    placeholder="Username"
                    autocomplete="username"
                    required
                  />
                </CInputGroup>
                <CInputGroup class="mb-4">
                  <CInputGroupText>*</CInputGroupText>
                  <CFormInput
                    v-model="form.password"
                    type="password"
                    placeholder="Password"
                    autocomplete="current-password"
                    required
                  />
                </CInputGroup>
                <CRow>
                  <CCol xs="12">
                    <CButton color="primary" class="w-100" type="submit" :disabled="authStore.loading">
                      {{ authStore.loading ? 'Signing in...' : 'Sign In' }}
                    </CButton>
                  </CCol>
                </CRow>
                <CAlert v-if="authStore.error" color="danger" class="mt-3 mb-0">
                  {{ authStore.error }}
                </CAlert>
              </CForm>
            </CCardBody>
          </CCard>
        </CCol>
      </CRow>
    </CContainer>
  </div>
</template>

<script>
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

export default {
  name: 'Login',
  setup() {
    const router = useRouter()
    const authStore = useAuthStore()
    const form = reactive({
      username: '',
      password: '',
    })

    async function handleLogin() {
      const success = await authStore.login(form)
      if (success) {
        router.push('/')
      }
    }

    return { form, authStore, handleLogin }
  },
}
</script>
