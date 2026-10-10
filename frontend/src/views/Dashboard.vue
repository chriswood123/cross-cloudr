<template>
  <div>
    <CHeader color="primary" class="text-white">
      <CContainer fluid>
        <CHeaderBrand href="#">Cross Cloudr</CHeaderBrand>
        <CHeaderNav class="ms-auto">
          <CNavItem>
            <span class="nav-link">{{ authStore.username }}</span>
          </CNavItem>
          <CNavItem>
            <CButton color="light" size="sm" @click="logout">Logout</CButton>
          </CNavItem>
        </CHeaderNav>
      </CContainer>
    </CHeader>
    <CContainer fluid class="mt-4">
      <CRow>
        <CCol md="2">
          <CListGroup>
            <CListGroupItem
              :active="$route.path === '/vms'"
              @click="$router.push('/vms')"
              class="cursor-pointer"
            >
              VMs
            </CListGroupItem>
            <CListGroupItem
              :active="$route.path === '/credentials'"
              @click="$router.push('/credentials')"
              class="cursor-pointer"
            >
              Credentials
            </CListGroupItem>
          </CListGroup>
        </CCol>
        <CCol md="10">
          <router-view />
        </CCol>
      </CRow>
    </CContainer>
  </div>
</template>

<script>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

export default {
  name: 'Dashboard',
  setup() {
    const router = useRouter()
    const authStore = useAuthStore()

    function logout() {
      authStore.logout()
      router.push('/login')
    }

    return { authStore, logout }
  },
}
</script>

<style scoped>
.cursor-pointer {
  cursor: pointer;
}
</style>
