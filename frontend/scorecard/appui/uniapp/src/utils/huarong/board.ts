/**
 * 华容道棋盘核心（纯函数，无 vue 依赖）
 * 布局：经典「横刀立马」，列 0–3、行 0–4，两空格在底行 (1,4)/(3,4)
 * 运行：node --experimental-strip-types src/utils/huarong/board.test.ts
 */

export const COLS = 4
export const ROWS = 5

export type Dir = 'up' | 'down' | 'left' | 'right'

export interface Piece {
  id: string
  name: string
  w: number
  h: number
  /** 左上角格坐标 */
  x: number
  y: number
}

/** 经典「横刀立马」初始布局（10 枚） */
export const INITIAL_PIECES: Piece[] = [
  { id: 'cao', name: '曹操', w: 2, h: 2, x: 1, y: 0 },
  { id: 'zf', name: '张飞', w: 1, h: 2, x: 0, y: 0 },
  { id: 'zy', name: '赵云', w: 1, h: 2, x: 3, y: 0 },
  { id: 'gy', name: '关羽', w: 2, h: 1, x: 1, y: 2 },
  { id: 'hz', name: '黄忠', w: 1, h: 2, x: 0, y: 2 },
  { id: 'mc', name: '马超', w: 1, h: 2, x: 3, y: 2 },
  { id: 'p1', name: '兵', w: 1, h: 1, x: 0, y: 4 },
  { id: 'p2', name: '兵', w: 1, h: 1, x: 1, y: 3 },
  { id: 'p3', name: '兵', w: 1, h: 1, x: 2, y: 3 },
  { id: 'p4', name: '兵', w: 1, h: 1, x: 2, y: 4 },
]

const DELTA: Record<Dir, { dx: number, dy: number }> = {
  up: { dx: 0, dy: -1 },
  down: { dx: 0, dy: 1 },
  left: { dx: -1, dy: 0 },
  right: { dx: 1, dy: 0 },
}

const DIRS: Dir[] = ['up', 'down', 'left', 'right']

/** 移动 1 格后是否合法：越界或目标格被占 → false */
export function canMove(pieces: Piece[], id: string, dir: Dir): boolean {
  const p = pieces.find(p => p.id === id)
  if (!p)
    return false
  const { dx, dy } = DELTA[dir]
  const nx = p.x + dx
  const ny = p.y + dy
  // 先查越界
  if (nx < 0 || ny < 0 || nx + p.w > COLS || ny + p.h > ROWS)
    return false
  // 再查与其余棋子矩形相交
  for (const q of pieces) {
    if (p === q)
      continue
    const disjoint = nx + p.w <= q.x || q.x + q.w <= nx || ny + p.h <= q.y || q.y + q.h <= ny
    if (!disjoint)
      return false
  }
  return true
}

/** 该棋子当前可移动的方向列表 */
export function movableDirs(pieces: Piece[], id: string): Dir[] {
  return DIRS.filter(dir => canMove(pieces, id, dir))
}

/** 移动 1 格，返回新数组（map 克隆目标枚，不 mutate 入参） */
export function movePiece(pieces: Piece[], id: string, dir: Dir): Piece[] {
  const { dx, dy } = DELTA[dir]
  return pieces.map(p => (p.id === id ? { ...p, x: p.x + dx, y: p.y + dy } : p))
}

/** 胜利：曹操占据底中 2×2（左上角 x=1,y=3） */
export function isWin(pieces: Piece[]): boolean {
  const cao = pieces.find(p => p.id === 'cao')
  return cao !== undefined && cao.x === 1 && cao.y === 3
}
