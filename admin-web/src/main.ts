import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createNaiveUI } from './plugins/naive'
import { router } from './router'
import App from './App.vue'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(createNaiveUI())

app.mount('#app')

