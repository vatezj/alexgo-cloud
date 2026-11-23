import assert from 'node:assert/strict'
import { evalTokens, ratEq } from './eval.ts'
import { dealCards, findSolution, formatExpr } from './solve.ts'

// 经典分数解 3,3,8,8：求出的解必须能被 evalTokens 精确判为 24
const s1 = findSolution([3, 3, 8, 8])!
assert.ok(s1, '[3,3,8,8] 应有解')
assert.ok(ratEq(evalTokens(s1)!, { n: 24, d: 1 }), '解求值须精确 24')

// 无解：四个 1
assert.equal(findSolution([1, 1, 1, 1]), null)

// 每个随机牌组都有解（抽 50 组）
for (let i = 0; i < 50; i++) {
  const cards = dealCards()
  assert.equal(cards.length, 4)
  assert.ok(cards.every(c => Number.isInteger(c) && c >= 1 && c <= 13))
  assert.ok(findSolution(cards), `发牌 [${cards}] 必须有解`)
}

// formatExpr 剥最外层括号
assert.equal(formatExpr(['(', 1, '+', 2, ')']), '1+2')
assert.equal(formatExpr([1, '+', 2]), '1+2')

console.log('solve.test.ts: ALL PASS')
