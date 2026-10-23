import { requestClient } from '#/api/request';

export interface FishgameFish {
  id: number;
  fish_id: string;
  name: string;
  rarity: string;
  period: string;
  depth: string;
  flavor: string;
  sort: number;
  status: number;
  created_at?: string;
  updated_at?: string;
}
export interface FishgameConfig {
  id: number;
  type: string;
  period: string;
  depth_bias: number;
  fish_id: string;
  week: string;
  start_at?: string;
  end_at?: string;
  status: number;
}
export interface FishgameEvent {
  id: number;
  player_id: number;
  name: string;
  t: number;
  props: string;
  created_at?: string;
}
export interface FishgamePlayer {
  id: number;
  openid: string;
  nick: string;
  dex: number;
  created_at: string;
  last_login_at?: string;
}
export interface FishgameStats {
  player_total: number;
  player_today: number;
  event_counts: Record<string, number>;
  daily: Array<{ day: string; name: string; count: number }>;
  funnel: Record<string, number>;
  funnel_ratio: Record<string, number>;
  sign_count: number;
  basket_count: number;
  ad_count: number;
}

export interface PageData<T> {
  list: T[];
  total: number;
  page: number;
  size: number;
}

// ---- fish ----
export function getFishgameFishesApi(params: Record<string, any>) {
  return requestClient.get<PageData<FishgameFish>>('/admin/fishgame/fishes', { params });
}
export function createFishgameFishApi(data: Partial<FishgameFish>) {
  return requestClient.post('/admin/fishgame/fishes', data);
}
export function updateFishgameFishApi(id: number, data: Partial<FishgameFish>) {
  return requestClient.put(`/admin/fishgame/fishes/${id}`, data);
}
export function deleteFishgameFishApi(id: number) {
  return requestClient.delete(`/admin/fishgame/fishes/${id}`);
}

// ---- config ----
export function getFishgameConfigsApi(params: Record<string, any>) {
  return requestClient.get<PageData<FishgameConfig>>('/admin/fishgame/configs', { params });
}
export function createFishgameConfigApi(data: Partial<FishgameConfig>) {
  return requestClient.post('/admin/fishgame/configs', data);
}
export function updateFishgameConfigApi(id: number, data: Partial<FishgameConfig>) {
  return requestClient.put(`/admin/fishgame/configs/${id}`, data);
}
export function deleteFishgameConfigApi(id: number) {
  return requestClient.delete(`/admin/fishgame/configs/${id}`);
}

// ---- events ----
export function getFishgameEventsApi(params: Record<string, any>) {
  return requestClient.get<PageData<FishgameEvent>>('/admin/fishgame/events', { params });
}

// ---- players ----
export function getFishgamePlayersApi(params: Record<string, any>) {
  return requestClient.get<PageData<FishgamePlayer>>('/admin/fishgame/players', { params });
}
export function getFishgamePlayerDetailApi(id: number) {
  return requestClient.get<{
    player: any;
    lit: Array<{ fish_id: string; count: number }>;
    events: FishgameEvent[];
  }>(`/admin/fishgame/players/${id}`);
}
export function getFishgamePlayerRankApi(id: number) {
  return requestClient.get<{ position: number; total: number }>(
    `/admin/fishgame/players/${id}/rank`,
  );
}

// ---- stats ----
export function getFishgameStatsApi() {
  return requestClient.get<FishgameStats>('/admin/fishgame/stats');
}
