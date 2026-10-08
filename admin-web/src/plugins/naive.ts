import type { App } from 'vue'
import {
  create,
  NButton,
  NCard,
  NConfigProvider,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NLayout,
  NLayoutContent,
  NLayoutFooter,
  NLayoutHeader,
  NLayoutSider,
  NMenu,
  NMessageProvider,
  NPageHeader,
  NSpace,
  NTag,
} from 'naive-ui'

export function createNaiveUI() {
  const naive = create({
    components: [
      NButton,
      NCard,
      NConfigProvider,
      NDataTable,
      NForm,
      NFormItem,
      NInput,
      NLayout,
      NLayoutContent,
      NLayoutFooter,
      NLayoutHeader,
      NLayoutSider,
      NMenu,
      NMessageProvider,
      NPageHeader,
      NSpace,
      NTag,
    ],
  })

  return {
    install(app: App) {
      app.use(naive)
    },
  }
}

