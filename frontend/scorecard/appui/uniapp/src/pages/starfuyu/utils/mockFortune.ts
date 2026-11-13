/**
 * 运势 mock 降级数据（决定 16）——仅 `generateMockFortune` 从 luckyU
 * `utils/imageGenerator.ts` 剪切迁入；generateHomePageImage 为死代码不迁（决定 19）。
 *
 * 返回类型换 Task 1 Fortune（api/starfuyu）：比 luckyU FortuneData 多 id / daily_overview 两字段。
 */
import type { Fortune } from '@/api/starfuyu'

/** 生成模拟运势数据（接口失败/空时的缓存降级）。 */
export function generateMockFortune(id: number, name: string, date?: string): Fortune {
  const currentDate = date || new Date().toISOString().split('T')[0]

  return {
    // Fortune.id 是运势行 PK；mock 无真实行，借星座 id 占位（页面组件不消费该字段）
    id,
    constellation: name,
    date: currentDate,
    daily_overview: '今日运势平稳，宜规划未来。',
    overall_score: 75,
    love_score: 80,
    career_score: 70,
    wealth_score: 65,
    health_score: 85,
    lucky_color: {
      chinese: '红色',
      english: 'Red',
      hex: '#FF0000',
    },
    lucky_number: 7,
    summary: '今日运势平稳，宜规划未来。',
    love_tip: '多与伴侣进行深度交流。',
    career_tip: '处理文件时请仔细核对。',
    wealth_tip: '避免进行高风险的投资。',
    health_tip: '注意规律作息，补充水分。',
  }
}
