/**
 * PKCE（RFC 7636）工具——SSO 授权码流前端侧。
 * verifier/challenge 一律 base64url（无填充）；S256 用 Web Crypto。
 */

/** 字节序列 → base64url（去填充） */
function base64UrlEncode(bytes: ArrayBuffer | Uint8Array): string {
  const view = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  let binary = '';
  for (const byte of view) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}

/** N 字节随机 → base64url；默认 32B → 43 字符（RFC 7636 verifier 43–128） */
export function randomUrlSafe(byteLength: number = 32): string {
  const buf = new Uint8Array(byteLength);
  crypto.getRandomValues(buf);
  return base64UrlEncode(buf);
}

/** S256 挑战：base64url(SHA-256(verifier)) */
export async function deriveChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest(
    'SHA-256',
    new TextEncoder().encode(verifier),
  );
  return base64UrlEncode(digest);
}
