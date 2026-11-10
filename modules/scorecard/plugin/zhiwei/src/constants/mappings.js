/** 奇门九宫 → 方位（中五宫无门，不参与推荐） */
export const QIMEN_PALACE_TO_DIRECTION = {
  1: '北', // 坎宫
  2: '西南', // 坤宫
  3: '东', // 震宫
  4: '东南', // 巽宫
  6: '西北', // 乾宫
  7: '西', // 兑宫
  8: '东北', // 艮宫
  9: '南', // 离宫
};

/** 奇门九宫序号 → 宫名（评分引擎按宫名查生门旺衰） */
export const QIMEN_PALACE_TO_NAME = {
  1: '坎宫',
  2: '坤宫',
  3: '震宫',
  4: '巽宫',
  5: '中五宫',
  6: '乾宫',
  7: '兑宫',
  8: '艮宫',
  9: '离宫',
};

/** 紫微地支 → 方位 */
export const ZIWEI_BRANCH_TO_DIRECTION = {
  子: '北',
  丑: '东北',
  寅: '东北',
  卯: '东',
  辰: '东南',
  巳: '东南',
  午: '南',
  未: '西南',
  申: '西南',
  酉: '西',
  戌: '西北',
  亥: '西北',
};

/** qimen-rs 英文枚举 → 中文（适配层，避免库字段渗透进评分逻辑） */
export const QIMEN_DOOR_TO_CHINESE = {
  xiu: '休门',
  si: '死门',
  shang: '伤门',
  du: '杜门',
  kai: '开门',
  jing: '惊门',
  sheng: '生门',
  scene: '景门',
};

export const QIMEN_STAR_TO_CHINESE = {
  tian_peng: '天蓬',
  tian_rui: '天芮',
  tian_chong: '天冲',
  tian_fu: '天辅',
  tian_qin: '天禽',
  tian_xin: '天心',
  tian_zhu: '天柱',
  tian_ren: '天任',
  tian_ying: '天英',
};

export const QIMEN_DEITY_TO_CHINESE = {
  zhi_fu: '值符',
  teng_she: '腾蛇',
  tai_yin: '太阴',
  liu_he: '六合',
  bai_hu: '白虎',
  xuan_wu: '玄武',
  jiu_di: '九地',
  jiu_tian: '九天',
};
