import Uni from '@uni-helper/plugin-uni'
import { isMpWeixin } from '@uni-helper/uni-env'
import UniHelperComponents from '@uni-helper/vite-plugin-uni-components'
import UniHelperLayouts from '@uni-helper/vite-plugin-uni-layouts'
import UniHelperManifest from '@uni-helper/vite-plugin-uni-manifest'
import UniHelperPages from '@uni-helper/vite-plugin-uni-pages'
import Optimization from '@uni-ku/bundle-optimizer'
import UniKuRoot from '@uni-ku/root'
import { UniEchartsResolver } from 'uni-echarts/resolver'
import { UniEcharts } from 'uni-echarts/vite'
import UnoCSS from 'unocss/vite'
import AutoImport from 'unplugin-auto-import/vite'
import { defineConfig } from 'vite'
import { WotResolver } from './src/resolver'
// https://vitejs.dev/config/
export default defineConfig({
  base: './',
  optimizeDeps: {
    exclude: ['@wot-ui/ui', 'uni-echarts'],
  },
  plugins: [
    // https://github.com/uni-helper/vite-plugin-uni-manifest
    UniHelperManifest(),
    // https://github.com/uni-helper/vite-plugin-uni-pages
    UniHelperPages({
      dts: 'src/uni-pages.d.ts',
      /**
       * 排除的页面，相对于 dir 和 subPackages
       * @default []
       */
      exclude: ['**/components/**/*.*'],
      /**
       * 分包扫描根（fun-compass 变更）：pages/fun 下六页进 subPackages，
       * root 自动派生为 pages/fun（路由前缀 pages/fun/compass/* 不变），主包自动剔除
       * starfuyu 分包（luckyu-into-scorecard）：pages/starfuyu 下 index/bazi 两页同理进 subPackages
       * 算24 分包 fun-minigames-24-huarong：pages/game24 进 subPackages
       */
      subPackages: ['src/pages/fun', 'src/pages/starfuyu', 'src/pages/game24'],
    }),
    // https://github.com/uni-helper/vite-plugin-uni-layouts
    UniHelperLayouts(),
    // https://github.com/uni-helper/vite-plugin-uni-components
    UniHelperComponents({
      resolvers: [WotResolver(), UniEchartsResolver()],
      dts: 'src/components.d.ts',
      dirs: ['src/components'],
      directoryAsNamespace: true,
    }),
    // https://github.com/uni-ku/root
    UniKuRoot(),
    // https://uni-echarts.xiaohe.ink
    UniEcharts(),
    // https://uni-helper.cn/plugin-uni
    Uni(),
    // https://github.com/uni-ku/bundle-optimizer
    Optimization({
      enable: isMpWeixin,
      logger: false,
    }),
    // https://github.com/antfu/unplugin-auto-import
    AutoImport({
      imports: ['vue', '@vueuse/core', 'pinia', 'uni-app', {
        from: '@wot-ui/router',
        imports: ['createRouter', 'useRouter', 'useRoute'],
      }, {
        from: '@wot-ui/ui',
        imports: ['useToast', 'useDialog', 'useNotify', 'CommonUtil'],
      }, {
        from: 'alova/client',
        imports: ['usePagination', 'useRequest'],
      }],
      dts: 'src/auto-imports.d.ts',
      dirs: ['src/composables/**', 'src/store/**', 'src/utils', 'src/api'],
      vueTemplate: true,
    }),
    // https://github.com/antfu/unocss
    // see unocss.config.ts for config
    UnoCSS(),
  ],
  css: {
    preprocessorOptions: {
      scss: {
        api: 'modern-compiler',
        silenceDeprecations: ['legacy-js-api'],
      },
    },
  },
})
