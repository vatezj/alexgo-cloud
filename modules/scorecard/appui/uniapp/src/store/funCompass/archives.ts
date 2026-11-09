/**
 * 趣味方位参考 · 生辰档案 store（schema v2：2026-11-06 修订轮）
 *
 * - 存储隔离（PRD §4 保持）：$id = 'fun_compass_archives'，脚手架 persist 插件
 *   （store/persist.ts）按 $id 自动 load/save —— storage 键即 PRD 指定命名空间，
 *   与账本/战绩的 scorecard-* 键完全不相交；本 store 不读不写任何账本数据。
 * - schema v2：hour/minute/gender 替代 hourIndex（时间选择器 + zhiwei gender 入参）；
 *   功能未上线无存量数据，直接换代不迁移（design.md D3）。
 * - clearAll 只清空本 state（落盘为该键的空数组），账本数据不受影响。
 * - 档案绝不上传：上传的只有排盘请求的出生字段（一次计算、不落档）。
 */

export interface FunCompassArchive {
  id: string
  /** 称呼（档案命名，可空串 → 页面兜底「未命名」） */
  name: string
  year: number
  month: number
  day: number
  hour: number
  minute: number
  gender: 'male' | 'female'
  createdAt: number
}

export type FunCompassArchiveDraft = Omit<FunCompassArchive, 'id' | 'createdAt'>

function genId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export const useFunCompassArchivesStore = defineStore('fun_compass_archives', {
  state: () => ({
    archives: [] as FunCompassArchive[],
  }),

  getters: {
    /** 新档案在前 */
    sorted(state): FunCompassArchive[] {
      return [...state.archives].sort((a, b) => b.createdAt - a.createdAt)
    },
    byId(state) {
      return (id: string): FunCompassArchive | null =>
        state.archives.find(a => a.id === id) ?? null
    },
  },

  actions: {
    add(draft: FunCompassArchiveDraft): FunCompassArchive {
      const archive: FunCompassArchive = {
        ...draft,
        id: genId(),
        createdAt: Date.now(),
      }
      this.archives.push(archive)
      return archive
    },

    /** 命中返回 true；未命中 false（编辑页按 id 预填后保存走这里） */
    update(id: string, patch: Partial<FunCompassArchiveDraft>): boolean {
      const target = this.archives.find(a => a.id === id)
      if (!target)
        return false
      Object.assign(target, patch)
      return true
    },

    remove(id: string): boolean {
      const idx = this.archives.findIndex(a => a.id === id)
      if (idx < 0)
        return false
      this.archives.splice(idx, 1)
      return true
    },

    /** 关于页「清除本工具全部档案」：只清本键，账本不动 */
    clearAll(): void {
      this.archives = []
    },
  },
})
