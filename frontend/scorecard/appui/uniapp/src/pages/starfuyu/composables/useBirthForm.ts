/**
 * 生辰表单管理 Composable（迁自 luckyU bazhi/composables/useBirthForm.ts，逻辑原样）
 *
 * - getBirthData() 返回 { birth_date, birth_time, birth_place } → 对齐 Task 1 BaziReq
 *   （birth_time 取时辰中值 'HH:mm:ss'；birth_place 源稿即空串，后端可选）
 * - ref 走 appui 自动导入；年份区间 1950-2025 与默认值沿用源稿（非目标：不改交互）
 */
export function useBirthForm() {
  /* ---- 年份 ---- */
  const years = ref<string[]>([])
  for (let i = 1950; i <= 2025; i++) {
    years.value.push(String(i))
  }
  const yearIndex = ref(45) // 默认 1995

  /* ---- 月份 ---- */
  const months = ref<string[]>([])
  for (let i = 1; i <= 12; i++) {
    months.value.push(String(i).padStart(2, '0'))
  }
  const monthIndex = ref(7) // 默认 08

  /* ---- 日 ---- */
  const days = ref<string[]>([])
  for (let i = 1; i <= 31; i++) {
    days.value.push(String(i).padStart(2, '0'))
  }
  const dayIndex = ref(14) // 默认 15

  /* ---- 时辰 ---- */
  const shichenList = ref([
    '子时 (23:00-01:00)',
    '丑时 (01:00-03:00)',
    '寅时 (03:00-05:00)',
    '卯时 (05:00-07:00)',
    '辰时 (07:00-09:00)',
    '巳时 (09:00-11:00)',
    '午时 (11:00-13:00)',
    '未时 (13:00-15:00)',
    '申时 (15:00-17:00)',
    '酉时 (17:00-19:00)',
    '戌时 (19:00-21:00)',
    '亥时 (21:00-23:00)',
  ])
  const shichenIndex = ref(4) // 默认 辰时

  /* ---- 事件处理 ---- */
  function onYearChange(e: any) {
    yearIndex.value = Number(e.detail.value)
  }
  function onMonthChange(e: any) {
    monthIndex.value = Number(e.detail.value)
  }
  function onDayChange(e: any) {
    dayIndex.value = Number(e.detail.value)
  }
  function onShichenChange(e: any) {
    shichenIndex.value = Number(e.detail.value)
  }

  // 获取完整的生辰信息字符串
  function getBirthInfo() {
    return `${years.value[yearIndex.value]}年${months.value[monthIndex.value]}月${days.value[dayIndex.value]}日 ${shichenList.value[shichenIndex.value]}`
  }

  // 将时辰转换为时间（取时辰范围的中间值）
  function shichenToTime(shichen: string): string {
    const timeMap: Record<string, string> = {
      '子时 (23:00-01:00)': '00:00:00',
      '丑时 (01:00-03:00)': '02:00:00',
      '寅时 (03:00-05:00)': '04:00:00',
      '卯时 (05:00-07:00)': '06:00:00',
      '辰时 (07:00-09:00)': '08:00:00',
      '巳时 (09:00-11:00)': '10:00:00',
      '午时 (11:00-13:00)': '12:00:00',
      '未时 (13:00-15:00)': '14:00:00',
      '申时 (15:00-17:00)': '16:00:00',
      '酉时 (17:00-19:00)': '18:00:00',
      '戌时 (19:00-21:00)': '20:00:00',
      '亥时 (21:00-23:00)': '22:00:00',
    }
    return timeMap[shichen] || '12:00:00'
  }

  // 获取 API 请求参数
  function getBirthData() {
    const year = years.value[yearIndex.value]
    const month = months.value[monthIndex.value]
    const day = days.value[dayIndex.value]
    const shichen = shichenList.value[shichenIndex.value]

    return {
      birth_date: `${year}-${month}-${day}`,
      birth_time: shichenToTime(shichen),
      birth_place: '',
    }
  }

  return {
    years,
    months,
    days,
    shichenList,
    yearIndex,
    monthIndex,
    dayIndex,
    shichenIndex,
    onYearChange,
    onMonthChange,
    onDayChange,
    onShichenChange,
    getBirthInfo,
    getBirthData,
  }
}
