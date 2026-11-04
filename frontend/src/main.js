import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import { initTelegram, tg } from './telegram'
import './styles/main.css'

initTelegram()

// A /start t_<id> deep link may arrive as start_param ("t_<id>"): open that profile.
const start = tg?.initDataUnsafe?.start_param
if (start && /^t_\d+$/.test(start) && location.pathname === '/') {
  router.replace(`/specialists/${start.slice(2)}`)
}

createApp(App).use(router).mount('#app')
