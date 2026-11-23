import type { Piece } from './board.ts'
import assert from 'node:assert/strict'
import { canMove, COLS, INITIAL_PIECES, isWin, movableDirs, movePiece, ROWS } from './board.ts'

function assertValid(pieces: Piece[]) {
  for (const p of pieces) {
    assert.ok(p.x >= 0 && p.y >= 0 && p.x + p.w <= COLS && p.y + p.h <= ROWS, `${p.id} 越界`)
    for (const q of pieces) {
      if (p !== q)
        assert.ok(p.x + p.w <= q.x || q.x + q.w <= p.x || p.y + p.h <= q.y || q.y + q.h <= p.y, `${p.id} 与 ${q.id} 重叠`)
    }
  }
}

assertValid(INITIAL_PIECES)
assert.equal(INITIAL_PIECES.length, 10)
assert.equal(isWin(INITIAL_PIECES), false)

// 初始可动性（依据上方布局表逐格推演写死）
assert.deepEqual(movableDirs(INITIAL_PIECES, 'p1'), ['right']) // (0,4)：右邻 (1,4) 空
assert.deepEqual(movableDirs(INITIAL_PIECES, 'gy'), []) // 关羽 (1,2)：上曹、下 p2/p3、左右 hz/mc 全挡
// 越界
assert.equal(canMove(INITIAL_PIECES, 'p1', 'down'), false) // y=4 再下越界
// 占格
assert.equal(canMove(INITIAL_PIECES, 'cao', 'up'), false) // (1,0) 上方越界
assert.equal(canMove(INITIAL_PIECES, 'p2', 'down'), true) // (1,3)→(1,4) 空：合法
assert.equal(canMove(INITIAL_PIECES, 'p3', 'down'), false) // (2,3)→(2,4) 是 p4：占格

// 合法移动后局面有效
const after = movePiece(INITIAL_PIECES, 'p1', 'right')
assertValid(after)
assert.equal(after.find(p => p.id === 'p1')!.x, 1) // 入参不 mutate：原件仍 x=0
assert.equal(INITIAL_PIECES.find(p => p.id === 'p1')!.x, 0)

// 胜利判定
assert.equal(isWin(INITIAL_PIECES), false)
const won = INITIAL_PIECES.map(p => p.id === 'cao' ? { ...p, x: 1, y: 3 } : p)
assert.equal(isWin(won), true)
const notWon = INITIAL_PIECES.map(p => p.id === 'cao' ? { ...p, x: 0, y: 3 } : p)
assert.equal(isWin(notWon), false) // 底部但不在正中

console.log('board.test.ts: ALL PASS')
