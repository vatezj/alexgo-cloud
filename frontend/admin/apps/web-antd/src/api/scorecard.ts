import { requestClient } from '#/api/request';

export interface ScorecardRoom {
  id: number;
  room_code: string;
  creator_id: number;
  status: number;
  tea_pool_total: number;
  tea_percent: number;
  tea_target_amount: number;
  max_lose_score: number;
  round_no: number;
  expire_time: string;
  created_at: string;
}
export interface ScorecardRecord {
  id: number;
  room_id: number;
  user_id: number;
  score: number;
  cum_lose_score: number;
  tea_pool_total: number;
  settle_type: number;
  created_at: string;
}
export interface ScorecardFriend {
  id: number;
  user_id: number;
  friend_user_id: number;
  rank_enabled: number;
  created_at: string;
}

export function getRoomsApi(params: Record<string, any>) {
  return requestClient.get<{ list: ScorecardRoom[]; total: number }>(
    '/admin/scorecard/rooms',
    { params },
  );
}
export function getRoomDetailApi(id: number) {
  return requestClient.get(`/admin/scorecard/rooms/${id}`);
}
export function closeRoomApi(id: number) {
  return requestClient.post(`/admin/scorecard/rooms/${id}/close`);
}
export function getRecordsApi(params: Record<string, any>) {
  return requestClient.get<{ list: ScorecardRecord[]; total: number }>(
    '/admin/scorecard/records',
    { params },
  );
}
export function getFriendsApi(params: Record<string, any>) {
  return requestClient.get<{ list: ScorecardFriend[]; total: number }>(
    '/admin/scorecard/friends',
    { params },
  );
}
