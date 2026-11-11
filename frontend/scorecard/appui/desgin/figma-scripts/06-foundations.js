// 06 🎨 Foundations 设计规范页 — fileKey: dHMgzmrrWJ8ztjWytPXdIW, 页 🎨 Foundations (14:72)
const page = await figma.getNodeByIdAsync('14:72');
await figma.setCurrentPageAsync(page);
await Promise.all([
  figma.loadFontAsync({ family: 'Oswald', style: 'Bold' }),
  figma.loadFontAsync({ family: 'Oswald', style: 'SemiBold' }),
  figma.loadFontAsync({ family: 'Oswald', style: 'Medium' }),
  figma.loadFontAsync({ family: 'Noto Sans SC', style: 'Bold' }),
  figma.loadFontAsync({ family: 'Noto Sans SC', style: 'Medium' }),
  figma.loadFontAsync({ family: 'Noto Sans SC', style: 'Regular' })
]);
const allVars = await figma.variables.getLocalVariablesAsync();
const byName = Object.fromEntries(allVars.map(v => [v.name, v]));
const V = (n) => { const v = byName[n]; if (!v) throw new Error('missing var ' + n); return v; };
const bindFill = (node, v) => { node.fills = [figma.variables.setBoundVariableForPaint({ type: 'SOLID', color: { r: 0, g: 0, b: 0 } }, 'color', v)]; };
const bindRadius = (node, v) => { node.setBoundVariable('topLeftRadius', v); node.setBoundVariable('topRightRadius', v); node.setBoundVariable('bottomLeftRadius', v); node.setBoundVariable('bottomRightRadius', v); };
const TS = {
  hero: 'S:5f7870f48f5e9cc6174ddc15354cca882c428498,', xl: 'S:01f181b5130353798d14cf64c80ae93dc4e34b7e,',
  lg: 'S:4c79652ae346b8e0d32fa769a280ee025efc4b9d,', md: 'S:8b7eebf40b1783a983ea6005b563bb2e0e3658e3,',
  sm: 'S:f0cfc3d3c829ed1858dbd1b23274271e9852d537,',
  h1: 'S:51c7c69b43b3675ff5878d9897cfeac47cf4e243,', h2: 'S:e724ff93ae56f28ce2c7d9081ef81386b562f6d7,',
  body: 'S:07b03ff2335b2c026337a0685835cce71af9dcd5,', label: 'S:3e082326e7ed79e94f11756f1118fd7f85e4b87a,',
  caption: 'S:ebe8931ec6c3010f156a6fd75958f3addc62baf8,'
};
const al = (dir, props = {}) => { const f = figma.createAutoLayout(dir, props); f.fills = []; return f; };
const mkText = (chars, style, colorVar) => { const t = figma.createText(); t.textStyleId = TS[style]; t.characters = chars; bindFill(t, V(colorVar)); return t; };

const dup = page.children.find(c => c.name === '📐 Design Rules');
if (dup) dup.remove();

const root = al('VERTICAL', { name: '📐 Design Rules', itemSpacing: 40 });
root.paddingLeft = 60; root.paddingRight = 60; root.paddingTop = 60; root.paddingBottom = 60;
bindFill(root, V('color/bg/page'));
root.resize(1200, 400);
root.counterAxisSizingMode = 'FIXED';
root.primaryAxisSizingMode = 'AUTO';
page.appendChild(root);
root.x = 0; root.y = 0;

// 标题
const head = al('VERTICAL', { name: 'Intro', itemSpacing: 8 });
root.appendChild(head); head.layoutSizingHorizontal = 'FILL';
head.appendChild(mkText('打牌记账 · 设计规范', 'h1', 'color/score/win'));
head.appendChild(mkText('牌桌绒布 + 黄铜筹码。金额是主角；赢金、输蓝、平米白；每个金额必带 +/− 符号。', 'body', 'color/text/dim'));

// ---- 配色 ----
const colorSec = al('VERTICAL', { name: 'Section/配色', itemSpacing: 16 });
root.appendChild(colorSec); colorSec.layoutSizingHorizontal = 'FILL';
colorSec.appendChild(mkText('配色 Color', 'h2', 'color/text/primary'));
const swatchRows = [
  [['felt/900', '#0B2B21', 'felt/900'], ['felt/700', '#123D30', 'felt/700'], ['felt/500', '#1A5040', 'felt/500'], ['felt/300', '#2E6A55', 'felt/300']],
  [['gold/500', '#F2B544', 'gold/500'], ['gold/600', '#D99E2E', 'gold/600'], ['red/500', '#E4593E', 'red/500'], ['blue/500', '#5B8DEF', 'blue/500']],
  [['blue/600', '#4578D6', 'blue/600'], ['paper/500', '#F4EDDC', 'paper/500'], ['cream/100', '#F2EDE1', 'cream/100'], ['sage/400', '#8FA89C', 'sage/400']]
];
for (const swRow of swatchRows) {
  const row = al('HORIZONTAL', { name: 'SwatchRow', itemSpacing: 24 });
  colorSec.appendChild(row);
  for (const [name, hex, varName] of swRow) {
    const sw = al('VERTICAL', { name: 'Swatch/' + name, itemSpacing: 8 });
    const chip = figma.createFrame();
    chip.name = 'Color';
    chip.resize(200, 72);
    bindFill(chip, V(varName));
    bindRadius(chip, V('radius/md'));
    chip.strokes = [{ type: 'SOLID', color: { r: 1, g: 1, b: 1 }, opacity: 0.12 }];
    chip.strokeWeight = 1;
    sw.appendChild(chip);
    sw.appendChild(mkText(name, 'label', 'color/text/primary'));
    sw.appendChild(mkText(hex, 'caption', 'color/text/dim'));
    row.appendChild(sw);
  }
}

