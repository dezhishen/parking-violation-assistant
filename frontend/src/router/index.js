import { createRouter, createWebHashHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import PlateDetailView from '../views/PlateDetailView.vue'
import RecordListView from '../views/RecordListView.vue'

const routes = [
  { path: '/', name: 'home', component: HomeView },
  { path: '/plates/:plate', name: 'plate-detail', component: PlateDetailView },
  { path: '/records', name: 'records', component: RecordListView },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  // 前进/后退恢复原位置，新页面回到顶部
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    return { top: 0 }
  }
})

export default router
