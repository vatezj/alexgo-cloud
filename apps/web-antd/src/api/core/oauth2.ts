import { useAppConfig } from '@vben/hooks';

/**
 * OAuth2 协议端点调用——必须用裸 fetch（不走 requestClient）：
 * ① RFC 6749 错误信封 {error,error_description} 不经过 vben 的 {code,data} 拦截器；
 * ② authorize/login 成功是 302，需要 redirect:'follow' 后从 resp.url 提取 code。
 */
const { apiURL } = useAppConfig(import.meta.env, import.meta.env.PROD);

/** PKCE 会话的 sessionStorage 键（登录页写入，/callback 消费后删除） */
export const OAUTH_SESSION_KEY = 'alexgo_oauth_session';

/** 登录页与 /callback 之间传递的 PKCE 上下文 */
export interface OAuthSession {
  verifier: string;
  state: string;
  /** 登录前目标地址（守卫 encodeURIComponent 后的 query.redirect，可为空串） */
  redirect: string;
}

export interface AuthorizeLoginParams {
  account_type?: string;
  client_id: string;
  code_challenge: string;
  code_challenge_method: 'S256';
  password: string;
  redirect_uri: string;
  response_type: 'code';
  state: string;
  username: string;
}

export interface OAuthTokenResponse {
  access_token: string;
  expires_in: number;
  refresh_token?: string;
  token_type: string;
}

/** 协议端点失败：HTTP 状态 + RFC 6749 信封 body */
export class OAuthHttpError extends Error {
  readonly body: null | Record<string, any>;

  constructor(status: number, body: null | Record<string, any>) {
    super(body?.error_description || body?.error || `HTTP ${status}`);
    this.name = 'OAuthHttpError';
    this.body = body;
  }
}

async function readBody(resp: Response): Promise<null | Record<string, any>> {
  try {
    return (await resp.json()) as null | Record<string, any>;
  } catch {
    return null;
  }
}

/**
 * POST /api/oauth2/authorize/login——成功 302 → redirect_uri?code&state，
 * fetch 跟随后返回最终 URL（/callback?code&state），由调用方提取并比对 state。
 */
export async function authorizeLoginApi(
  params: AuthorizeLoginParams,
): Promise<string> {
  const resp = await fetch(`${apiURL}/oauth2/authorize/login`, {
    body: JSON.stringify(params),
    headers: { 'Content-Type': 'application/json' },
    method: 'POST',
    redirect: 'follow',
  });
  if (!resp.ok) {
    throw new OAuthHttpError(resp.status, await readBody(resp));
  }
  return resp.url;
}

/** POST /api/oauth2/token——grant_type=authorization_code 换 token。 */
export async function exchangeTokenApi(params: {
  code: string;
  code_verifier: string;
  client_id: string;
  redirect_uri: string;
}): Promise<OAuthTokenResponse> {
  const resp = await fetch(`${apiURL}/oauth2/token`, {
    body: JSON.stringify({ grant_type: 'authorization_code', ...params }),
    headers: { 'Content-Type': 'application/json' },
    method: 'POST',
  });
  const body = await readBody(resp);
  if (!resp.ok) {
    throw new OAuthHttpError(resp.status, body);
  }
  return body as unknown as OAuthTokenResponse;
}
