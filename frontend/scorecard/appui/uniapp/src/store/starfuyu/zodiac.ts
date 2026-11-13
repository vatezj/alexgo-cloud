/**
 * starfuyu 星座 store（服务端 profile 权威）
 *
 * - persist：走脚手架 pinia persist 插件（$id = 'starfuyu-zodiac'，不在排除列表即自动持久化）
 * - currentName 只存服务端认可的星座中文名；'' = 未设
 * - select：先 PUT /profile 成功才落本地；失败抛错由 UI toast（服务端赢，本地不落）
 */
import { setProfile } from '@/api/starfuyu'

export interface Zodiac {
  id: number
  name: string
  enName: string
  dateRange: string
  icon: string
  mascot: string
  luckyColor: string
  luckyColors: string[]
}

/** 12 星座静态表（自 modules/starfuyu/luckyU 迁入；icon/mascot 指向分包 static 目标路径，资产见 Task 4）。 */
export const zodiacList: Zodiac[] = [
  {
    id: 1,
    name: '白羊座',
    enName: 'Aries',
    dateRange: '3.21-4.19',
    icon: '/static/starfuyu/zodiac/icons/aries.png',
    mascot: '/static/starfuyu/zodiac/mascots/aries.png',
    luckyColor: '#FF4500',
    luckyColors: ['#FF6B35', '#FF8C42'],
  },
  {
    id: 2,
    name: '金牛座',
    enName: 'Taurus',
    dateRange: '4.20-5.20',
    icon: '/static/starfuyu/zodiac/icons/taurus.png',
    mascot: '/static/starfuyu/zodiac/mascots/taurus.png',
    luckyColor: '#228B22',
    luckyColors: ['#32CD32', '#90EE90'],
  },
  {
    id: 3,
    name: '双子座',
    enName: 'Gemini',
    dateRange: '5.21-6.21',
    icon: '/static/starfuyu/zodiac/icons/gemini.png',
    mascot: '/static/starfuyu/zodiac/mascots/gemini.png',
    luckyColor: '#FFD700',
    luckyColors: ['#FFE4B5', '#87CEEB'],
  },
  {
    id: 4,
    name: '巨蟹座',
    enName: 'Cancer',
    dateRange: '6.22-7.22',
    icon: '/static/starfuyu/zodiac/icons/cancer.png',
    mascot: '/static/starfuyu/zodiac/mascots/cancer.png',
    luckyColor: '#C0C0C0',
    luckyColors: ['#F5F5F5', '#FFFFFF'],
  },
  {
    id: 5,
    name: '狮子座',
    enName: 'Leo',
    dateRange: '7.23-8.22',
    icon: '/static/starfuyu/zodiac/icons/leo.png',
    mascot: '/static/starfuyu/zodiac/mascots/leo.png',
    luckyColor: '#FFD700',
    luckyColors: ['#FFA500', '#FF8C00'],
  },
  {
    id: 6,
    name: '处女座',
    enName: 'Virgo',
    dateRange: '8.23-9.22',
    icon: '/static/starfuyu/zodiac/icons/virgo.png',
    mascot: '/static/starfuyu/zodiac/mascots/virgo.png',
    luckyColor: '#DEB887',
    luckyColors: ['#D2B48C', '#8B4513'],
  },
  {
    id: 7,
    name: '天秤座',
    enName: 'Libra',
    dateRange: '9.23-10.23',
    icon: '/static/starfuyu/zodiac/icons/libra.png',
    mascot: '/static/starfuyu/zodiac/mascots/libra.png',
    luckyColor: '#FFC0CB',
    luckyColors: ['#87CEEB', '#B0E0E6'],
  },
  {
    id: 8,
    name: '天蝎座',
    enName: 'Scorpio',
    dateRange: '10.24-11.22',
    icon: '/static/starfuyu/zodiac/icons/scorpio.png',
    mascot: '/static/starfuyu/zodiac/mascots/scorpio.png',
    luckyColor: '#8B0000',
    luckyColors: ['#000000', '#4B0082'],
  },
  {
    id: 9,
    name: '射手座',
    enName: 'Sagittarius',
    dateRange: '11.23-12.21',
    icon: '/static/starfuyu/zodiac/icons/sagittarius.png',
    mascot: '/static/starfuyu/zodiac/mascots/sagittarius.png',
    luckyColor: '#8A2BE2',
    luckyColors: ['#4169E1', '#0000CD'],
  },
  {
    id: 10,
    name: '摩羯座',
    enName: 'Capricorn',
    dateRange: '12.22-1.19',
    icon: '/static/starfuyu/zodiac/icons/capricorn.png',
    mascot: '/static/starfuyu/zodiac/mascots/capricorn.png',
    luckyColor: '#654321',
    luckyColors: ['#228B22', '#006400'],
  },
  {
    id: 11,
    name: '水瓶座',
    enName: 'Aquarius',
    dateRange: '1.20-2.18',
    icon: '/static/starfuyu/zodiac/icons/aquarius.png',
    mascot: '/static/starfuyu/zodiac/mascots/aquarius.png',
    luckyColor: '#00CED1',
    luckyColors: ['#87CEEB', '#C0C0C0'],
  },
  {
    id: 12,
    name: '双鱼座',
    enName: 'Pisces',
    dateRange: '2.19-3.20',
    icon: '/static/starfuyu/zodiac/icons/pisces.png',
    mascot: '/static/starfuyu/zodiac/mascots/pisces.png',
    luckyColor: '#20B2AA',
    luckyColors: ['#AFEEEE', '#E0FFFF'],
  },
]

export const useZodiacStore = defineStore('starfuyu-zodiac', {
  state: () => ({
    /** 服务端认可的星座中文名；'' = 未设。 */
    currentName: '' as string,
  }),

  getters: {
    /** 当前星座条目；currentName 不在表内（未设/异常值）→ null。 */
    current: (s): Zodiac | null => zodiacList.find(z => z.name === s.currentName) ?? null,
    /** 是否已选星座。 */
    hasSelected: (s): boolean => s.currentName !== '',
  },

  actions: {
    /** 服务端回填/本地缓存写入（不打接口）。 */
    hydrate(constellation: string): void {
      this.currentName = constellation
    },

    /**
     * 选座：先 PUT /profile，成功才 hydrate；失败抛错由 UI toast（本地不落，服务端赢）。
     */
    async select(constellation: string): Promise<void> {
      await setProfile(constellation)
      this.currentName = constellation
    },
  },
})