// ---- 字体 ----
const typeSec = al('VERTICAL', { name: 'Section/字体', itemSpacing: 14 });
root.appendChild(typeSec); typeSec.layoutSizingHorizontal = 'FILL';
typeSec.appendChild(mkText('字体 Typography · Oswald 金额 + Noto Sans SC', 'h2', 'color/text/primary'));
const samples = [
  ['score/hero  72', '+1,240', 'hero', 'color/score/win'],
  ['score/xl  48', '+360', 'xl', 'color/score/lose'],
  ['score/lg  32', '−80', 'lg', 'color/score/win'],
  ['score/md  24', '0', 'md', 'color/score/neutral'],
  ['h1  26 Bold', '页面标题 H1', 'h1', 'color/text/primary'],
  ['h2  18 Medium', '卡片标题 H2', 'h2', 'color/text/primary'],
  ['body  15 Regular', '正文：每一笔都有据可查', 'body', 'color/text/primary'],
  ['label  13 Medium', '标签 LABEL', 'label', 'color/text/dim'],
  ['caption  11', '辅助说明文字 11pt', 'caption', 'color/text/dim']
];
for (const [meta, sample, style, cv] of samples) {
  const row = al('HORIZONTAL', { name: 'Type/' + meta, itemSpacing: 24 });
  row.counterAxisAlignItems = 'CENTER';
  typeSec.appendChild(row); row.layoutSizingHorizontal = 'FILL';
  const m = mkText(meta, 'caption', 'color/text/dim');
  m.textAutoResize = 'HEIGHT';
  row.appendChild(m);
  m.layoutSizingHorizontal = 'FIXED';
  m.resize(160, m.height);
  row.appendChild(mkText(sample, style, cv));
}

// ---- 圆角 ----
const rSec = al('VERTICAL', { name: 'Section/圆角', itemSpacing: 14 });
root.appendChild(rSec); rSec.layoutSizingHorizontal = 'FILL';
rSec.appendChild(mkText('圆角 Radius', 'h2', 'color/text/primary'));
const rRow = al('HORIZONTAL', { name: 'Row', itemSpacing: 24 });
rSec.appendChild(rRow);
for (const r of ['sm', 'md', 'lg', 'xl', 'full']) {
  const col = al('VERTICAL', { name: 'Radius/' + r, itemSpacing: 8 });
  col.counterAxisAlignItems = 'CENTER';
  const sq = figma.createFrame();
  sq.resize(96, 96);
  bindFill(sq, V('color/bg/surface'));
  sq.strokes = [figma.variables.setBoundVariableForPaint({ type: 'SOLID', color: { r: 0, g: 0, b: 0 } }, 'color', V('color/border/gold'))];
  sq.strokeWeight = 1.5;
  bindRadius(sq, V('radius/' + r));
  col.appendChild(sq);
  col.appendChild(mkText(r, 'label', 'color/text/dim'));
  rRow.appendChild(col);
}

// ---- 间距 ----
const sSec = al('VERTICAL', { name: 'Section/间距', itemSpacing: 14 });
root.appendChild(sSec); sSec.layoutSizingHorizontal = 'FILL';
sSec.appendChild(mkText('间距 Spacing（4 制）', 'h2', 'color/text/primary'));
const sRow = al('HORIZONTAL', { name: 'Row', itemSpacing: 20 });
sRow.counterAxisAlignItems = 'MAX';
sSec.appendChild(sRow);
for (const s of [4, 8, 12, 16, 20, 24, 32, 40]) {
  const col = al('VERTICAL', { name: 'Space/' + s, itemSpacing: 6 });
  col.counterAxisAlignItems = 'CENTER';
  const bar = figma.createFrame();
  bar.resize(s, 48);
  bindFill(bar, V('color/score/win'));
  bindRadius(bar, V('radius/sm'));
  col.appendChild(bar);
  col.appendChild(mkText(String(s), 'caption', 'color/text/dim'));
  sRow.appendChild(col);
}

// ---- 规则 ----
const ruleSec = al('VERTICAL', { name: 'Section/规则', itemSpacing: 10 });
root.appendChild(ruleSec); ruleSec.layoutSizingHorizontal = 'FILL';
ruleSec.appendChild(mkText('硬性规则', 'h2', 'color/score/win'));
const rules = [
  '1. 每个金额必须带 + / − / 0 符号，颜色只是增强（防色弱）',
  '2. 主操作全部位于屏幕下 1/3 拇指热区，触达区 ≥ 44',
  '3. 赢 = 金 gold/500，输 = 蓝 blue/500，平 = 米白 paper/500，不引入第四色',
  '4. 一屏只有一个视觉主角；金额永远是视觉权重最大的元素',
  '5. 结构容器 fills = []（createAutoLayout 默认白底是坑）',
  '6. 变量必须设 scopes 与 WEB code syntax（var(--…) 包裹）',
  '7. 组件变体命名 Property=Value；页面三页制：Screens / Components / Foundations'
];
for (const r of rules) ruleSec.appendChild(mkText(r, 'body', 'color/text/primary'));

const all = root.findAll(() => true).map(n => n.id);
return { rulesPageFrameId: root.id, createdNodeIds: [root.id, ...all], count: all.length + 1, w: root.width, h: root.height };