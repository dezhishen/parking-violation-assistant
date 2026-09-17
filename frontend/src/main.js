import { createApp } from 'vue'
import { vLoading } from 'element-plus'
import 'element-plus/dist/index.css'

import App from './App.vue'
import router from './router/index.js'

const app = createApp(App)

// Element Plus 组件由 unplugin-vue-components 按需引入；
// 图标在各组件内单独 import，避免把整包图标打进产物。
// v-loading 指令需要显式注册。
app.directive('loading', vLoading)

app.use(router)

app.mount('#app')
