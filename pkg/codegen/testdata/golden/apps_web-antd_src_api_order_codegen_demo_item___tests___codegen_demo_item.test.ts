// 生成代码，勿手改（codegen M4 · codegen_demo_items）。
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

  it('eq 精确匹配且 String 双向转换', () => {
    expect(matchQuery(rows, { id: 1 })).toHaveLength(1);
    expect(matchQuery(rows, { id: '1' })).toHaveLength(1);
    expect(matchQuery(rows, { id: 99 })).toHaveLength(0);
  });

  it('like 子串匹配且不区分大小写', () => {
    expect(matchQuery(rows, { name: 'ALP' })).toHaveLength(1);
    expect(matchQuery(rows, { name: 'et' })).toHaveLength(1);
  });

  it('多条件 AND 组合', () => {
    expect(matchQuery(rows, { name: 'a', status: 1 })).toHaveLength(1);
  });
});

describe('queryOps', () => {
  it('按列类型给出操作符（string→like、time→between、其余 eq）', () => {
    expect(queryOps.id).toBe('eq');
    expect(queryOps.name).toBe('like');
    expect(queryOps.created_at).toBe('between');
  });
});
