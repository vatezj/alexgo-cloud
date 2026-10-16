import { baseRequestClient, requestClient } from '#/api/request';

export namespace AuthApi {
  /** refresh 业务载荷（后端 system/service.LoginResult 同形）。 */
  export interface RefreshTokenData {
    expires_in: number;
    refresh_token: string;
    token: string;
  }

  /** 统一信封体（pkg/resp 唯一出站出口：{code,msg,data}）。 */
  export interface RefreshTokenEnvelope {
    code: number;
    data: RefreshTokenData;
    msg: string;
  }

  /**
   * baseRequestClient 为裸 client（responseReturn:'raw'、无信封拦截器）——
   * RequestClient.request<T> 直接返回 AxiosResponse（response as T），
   * 故按 AxiosResponse 鸭子形状声明：data=信封体，status=HTTP 状态。
   */
  export interface RefreshTokenResult {
    data: RefreshTokenEnvelope;
    status: number;
  }
}

/**
 * 刷新 accessToken——信封读法：resp.data=信封体 {code,msg,data}，
 * 新令牌在 resp.data.data.token（旧形态 resp.data 直为 token 已失效）。
 * 启用 refresh 链（preferences.app.enableRefreshToken=true）前的剩余缺口：
 * 请求体还须传 refresh_token（当前 withCredentials 误放 data 位，后端会拒
 * CodeParamMissing），且刷新后须回写轮换的 refresh_token——一期关闭、暂不可达。
 */
export async function refreshTokenApi() {
  return baseRequestClient.post<AuthApi.RefreshTokenResult>(
    '/auth/refresh',
    undefined,
    { withCredentials: true },
  );
}

/**
 * 退出登录
 */
export async function logoutApi() {
  // alexGo：走 requestClient（带 Authorization）——后端按 token 撤销；
  // 后端恒 200，撤销失败也不阻断前端登出流程。refreshTokenApi 已迁信封读法
  // （resp.data.data.token），refresh 链一期仍关闭（enableRefreshToken:false）。
  return requestClient.post('/auth/logout');
}

/**
 * 获取用户权限码
 */
export async function getAccessCodesApi() {
  return requestClient.get<string[]>('/auth/codes');
}
