/**
 * 趣味方位参考 · 锁定文案常量（PRD desgin/xuanzuo.md §5 / 设计稿 preview/*.html）
 *
 * - 免责与提示句为合规锁定语，**逐字**使用，禁止改写、拼接或省略；
 * - 六页共享的句子集中在此（DRY），页面模板只 import；
 * - FORBIDDEN_WORDS 供自测检索（tasks 5.1）：模块可见文案 0 命中；
 *   「收支 / 投资」仅允许出现在下方免责的否定语境中。
 */

/** 模块底部免责 · 第 1 句（六页逐字） */
export const DISCLAIMER_1
  = '提示：本趣味方位参考仅传统民俗娱乐，无科学依据，和记账理财无关，不构成任何决策建议。棋牌请理性游玩，严禁赌博。'

/** 模块底部免责 · 第 2 句（六页逐字；PRD 原为座位模拟器备注，规格化为全模块固定） */
export const DISCLAIMER_2 = '本工具仅民俗趣味参考，和资金收支、投资没有任何关联。'

/** 关于页免责全文（PRD §5 逐字，四段） */
export const ABOUT_TITLE = '免责说明'

export const ABOUT_PARAGRAPHS: readonly string[] = [
  '趣味方位参考为本记账小程序内置的休闲彩蛋工具，仅为传统民俗文化趣味参考，无科学依据；本功能和记账、理财、收支预测完全无关，不构成任何资金、博弈决策建议。',
  '所有五行方位结论存在流派差异，不作唯一标准。棋牌娱乐请勿参与赌博。',
  '本模块录入的生辰档案仅保存在当前设备本地；排盘计算经后端趣味服务完成，出生年月日时间与性别仅用于本次计算，不保存、不与您的记账数据或记账业务关联。',
  '您可以随时一键清除本工具全部档案数据，清除不会影响您的账本记录。',
]

/** 模块首页前置提示条（逐字） */
export const HOME_LEAD_TIP
  = '本为休闲趣味小工具，和记账、理财功能相互独立，仅供娱乐。'

/** 关于页危险按钮与说明 */
export const CLEAR_BUTTON = '清除本工具全部档案'
export const CLEAR_HINT = '只清方位工具的生辰档案，账本数据不动'
export const CLEAR_DONE_TOAST = '已清除，账本数据不受影响'

/** 数据用途小字（2026-11-06 如实化：档案本机 + 排盘即算即走，design D8） */
export const LOCAL_ONLY = '档案仅存本机 · 排盘即算即走'
export const LOCAL_ONLY_LIST = '档案仅存本机 · 不关联账号'

/** 模块首页脚注 */
export const HOME_FOOT = '以上工具均为民俗趣味参考，数据只存本机'
export const HOME_LOCAL_DOT = '生辰档案仅存本机，不上传'

/** 【我的】页趣味分组脚注（用户可见层；开关行为见 useFunCompassConfig） */
export const MINE_GROUP_FOOT = '民俗趣味 · 档案只存本机'

/** 档案空态行动指引（空状态写行动指引，不写道歉文案） */
export const ARCHIVE_EMPTY_HINT = '还没有档案，填一份生辰就能排盘求座'

/** 黑名单词（PRD §7）：模块可见文案检索期望 0 命中 */
export const FORBIDDEN_WORDS = [
  '财运',
  '收益',
  '进账',
  '生意',
  '回款',
  '财源',
  '利于赚钱',
  '旺收入',
  '赚钱',
] as const
