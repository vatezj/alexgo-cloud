/**
 * 打牌记账 4 主题 store + 组合式
 *
 * - themeId: 1|2|3|4（1=墨金 BASELINE，2=深海筹码，3=红黑赌场，4=青瓷茶室）
 * - persist：走脚手架 pinia persist 插件（store/persist.ts 按 $id 持久化），$id = 'scorecard-theme'
 * - 根节点 class：`theme-t1`..`theme-t4` 互斥；H5 挂 documentElement，小程序端由 App.ku.vue 根视图绑定
 */

export type ScorecardThemeId = 1 | 2 | 3 | 4

const THEME_CLASS_PREFIX = 'theme-t'
const THEME_IDS: readonly number[] = [1, 2, 3, 4]

/** 校正持久化里的非法 themeId（坏数据回退 1） */
function normalizeThemeId(id: unknown): ScorecardThemeId {
  return THEME_IDS.includes(id as number) ? id as ScorecardThemeId : 1
}

export const useScorecardThemeStore = defineStore('scorecard-theme', {
  state: () => ({
    themeId: 1 as ScorecardThemeId,
  }),

  actions: {
    setTheme(id: ScorecardThemeId) {
      this.themeId = normalizeThemeId(id)
    },
  },
})

/** 根节点互斥换 class（H5：documentElement；mp：见 App.ku.vue 绑定） */
function applyRootThemeClass(id: ScorecardThemeId) {
  // #ifdef H5
  const root = document.documentElement
  for (const n of THEME_IDS) {
    root.classList.remove(`${THEME_CLASS_PREFIX}${n}`)
  }
  root.classList.add(`${THEME_CLASS_PREFIX}${normalizeThemeId(id)}`)
  // #endif
  // #ifndef H5
  // 小程序无 document：根节点 class 由 App.ku.vue 根视图按 store 响应式绑定
  // #endif
}

/**
 * 打牌记账主题
 *
 * @example
 * ```ts
 * const { themeId, setTheme } = useScorecardTheme()
 * setTheme(2) // 写 store（persist）+ 换根节点 class
 * ```
 */
export function useScorecardTheme() {
  const store = useScorecardThemeStore()
  const themeId = computed(() => store.themeId)

  function setTheme(id: ScorecardThemeId) {
    store.setTheme(id)
    applyRootThemeClass(store.themeId)
  }

  return { themeId, setTheme }
}
