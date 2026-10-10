import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
import Credentials from '../views/Credentials.vue'
import VMList from '../views/VMList.vue'

const routes = [
  { path: '/login', name: 'Login', component: Login, meta: { public: true } },
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard,
    children: [
      { path: '', redirect: '/vms' },
      { path: 'vms', name: 'VMs', component: VMList },
      { path: 'credentials', name: 'Credentials', component: Credentials },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  if (!to.meta.public && !authStore.token) {
    next('/login')
  } else {
    next()
  }
})

export default router
