/**
 * 算24 有理数中缀表达式求值器（openspec fun-minigames-24-huarong）
 * 纯函数：递归下降（加减 → 乘除 → 数字/括号）+ 有理数四则，全程精确分数。
 * 语法非法或除零 → null，不抛异常。
 */

export type Token = number | '+' | '-' | '*' | '/' | '(' | ')'

export interface Rational {
  /** 分子（负号在此），既约 */
  n: number
  /** 分母，恒 > 0，既约 */
  d: number
}

function gcd(a: number, b: number): number {
  let x = Math.abs(a)
  let y = Math.abs(b)
  while (y !== 0) {
    const t = x % y
    x = y
    y = t
  }
  return x
}

/** 约分归一（d === 0 抛错仅供内部，求值路径不调它） */
export function rat(n: number, d: number): Rational {
  if (d === 0)
    throw new Error('rat: denominator is 0')
  if (d < 0) {
    n = -n
    d = -d
  }
  const g = gcd(n, d)
  return { n: n / g, d: d / g }
}

export function ratEq(a: Rational, b: Rational): boolean {
  return a.n * b.d === b.n * a.d
}

function ratAdd(a: Rational, b: Rational): Rational {
  return rat(a.n * b.d + b.n * a.d, a.d * b.d)
}

function ratSub(a: Rational, b: Rational): Rational {
  return rat(a.n * b.d - b.n * a.d, a.d * b.d)
}

function ratMul(a: Rational, b: Rational): Rational {
  return rat(a.n * b.n, a.d * b.d)
}

function ratDiv(a: Rational, b: Rational): Rational | null {
  if (b.n === 0)
    return null // 除零
  return rat(a.n * b.d, a.d * b.n)
}

/**
 * 中缀求值：标准优先级 + 括号。
 * 语法非法或除零 → null。
 */
export function evalTokens(tokens: Token[]): Rational | null {
  let pos = 0

  // 加减层
  function parseExpr(): Rational | null {
    let left: Rational | null = parseTerm()
    if (left === null)
      return null
    while (pos < tokens.length && (tokens[pos] === '+' || tokens[pos] === '-')) {
      const op = tokens[pos]
      pos++
      const right = parseTerm()
      if (right === null)
        return null
      // 显式标注打破 left↔v 循环推断（vue-tsc TS7022）
      const v: Rational = op === '+' ? ratAdd(left, right) : ratSub(left, right)
      left = v
    }
    return left
  }

  // 乘除层
  function parseTerm(): Rational | null {
    let left: Rational | null = parseFactor()
    if (left === null)
      return null
    while (pos < tokens.length && (tokens[pos] === '*' || tokens[pos] === '/')) {
      const op = tokens[pos]
      pos++
      const right = parseFactor()
      if (right === null)
        return null
      const v: Rational | null = op === '*' ? ratMul(left, right) : ratDiv(left, right)
      if (v === null)
        return null // 除零
      left = v
    }
    return left
  }

  // 数字 / 括号
  function parseFactor(): Rational | null {
    if (pos >= tokens.length)
      return null
    const t = tokens[pos]
    if (typeof t === 'number') {
      pos++
      return rat(t, 1)
    }
    if (t === '(') {
      pos++
      const inner = parseExpr()
      if (inner === null)
        return null
      if (pos >= tokens.length || tokens[pos] !== ')')
        return null // 括号不闭合
      pos++
      return inner
    }
    return null // 期望数字或 '('，实际为运算符/')'
  }

  const result = parseExpr()
  if (result === null)
    return null
  if (pos !== tokens.length)
    return null // 尾部残留 token（如 数字连数字、多余括号）
  return result
}
