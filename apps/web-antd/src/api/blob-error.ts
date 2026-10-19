// download 的 400 错误体是 Blob——转文本解析信封 msg，否则只显 axios 通用
// 文案（P1-2 RF④：export 30013 要弹「导出行数超过上限」而非乱码/通用错）。
export async function blobErrorDetail(blob: Blob): Promise<string> {
  try {
    const body = JSON.parse(await blob.text()) as {
      error?: string;
      message?: string;
      msg?: string;
    };
    return body?.msg ?? body?.message ?? body?.error ?? '';
  } catch {
    return '';
  }
}
