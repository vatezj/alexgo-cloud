<script setup lang="ts">
import { onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { LOGIN_PATH } from '@vben/constants';
import { preferences } from '@vben/preferences';
import { useAccessStore, useUserStore } from '@vben/stores';

import { notification } from 'ant-design-vue';

import { getAccessCodesApi, getUserInfoApi } from '#/api';
import type { OAuthSession } from '#/api/core/oauth2';
import {
  exchangeTokenApi,
  OAuthHttpError,
  OAUTH_SESSION_KEY,
} from '#/api/core/oauth2';
import { $t } from '#/locales';

const route = useRoute();
const router = useRouter();
const accessStore = useAccessStore();
const userStore = useUserStore();

/** 统一失败出口：清 PKCE 会话 → 中文提示 → 回登录页（登录页守卫会回带 redirect） */
function failLogin(reason: string) {
  sessionStorage.removeItem(OAUTH_SESSION_KEY);
  notification.error({
    description: reason,
    duration: 3,
    message: '登录失败',
  });
  router.replace(LOGIN_PATH);
}

onMounted(async () => {
  const code = typeof route.query.code === 'string' ? route.query.code : '';
  const state =
    typeof route.query.state === 'string' ? route.query.state : '';
  if (!code || !state) {
    failLogin('回调参数缺失');
    return;
  }

  let session: null | OAuthSession = null;
  try {
    const raw = sessionStorage.getItem(OAUTH_SESSION_KEY);
    session = raw ? (JSON.parse(raw) as OAuthSession) : null;
  } catch {
    session = null;
  }
  if (!session) {
    failLogin('登录会话已失效，请重新登录');
    return;
  }
  if (session.state !== state) {
    failLogin('state 校验失败');
    return;
  }

  try {
    const token = await exchangeTokenApi({
      code,
      client_id: 'alexgo-admin',
      code_verifier: session.verifier,
      redirect_uri: `${window.location.origin}/callback`,
    });
    accessStore.setAccessToken(token.access_token);
    sessionStorage.removeItem(OAUTH_SESSION_KEY);

    const [userInfo, accessCodes] = await Promise.all([
      getUserInfoApi(),
      getAccessCodesApi(),
    ]);
    userStore.setUserInfo(userInfo);
    accessStore.setAccessCodes(accessCodes);

    if (accessStore.loginExpired) {
      accessStore.setLoginExpired(false);
      return;
    }

    const target =
      (session.redirect && decodeURIComponent(session.redirect)) ||
      userInfo.homePath ||
      preferences.app.defaultHomePath;
    await router.push(target);

    if (userInfo.realName) {
      notification.success({
        description: `${$t('authentication.loginSuccessDesc')}:${userInfo.realName}`,
        duration: 3,
        message: $t('authentication.loginSuccess'),
      });
    }
  } catch (error: any) {
    failLogin(
      error instanceof OAuthHttpError
        ? error.message
        : error?.message || '请稍后重试',
    );
  }
});
</script>

<template>
  <div class="flex h-screen items-center justify-center">正在完成登录…</div>
</template>
