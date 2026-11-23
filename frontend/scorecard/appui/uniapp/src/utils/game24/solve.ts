/**
 * 算24 穷举求解器 + 有解发牌（openspec fun-minigames-24-huarong）
 * 穷举：每次取两张数 a,b 试 a⊕b / b⊕a（-、/ 不交换），连同余数递归；
 * 剩一张时用 evalTokens 精确判 24。子结果一律包 (…) ，冗余括号由 formatExpr 展示层处理。
 */
import type { Token } from './eval.ts'
import { evalTokens, ratEq } from './eval.ts'

const OPS = ['+', '-', '*', '/'] as const

/** 整串是否被最外层一对括号完整包裹（其匹配 ')' 落在末位） */
function wrapsWhole(t: Token[]): boolean {
  if (t.length < 2 || t[0] !== '(' || t[t.length - 1] !== ')')
    return false
  let depth = 0
  for (let i = 0; i < t.length; i++) {
    if (t[i] === '(') {
      depth++
    }
    else if (t[i] === ')') {
      depth--
      if (depth === 0)
        return i === t.length - 1
    }
  }
  return false
}

function solve(items: Token[][]): Token[] | null {
  if (items.length === 1) {
    const v = evalTokens(items[0])
    return v !== null && ratEq(v, { n: 24, d: 1 }) ? items[0] : null
  }
  for (let i = 0; i < items.length; i++) {
    for (let j = 0; j < items.length; j++) {
      if (i === j)
        continue
      const a = items[i]
      const b = items[j]
      const rest = items.filter((_, k) => k !== i && k !== j)
      for (const op of OPS) {
        const cand: Token[] = ['(', ...a, op, ...b, ')']
        if (evalTokens(cand) === null)
          continue // 除零剪枝
        const found = solve([...rest, cand])
        if (found !== null)
          return found
      }
    }
  }
  return null
}

/**
 * 求 24 解：返回每个二元运算都带括号的中缀 Token 序列（evalTokens 可直接求值），
 * 无解返回 null。
 */
export function findSolution(cards: number[]): Token[] | null {
  if (cards.length === 0)
    return null
  return solve(cards.map(c => [c]))
}

/** 展示串：剥除最外层一对冗余括号（如 (1+2) → 1+2），其余保留 */
export function formatExpr(tokens: Token[]): string {
  const t = wrapsWhole(tokens) ? tokens.slice(1, -1) : tokens
  return t.join('')
}

/** 发 4 张 1–13（可重复），有解才返回；rng 默认 Math.random，1000 次上限兜底 */
export function dealCards(rng: () => number = Math.random): number[] {
  for (let attempt = 0; attempt < 1000; attempt++) {
    const cards = Array.from({ length: 4 }, () => 1 + Math.floor(rng() * 13))
    if (findSolution(cards) !== null)
      return cards
  }
  throw new Error('dealCards: 1000 次内未摇到有解牌组（rng 异常？）')
}
