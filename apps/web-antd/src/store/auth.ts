import type { Recordable, UserInfo } from '@vben/types';

import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { LOGIN_PATH } from '@vben/constants';
import { resetAllStores, useAccessStore, useUserStore } from '@vben/stores';

import { notification } from 'ant-design-vue';
import { defineStore } from 'pinia';

import { getUserInfoApi, logoutApi } from '#/api';
import {
  authorizeLoginApi,
  OAuthHttpError,
  OAUTH_SESSION_KEY,
  type OAuthSession,
} from '#/api/core/oauth2';
import { deriveChallenge, randomUrlSafe } from '#/utils/pkce';

export const useAuthStore = defineStore('auth', () => {
  const accessStore = useAccessStore();
  const userStore = useUserStore();
  const router = useRouter();

  const loginLoading = ref(false);

  /**
   * 异步处理登录操作（SSO：OAuth2 授权码 + PKCE，spec §9 D15）
   * 生成 verifier/state/S256 → 存 sessionStorage → POST /oauth2/authorize/login
   * （redirect:'follow'，从 resp.url 提取 code 并比对 state）→ 转 /callback 完成
   * code→token、用户信息与权限码加载、登录后跳转。
   * _onSuccess 保留签名但 SSO 下不再调用（导航归 /callback；唯一调用方
   * login.vue 只传表单值）。
   * @param params 登录表单数据（只消费 username/password）
   */
  async function authLogin(
    params: Recordable<any>,
    _onSuccess?: () => Promise<void> | void,
  ) {
    let userInfo: null | UserInfo = null;
    try {
      loginLoading.value = true;

      const verifier = randomUrlSafe(32);
      const state = randomUrlSafe(16);
      const codeChallenge = await deriveChallenge(verifier);
      const session: OAuthSession = {
        verifier,
        state,
        redirect: String(router.currentRoute.value.query.redirect ?? ''),
      };
      sessionStorage.setItem(OAUTH_SESSION_KEY, JSON.stringify(session));

      const finalUrl = await authorizeLoginApi({
        account_type: 'system',
        client_id: 'alexgo-admin',
        code_challenge: codeChallenge,
        code_challenge_method: 'S256',
        password: params.password,
        redirect_uri: `${window.location.origin}/callback`,
        response_type: 'code',
        state,
        username: params.username,
      });

      // 302 已被 fetch 跟随到 /callback?code&state——提取后回放给路由
      const callbackUrl = new URL(finalUrl);
      const code = callbackUrl.searchParams.get('code');
      const returnedState = callbackUrl.searchParams.get('state');
      if (!code || returnedState !== state) {
        throw new Error('授权回调校验失败（缺 code 或 state 不匹配）');
      }

      await router.push({
        path: '/callback',
        query: { code, state: returnedState },
      });
    } catch (error: any) {
      sessionStorage.removeItem(OAUTH_SESSION_KEY);
      notification.error({
        description:
          error instanceof OAuthHttpError
            ? error.message
            : error?.message || '请检查账号密码后重试',
        duration: 3,
        message: '登录失败',
      });
    } finally {
      loginLoading.value = false;
    }

    return { userInfo };
  }

  async function logout(redirect: boolean = true) {
    try {
      await logoutApi();
    } catch {
      // 不做任何处理
    }
    resetAllStores();
    accessStore.setLoginExpired(false);

    // 回登录页带上当前路由地址
    await router.replace({
      path: LOGIN_PATH,
      query: redirect
        ? {
            redirect: encodeURIComponent(router.currentRoute.value.fullPath),
          }
        : {},
    });
  }

  async function fetchUserInfo() {
    const userInfo = await getUserInfoApi();
    userStore.setUserInfo(userInfo);
    return userInfo;
  }

  function $reset() {
    loginLoading.value = false;
  }

  return {
    $reset,
    authLogin,
    fetchUserInfo,
    loginLoading,
    logout,
  };
});
