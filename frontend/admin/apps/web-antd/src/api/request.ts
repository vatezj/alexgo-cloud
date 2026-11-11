/**
 * 该文件可自行根据业务逻辑进行调整
 */
import type { RequestClientOptions } from '@vben/request';

import { useAppConfig } from '@vben/hooks';
import { preferences } from '@vben/preferences';
import {
  authenticateResponseInterceptor,
  defaultResponseInterceptor,
  errorMessageResponseInterceptor,
  RequestClient,
} from '@vben/request';
import { useAccessStore } from '@vben/stores';

import { message } from 'ant-design-vue';

import { useAuthStore } from '#/store';

import { blobErrorDetail } from './blob-error';
import { refreshTokenApi } from './core';

const { apiURL } = useAppConfig(import.meta.env, import.meta.env.PROD);

function createRequestClient(baseURL: string, options?: RequestClientOptions) {
  const client = new RequestClient({
    ...options,
    baseURL,
  });

  /**
   * 重新认证逻辑
   */
  async function doReAuthenticate() {
    console.warn('Access token or refresh token is invalid or expired. ');
    const accessStore = useAccessStore();
    const authStore = useAuthStore();
    accessStore.setAccessToken(null);
    if (
      preferences.app.loginExpiredMode === 'modal' &&
      accessStore.isAccessChecked
    ) {
      accessStore.setLoginExpired(true);
    } else {
      await authStore.logout();
    }
  }

  /**
   * 刷新token逻辑
   */
  async function doRefreshToken() {
    const accessStore = useAccessStore();
    const resp = await refreshTokenApi();
    // 裸 client（responseReturn:'raw'）→ resp=AxiosResponse：resp.data 是统一
    // 信封体 {code,msg,data}，新令牌须再下钻 .data.data.token（统一信封迁移到此
    // 的读法修正——旧形态 resp.data 直为 token 字符串）。
    const newToken = resp.data.data.token;
    accessStore.setAccessToken(newToken);
    return newToken;
  }

  function formatToken(token: null | string) {
    return token ? `Bearer ${token}` : null;
  }

  // 请求头处理
  client.addRequestInterceptor({
    fulfilled: async (config) => {
      const accessStore = useAccessStore();

      config.headers.Authorization = formatToken(accessStore.accessToken);
      config.headers['Accept-Language'] = preferences.app.locale;
      return config;
    },
  });

  // 处理返回的响应数据格式
  client.addResponseInterceptor(
    defaultResponseInterceptor({
      codeField: 'code',
      // alexGo：统一信封 {code,msg,data}（pkg/resp 唯一出站出口，code 恒在）——
      //   code===0 成功 → 解出 data（动作类 data:null → null）
      //   非 2xx 或 2xx 但 code!==0 走 rejected（successCode 不参与）
      //   → authenticate → errorMessage（优先弹后端 msg 文案）。
      dataField: (response: any) =>
        response &&
        typeof response === 'object' &&
        'data' in response &&
        response.data !== undefined
          ? response.data
          : response,
      successCode: (code: any) => code === 0,
    }),
  );

  // token过期的处理
  client.addResponseInterceptor(
    authenticateResponseInterceptor({
      client,
      doReAuthenticate,
      doRefreshToken,
      enableRefreshToken: preferences.app.enableRefreshToken,
      formatToken,
    }),
  );

  // 通用的错误处理,如果没有进入上面的错误处理逻辑，就会进入这里
  client.addResponseInterceptor(
    errorMessageResponseInterceptor((msg: string, error) => {
      // 这里可以根据业务进行定制,你可以拿到 error 内的信息进行定制化处理，根据不同的 code 做不同的提示，而不是直接使用 message.error 提示 msg
      // 统一信封失败体 {code:<5位>,msg:<中文>,data:null}——文案在 msg 字段
      // （message/error 为旧形态兼容兜底）。
      const responseData = error?.response?.data ?? {};
      // download 的 responseType='blob'，400 错误体也是 Blob——先转文本
      // 再取 msg 字段，否则只显状态码通用文案（M3 终审跟进票③）。
      // 拦截器回调不被 await（preset-interceptors 同步调用），这里 fire-and-forget。
      if (responseData instanceof Blob) {
        void blobErrorDetail(responseData)
          .then((detail: string) => {
            message.error(detail || msg);
          })
          .catch(() => message.error(msg));
        return;
      }
      const errorMessage =
        responseData?.msg ?? responseData?.message ?? responseData?.error ?? '';
      // 如果没有错误信息，则会根据状态码进行提示
      message.error(errorMessage || msg);
    }),
  );

  return client;
}

export const requestClient = createRequestClient(apiURL, {
  responseReturn: 'data',
});

export const baseRequestClient = new RequestClient({ baseURL: apiURL });
