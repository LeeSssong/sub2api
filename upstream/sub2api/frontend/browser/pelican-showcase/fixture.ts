import { createApp } from 'vue'
import PelicanShowcaseView from '../../src/views/user/PelicanShowcaseView.vue'
import { i18n, loadLocaleMessages } from '../../src/i18n'
import '../../src/style.css'

void loadLocaleMessages('zh').then(() => {
  i18n.global.locale.value = 'zh'
  if (new URLSearchParams(location.search).get('theme') === 'light') document.documentElement.classList.remove('dark')
  createApp(PelicanShowcaseView).use(i18n).mount('#app')
})
