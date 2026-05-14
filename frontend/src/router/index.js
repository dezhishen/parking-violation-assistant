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
  routes
})

export default router
