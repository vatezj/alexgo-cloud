import { requestClient } from '#/api/request';

export interface TaokoulingChannel {
  id: number;
  name: string;
  partner: string;
  host: string;
  pid: string;
  apikey: string;
  tb_name: string;
  status: number;
  created_at: string;
  updated_at: string;
}
export interface TaokoulingLog {
  id: number;
  endpoint: string;
  partner: string;
  input_text: string;
  extracted_tkl: string;
  item_id: string;
  status: number;
  error: string;
  duration_ms: number;
  ip: string;
  created_at: string;
}

/** resp.Page 信封 data:{list,total,page,size}（size≤100） */
interface PageResult<T> {
  list: T[];
  total: number;
  page: number;
  size: number;
}

export function listChannelsApi(params: Record<string, any>) {
  return requestClient.get<PageResult<TaokoulingChannel>>(
    '/admin/taokouling/channels',
    { params },
  );
}
export function createChannelApi(data: Partial<TaokoulingChannel>) {
  return requestClient.post<TaokoulingChannel>(
    '/admin/taokouling/channels',
    data,
  );
}
export function updateChannelApi(
  id: number,
  data: Partial<TaokoulingChannel>,
) {
  return requestClient.put<TaokoulingChannel>(
    `/admin/taokouling/channels/${id}`,
    data,
  );
}
export function deleteChannelApi(id: number) {
  return requestClient.delete(`/admin/taokouling/channels/${id}`);
}
export function listLogsApi(params: Record<string, any>) {
  return requestClient.get<PageResult<TaokoulingLog>>(
    '/admin/taokouling/logs',
    { params },
  );
}
