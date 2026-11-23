/**
 * 算24 有理数中缀表达式求值器单测（openspec fun-minigames-24-huarong）
 * 运行：node --experimental-strip-types src/utils/game24/eval.test.ts
 */
import type { Token } from './eval.ts'
import assert from 'node:assert/strict'
import { evalTokens, ratEq } from './eval.ts'

// 精确 24：8/(3-8/3)（浮点会误差的分数解）
const r1Tokens: Token[] = [8, '/', '(', 3, '-', 8, '/', 3, ')']
const r1 = evalTokens(r1Tokens)!
assert.ok(r1 && ratEq(r1, { n: 24, d: 1 }), '8/(3-8/3) 应精确等于 24')
// 优先级：1+2*3 = 7，非 9
assert.ok(ratEq(evalTokens([1, '+', 2, '*', 3])!, { n: 7, d: 1 }))
// 括号改变优先级：(1+2)*3 = 9
assert.ok(ratEq(evalTokens(['(', 1, '+', 2, ')', '*', 3])!, { n: 9, d: 1 }))
// 减法可为负：1-5
assert.ok(ratEq(evalTokens([1, '-', 5])!, { n: -4, d: 1 }))
// 除零 → null
assert.equal(evalTokens([5, '/', 0]), null)
// 非法序列 → null（不抛）
assert.equal(evalTokens([2, '+', '+', 3]), null)
assert.equal(evalTokens(['(', 1, '+', 2]), null) // 括号不闭合
assert.equal(evalTokens([1, 2]), null) // 数字连数字
assert.equal(evalTokens([]), null)
console.log('eval.test.ts: ALL PASS')
