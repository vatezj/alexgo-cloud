/**
 * 结算 IOU 贪心简化（纯函数：无 uni API / 无 store，可在 console 直接验收）
 *
 * 规则（Task 9 brief Step 1）：
 * - 只取非零分成员；正分=债权人、负分=债务人，两侧按分值降序
 * - 反复配对「最大债权人 × 最大债务人」，简化额 = min(债权, 债务)，一方清零后推进指针
 * - 输出 {from_user_id, to_user_id, amount}：from=债务人（负分侧），to=债权人（正分侧），amount>0
 * - 空输入 / 全零 → []
 *
 * 验收用例（brief Step 3，console 手测）：
 * - [{u:1,s:100},{u:2,s:-60},{u:3,s:-40}] → [{from:2,to:1,amount:60},{from:3,to:1,amount:40}]
 * - 全零 → []
 * - [{u:1,s:50},{u:2,s:-50}] → [{from:2,to:1,amount:50}]
 */

/** IOU 计算输入成员（只消费这三个字段）。 */
export interface IouMember {
  user_id: number
  nickname: string
  cur_score: number
}

/** 一条简化记分边：from（债务人，负分）→ to（债权人，正分），amount 恒为正。 */
export interface IouEdge {
  from_user_id: number
  to_user_id: number
  amount: number
}

/** 贪心简化：最小化「谁记给谁多少分」的条数。 */
export function calcIou(members: IouMember[]): IouEdge[] {
  const creditors: { user_id: number, rest: number }[] = []
  const debtors: { user_id: number, rest: number }[] = []

  for (const m of members) {
    if (m.cur_score > 0)
      creditors.push({ user_id: m.user_id, rest: m.cur_score })
    else if (m.cur_score < 0)
      debtors.push({ user_id: m.user_id, rest: -m.cur_score })
  }

  // 债权 / 债务额降序：贪心先消最大额
  creditors.sort((a, b) => b.rest - a.rest)
  debtors.sort((a, b) => b.rest - a.rest)

  const edges: IouEdge[] = []
  let ci = 0
  let di = 0

  while (ci < creditors.length && di < debtors.length) {
    const cred = creditors[ci]
    const debt = debtors[di]
    const amount = Math.min(cred.rest, debt.rest)
    if (amount > 0) {
      edges.push({
        from_user_id: debt.user_id,
        to_user_id: cred.user_id,
        amount,
      })
    }
    cred.rest -= amount
    debt.rest -= amount
    // 每笔至少清零一侧；被清零的一侧推进到下一个债权人/债务人
    if (cred.rest === 0)
      ci++
    if (debt.rest === 0)
      di++
  }

  return edges
}
