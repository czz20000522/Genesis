import { createApp } from 'vue'
import { ElAlert, ElButton, ElCollapse, ElCollapseItem, ElEmpty, ElIcon, ElInput, ElOption, ElSelect, ElTooltip } from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import './styles.css'

const app = createApp(App)

for (const component of [ElAlert, ElButton, ElCollapse, ElCollapseItem, ElEmpty, ElIcon, ElInput, ElOption, ElSelect, ElTooltip]) {
  if (component.name) app.component(component.name, component)
}

app.mount('#app')
