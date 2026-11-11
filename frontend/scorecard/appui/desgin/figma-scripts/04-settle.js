// 04 结算页 — fileKey: dHMgzmrrWJ8ztjWytPXdIW, 页 📱 Screens (0:1)
const page = await figma.getNodeByIdAsync('0:1');
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
const TS = {
  hero: 'S:5f7870f48f5e9cc6174ddc15354cca882c428498,',
  h1: 'S:51c7c69b43b3675ff5878d9897cfeac47cf4e243,', h2: 'S:e724ff93ae56f28ce2c7d9081ef81386b562f6d7,',
  body: 'S:07b03ff2335b2c026337a0685835cce71af9dcd5,', label: 'S:3e082326e7ed79e94f11756f1118fd7f85e4b87a,',
  caption: 'S:ebe8931ec6c3010f156a6fd75958f3addc62baf8,'
};
const al = (dir, props = {}) => { const f = figma.createAutoLayout(dir, props); f.fills = []; return f; };
const mkText = (chars, style, colorVar) => { const t = figma.createText(); t.textStyleId = TS[style]; t.characters = chars; bindFill(t, V(colorVar)); return t; };
const overrideText = (container, from, to) => {
  const t = container.findAllWithCriteria({ types: ['TEXT'] }).find(n => n.characters === from);
  if (!t) throw new Error('override target not found: ' + from);
  t.characters = to;
};

const dup = page.children.find(c => c.name === '04 结算');
if (dup) dup.remove();
let maxX = 0;
for (const c of page.children) maxX = Math.max(maxX, c.x + c.width);

const root = figma.createFrame();
root.name = '04 结算';
root.layoutMode = 'VERTICAL';
root.resize(390, 844);
root.primaryAxisSizingMode = 'FIXED';
root.counterAxisSizingMode = 'FIXED';
bindFill(root, V('color/bg/page'));
root.clipsContent = true;
page.appendChild(root);
root.x = maxX + 120; root.y = 0;

const sb = al('HORIZONTAL', { name: 'StatusBar', itemSpacing: 8 });
sb.paddingLeft = 24; sb.paddingRight = 24; sb.paddingTop = 14; sb.paddingBottom = 6;
sb.primaryAxisAlignItems = 'SPACE_BETWEEN'; sb.counterAxisAlignItems = 'CENTER';
root.appendChild(sb); sb.layoutSizingHorizontal = 'FILL';
sb.appendChild(mkText('23:05', 'label', 'color/text/primary'));
sb.appendChild(mkText('5G ▮100%', 'caption', 'color/text/dim'));

const hd = al('HORIZONTAL', { name: 'Header', itemSpacing: 12 });
hd.paddingLeft = 20; hd.paddingRight = 20; hd.paddingTop = 12; hd.paddingBottom = 4;
hd.counterAxisAlignItems = 'CENTER';
root.appendChild(hd); hd.layoutSizingHorizontal = 'FILL';
hd.appendChild(mkText('‹', 'h1', 'color/text/primary'));
hd.appendChild(mkText('结算', 'h1', 'color/text/primary'));

// Hero：我的净成绩（本屏唯一主角）
const hero = al('VERTICAL', { name: 'Hero/我的净成绩', itemSpacing: 6 });
hero.paddingTop = 24; hero.paddingBottom = 20;
hero.counterAxisAlignItems = 'CENTER';
root.appendChild(hero); hero.layoutSizingHorizontal = 'FILL';
hero.appendChild(mkText('我的净成绩', 'label', 'color/text/dim'));
const heroNum = mkText('−160', 'hero', 'color/score/lose');
hero.appendChild(heroNum);
hero.appendChild(mkText('6 局 · 底分 ¥10 · 麻将', 'caption', 'color/text/dim'));

// 结算卡列表
const list = al('VERTICAL', { name: 'Section/结算卡', itemSpacing: 10 });
list.paddingLeft = 20; list.paddingRight = 20; list.paddingTop = 8; list.paddingBottom = 8;
root.appendChild(list); list.layoutSizingHorizontal = 'FILL';

const winComp = await figma.getNodeByIdAsync('17:8');   // 老王 +360 默认
const loseComp = await figma.getNodeByIdAsync('17:13'); // 阿强 −240 默认

const s1 = winComp.createInstance(); list.appendChild(s1); s1.layoutSizingHorizontal = 'FILL';
const s2 = loseComp.createInstance(); list.appendChild(s2); s2.layoutSizingHorizontal = 'FILL';
overrideText(s2, '阿强', '李姐');
overrideText(s2, '净输 · 6 局', '净输 · 6 局');
overrideText(s2, '−240', '−80');
const s3 = loseComp.createInstance(); list.appendChild(s3); s3.layoutSizingHorizontal = 'FILL';
overrideText(s3, '阿强', '我');
overrideText(s3, '−240', '−160');
const s4 = loseComp.createInstance(); list.appendChild(s4); s4.layoutSizingHorizontal = 'FILL';
overrideText(s4, '−240', '−120');
// 四人合计：+360 −80 −160 −120 = 0（筹码守恒）

const sp = figma.createFrame();
sp.name = 'Spacer'; sp.fills = []; sp.resize(10, 10);
root.appendChild(sp); sp.layoutGrow = 1; sp.layoutSizingHorizontal = 'FILL';

// CTA
const cta = al('VERTICAL', { name: 'Section/CTA', itemSpacing: 12 });
cta.paddingLeft = 20; cta.paddingRight = 20; cta.paddingTop = 8; cta.paddingBottom = 28;
root.appendChild(cta); cta.layoutSizingHorizontal = 'FILL';
const edit = (await figma.getNodeByIdAsync('15:21')).createInstance();
cta.appendChild(edit); edit.layoutSizingHorizontal = 'FILL';
overrideText(edit, '撤销上一步', '编辑明细');
const save = (await figma.getNodeByIdAsync('15:19')).createInstance();
cta.appendChild(save); save.layoutSizingHorizontal = 'FILL';
overrideText(save, '开始牌局', '保存战绩');

const all = root.findAll(() => true).map(n => n.id);
return { screenId: root.id, x: root.x, createdNodeIds: [root.id, ...all], count: all.length + 1 };