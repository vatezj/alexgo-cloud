// 生成代码，勿手改（codegen M4 · room_users）。
import { describe, expect, it, vi } from 'vitest';

import { matchQuery, queryOps } from '../api';

// 隔离 request.ts（axios 链在 node 环境不可用）：matchQuery 是纯函数，但同模块会 import 客户端。
vi.mock('#/api/request', () => ({
  requestClient: {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}));

interface Row {
  id?: number;
  name?: string;
  status?: number;
}

const rows: Row[] = [
  { id: 1, name: 'Alpha', status: 1 },
  { id: 2, name: 'beta', status: 0 },
  { id: 3, name: '', status: 1 },
];

describe('matchQuery', () => {
  it('空表单值返回全部行', () => {
    expect(matchQuery(rows, {})).toHaveLength(3);
    expect(matchQuery(rows, { name: '' })).toHaveLength(3);
    expect(matchQuery(rows, { name: null })).toHaveLength(3);
    expect(matchQuery(rows, { name: undefined })).toHaveLength(3);
  });

  it('未配置操作符的键默认 eq（String 双向精确匹配）', () => {
    // 表无关：用 queryOps 里不存在的键走 ?? 'eq' 分支。
    const key = '__not_configured__';
    expect(matchQuery(rows, { [key]: 'x' })).toHaveLength(0);
    expect(matchQuery(rows, { id: 1, [key]: '' })).toHaveLength(1);
  });

  it('精确全值匹配在 eq/like 语义下均命中', () => {
    // 用完整单元格值：eq 精确相等与 like 子串包含都成立，不依赖列的 query_op 配置。
    expect(matchQuery(rows, { name: 'Alpha' })).toHaveLength(1);
    expect(matchQuery(rows, { id: '1', name: 'Alpha' })).toHaveLength(1);
    expect(matchQuery(rows, { id: 99 })).toHaveLength(0);
  });

  it('多条件 AND 组合', () => {
    expect(matchQuery(rows, { name: 'Alpha', status: 1 })).toHaveLength(1);
    expect(matchQuery(rows, { name: 'Alpha', status: 0 })).toHaveLength(0);
  });
});

describe('queryOps', () => {
  it('值域限于 eq/like/between（键=启用了查询的列）', () => {
    for (const [key, op] of Object.entries(queryOps)) {
      expect(['eq', 'like', 'between'], `queryOps.${key}`).toContain(op);
    }
  });
});
