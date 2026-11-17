/** 墨金 T1 主题 token — 源：modules/scorecard/appui/uniapp/src/theme/scorecard.scss */
export const T = {
  // 墨面
  ink900: "#14120F",
  ink700: "#1E1B17",
  ink600: "#2A2722",
  // 桌布
  felt: "#12261E",
  feltHi: "#17362B",
  feltDeep: "#0D1D17",
  // 牌面 / chrome
  tile: "#F2EDE3",
  tileShade: "#E5DCC9",
  tileEmboss: "#B9B0A0",
  jet: "#2A2722",
  brass: "#C9A24B",
  brassHi: "#E0BC6A",
  brassLo: "#8F6F2E",
  brassA50: "rgba(201, 162, 75, 0.5)",
  jade: "#3E7D5A",
  // 赢 / 输（赢色与 chrome 解耦）
  win: "#C9A24B",
  lose: "#6B8BD0",
  // 合规锁定红
  complyBg: "rgba(210, 59, 34, 0.13)",
  complyBorder: "rgba(210, 59, 34, 0.45)",
  complyStrong: "#F0906F",
  // 文字
  onInk: "#EDE6D8",
  dim: "#9B917F",
  onInkA04: "rgba(237, 230, 216, 0.04)",
  onInkA28: "rgba(237, 230, 216, 0.28)",
  // 几何 / 阴影
  spot:
    "radial-gradient(130% 75% at 50% -8%, rgba(201, 162, 75, 0.16), transparent 55%)",
  shadowTile: "0 2px 0 rgba(0, 0, 0, 0.55), 0 12px 26px rgba(0, 0, 0, 0.45)",
  shadowChip: "0 3px 0 rgba(0, 0, 0, 0.45), 0 8px 14px rgba(0, 0, 0, 0.35)",
} as const;

export const SERIF = '"Noto Serif SC", serif';
export const SANS = '"Noto Sans SC", sans-serif';
export const NUM = '"Oswald", sans-serif';
