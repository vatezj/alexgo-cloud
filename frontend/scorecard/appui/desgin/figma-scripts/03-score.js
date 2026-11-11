// 03 记分页（核心）— fileKey: dHMgzmrrWJ8ztjWytPXdIW, 页 📱 Screens (0:1)
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
const bindRadius = (node, v) => { node.setBoundVariable('topLeftRadius', v); node.setBoundVariable('topRightRadius', v); node.setBoundVariable('bottomLeftRadius', v); node.setBoundVariable('bottomRightRadius', v); };
const TS = {
  xl: 'S:01f181b5130353798d14cf64c80ae93dc4e34b7e,', sm: 'S:f0cfc3d3c829ed1858dbd1b23274271e9852d537,',
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
  return t.id;
};

const dup = page.children.find(c => c.name === '03 记分页');
if (dup) dup.remove();
let maxX = 0;
for (const c of page.children) maxX = Math.max(maxX, c.x + c.width);

const root = figma.createFrame();
root.name = '03 记分页';
root.layoutMode = 'VERTICAL';
root.resize(390, 844);
root.primaryAxisSizingMode = 'FIXED';
root.counterAxisSizingMode = 'FIXED';
bindFill(root, V('color/bg/page'));
root.clipsContent = true;
page.appendChild(root);
root.x = maxX + 120; root.y = 0;

// StatusBar
const sb = al('HORIZONTAL', { name: 'StatusBar', itemSpacing: 8 });
sb.paddingLeft = 24; sb.paddingRight = 24; sb.paddingTop = 14; sb.paddingBottom = 6;
sb.primaryAxisAlignItems = 'SPACE_BETWEEN'; sb.counterAxisAlignItems = 'CENTER';
root.appendChild(sb); sb.layoutSizingHorizontal = 'FILL';
sb.appendChild(mkText('21:36', 'label', 'color/text/primary'));
sb.appendChild(mkText('5G ▮100%', 'caption', 'color/text/dim'));

// Header: 标题 + 底分控件
const hd = al('HORIZONTAL', { name: 'Header', itemSpacing: 8 });
hd.paddingLeft = 20; hd.paddingRight = 20; hd.paddingTop = 8; hd.paddingBottom = 8;
hd.primaryAxisAlignItems = 'SPACE_BETWEEN'; hd.counterAxisAlignItems = 'CENTER';
root.appendChild(hd); hd.layoutSizingHorizontal = 'FILL';
const hCol = al('VERTICAL', { name: 'Title', itemSpacing: 2 });
hCol.appendChild(mkText('麻将 · 第 6 局', 'h2', 'color/text/primary'));
hCol.appendChild(mkText('老王家 · 4 人', 'caption', 'color/text/dim'));
hd.appendChild(hCol);
const bsc = (await figma.getNodeByIdAsync('15:11')).createInstance();
hd.appendChild(bsc);

// 玩家卡 ×4
const cards = al('VERTICAL', { name: 'Section/玩家卡', itemSpacing: 12 });
cards.paddingLeft = 20; cards.paddingRight = 20; cards.paddingTop = 8; cards.paddingBottom = 8;
root.appendChild(cards); cards.layoutSizingHorizontal = 'FILL';

const winComp = await figma.getNodeByIdAsync('16:2');   // 老王 +120 默认
const loseComp = await figma.getNodeByIdAsync('16:15'); // 李姐 −80 默认
const tieComp = await figma.getNodeByIdAsync('16:28');  // 阿强 0 默认

const c1 = winComp.createInstance(); cards.appendChild(c1); c1.layoutSizingHorizontal = 'FILL';
const c2 = loseComp.createInstance(); cards.appendChild(c2); c2.layoutSizingHorizontal = 'FILL';
const c3 = tieComp.createInstance(); cards.appendChild(c3); c3.layoutSizingHorizontal = 'FILL';
const c4 = loseComp.createInstance(); cards.appendChild(c4); c4.layoutSizingHorizontal = 'FILL';
// 覆盖第4张卡为「我 −40」
overrideText(c4, '李', '我');
overrideText(c4, '李姐', '我');
overrideText(c4, '−80', '−40');

// spacer
const sp = figma.createFrame();
sp.name = 'Spacer'; sp.fills = []; sp.resize(10, 10);
root.appendChild(sp); sp.layoutGrow = 1; sp.layoutSizingHorizontal = 'FILL';

// 底部操作条：撤销 + 结束
const bar = al('HORIZONTAL', { name: 'BottomBar', itemSpacing: 12 });
bar.paddingLeft = 20; bar.paddingRight = 20; bar.paddingTop = 8; bar.paddingBottom = 24;
root.appendChild(bar); bar.layoutSizingHorizontal = 'FILL';
const undo = (await figma.getNodeByIdAsync('15:21')).createInstance();
bar.appendChild(undo);
undo.resize(132, 56);
overrideText(undo, '撤销上一步', '撤销');
const endBtn = (await figma.getNodeByIdAsync('15:19')).createInstance();
bar.appendChild(endBtn); endBtn.layoutSizingHorizontal = 'FILL';
overrideText(endBtn, '开始牌局', '结束牌局');

// ===== 动效标注1：+20 加分浮标（绝对定位，压在「我」的分数上方）=====
const pill = al('HORIZONTAL', { name: 'Motion/加分浮标 +20' });
pill.paddingLeft = 14; pill.paddingRight = 14; pill.paddingTop = 6; pill.paddingBottom = 6;
bindFill(pill, V('color/score/win'));
bindRadius(pill, V('radius/full'));
pill.appendChild(mkText('+20', 'sm', 'color/text/on-gold'));
root.appendChild(pill);
pill.layoutPositioning = 'ABSOLUTE';
pill.x = 235; pill.y = 618;
pill.rotation = -8;
pill.effectStyleId = 'S:b8d4319c65d58c3d417669aae9a1495b960f4ef2,';

// ===== 动效标注2：画布旁的说明卡 =====
const note = al('VERTICAL', { name: '📝 动效规范 · 记分', itemSpacing: 10 });
note.paddingLeft = 20; note.paddingRight = 20; note.paddingTop = 18; note.paddingBottom = 18;
bindFill(note, V('color/bg/surface'));
bindRadius(note, V('radius/lg'));
note.resize(300, 100);
note.counterAxisSizingMode = 'FIXED';
note.primaryAxisSizingMode = 'AUTO';
page.appendChild(note);
note.x = root.x + 390 + 40; note.y = 0;
note.appendChild(mkText('动效 · 记分瞬间', 'h2', 'color/score/win'));
const lines = [
  '1. 按下 ± → 按钮切 Pressed（80ms）',
  '2. 分数上方弹出 +N 金色浮标',
  '   上移 12px，400ms 淡出',
  '3. 分数按结果变金/蓝 + glow 300ms',
  '   本局内保持结果色，下局重置',
  '4. 撤销 → toast「已撤销 老王 +10」',
  '',
  '画布中的 +20 胶囊 = 浮标起始帧'
];
for (const ln of lines) {
  const t = mkText(ln === '' ? ' ' : ln, 'caption', ln.startsWith('画布') ? 'color/score/win' : 'color/text/dim');
  note.appendChild(t);
}

const all = root.findAll(() => true).map(n => n.id);
return { screenId: root.id, noteId: note.id, pillId: pill.id, x: root.x, createdNodeIds: [root.id, note.id, ...all], count: all.length + 2 };