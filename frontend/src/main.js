import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
import { loadSavedTheme } from './theme'
import { loadSavedLocale, i18nPlugin } from './i18n'

loadSavedTheme()
loadSavedLocale()

createApp(App).use(router).use(i18nPlugin).mount('#app')
