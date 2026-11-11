// 02 开局设置 — 目标 fileKey: dHMgzmrrWJ8ztjWytPXdIW, 页 📱 Screens (0:1)
const page = await figma.getNodeByIdAsync('0:1');
await figma.setCurrentPageAsync(page);
await Promise.all([
  figma.loadFontAsync({ family: 'Oswald', style: 'Medium' }),
  figma.loadFontAsync({ family: 'Noto Sans SC', style: 'Bold' }),
  figma.loadFontAsync({ family: 'Noto Sans SC', style: 'Medium' }),
  figma.loadFontAsync({ family: 'Noto Sans SC', style: 'Regular' })
]);
const allVars = await figma.variables.getLocalVariablesAsync();
const byName = Object.fromEntries(allVars.map(v => [v.name, v]));
const V = (n) => { const v = byName[n]; if (!v) throw new Error('missing var ' + n); return v; };
const bindFill = (node, v) => { node.fills = [figma.variables.setBoundVariableForPaint({ type: 'SOLID', color: { r: 0, g: 0, b: 0 } }, 'color', v)]; };
const bindStroke = (node, v) => { node.strokes = [figma.variables.setBoundVariableForPaint({ type: 'SOLID', color: { r: 0, g: 0, b: 0 } }, 'color', v)]; };
const bindRadius = (node, v) => { node.setBoundVariable('topLeftRadius', v); node.setBoundVariable('topRightRadius', v); node.setBoundVariable('bottomLeftRadius', v); node.setBoundVariable('bottomRightRadius', v); };
const TS = {
  h1: 'S:51c7c69b43b3675ff5878d9897cfeac47cf4e243,', h2: 'S:e724ff93ae56f28ce2c7d9081ef81386b562f6d7,',
  body: 'S:07b03ff2335b2c026337a0685835cce71af9dcd5,', label: 'S:3e082326e7ed79e94f11756f1118fd7f85e4b87a,',
  caption: 'S:ebe8931ec6c3010f156a6fd75958f3addc62baf8,'
};
const al = (dir, props = {}) => { const f = figma.createAutoLayout(dir, props); f.fills = []; return f; };
const mkText = (chars, style, colorVar) => { const t = figma.createText(); t.textStyleId = TS[style]; t.characters = chars; bindFill(t, V(colorVar)); return t; };

// 幂等：重名屏幕先删
const dup = page.children.find(c => c.name === '02 开局设置');
if (dup) dup.remove();

let maxX = 0;
for (const c of page.children) maxX = Math.max(maxX, c.x + c.width);

const root = figma.createFrame();
root.name = '02 开局设置';
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
sb.appendChild(mkText('21:30', 'label', 'color/text/primary'));
sb.appendChild(mkText('5G ▮100%', 'caption', 'color/text/dim'));

const hd = al('HORIZONTAL', { name: 'Header', itemSpacing: 12 });
hd.paddingLeft = 20; hd.paddingRight = 20; hd.paddingTop = 12; hd.paddingBottom = 8;
hd.counterAxisAlignItems = 'CENTER';
root.appendChild(hd); hd.layoutSizingHorizontal = 'FILL';
hd.appendChild(mkText('‹', 'h1', 'color/text/primary'));
hd.appendChild(mkText('新开局', 'h1', 'color/text/primary'));

const gt = al('VERTICAL', { name: 'Section/游戏类型', itemSpacing: 12 });
gt.paddingLeft = 20; gt.paddingRight = 20; gt.paddingTop = 16; gt.paddingBottom = 8;
root.appendChild(gt); gt.layoutSizingHorizontal = 'FILL';
gt.appendChild(mkText('选玩法', 'label', 'color/text/dim'));
const mkChip = (label, selected) => {
  const chip = al('HORIZONTAL', { name: 'Chip/' + label });
  chip.paddingLeft = 22; chip.paddingRight = 22; chip.paddingTop = 11; chip.paddingBottom = 11;
  bindRadius(chip, V('radius/full'));
  if (selected) { bindFill(chip, V('color/action/primary')); }
  else {
    bindFill(chip, V('color/bg/surface'));
    bindStroke(chip, V('color/border/default')); chip.strokeWeight = 1;
  }
  chip.appendChild(mkText(label, 'body', selected ? 'color/action/primary-text' : 'color/text/primary'));
  return chip;
};
const row1 = al('HORIZONTAL', { name: 'ChipRow', itemSpacing: 10 });
gt.appendChild(row1); row1.layoutSizingHorizontal = 'FILL';
row1.appendChild(mkChip('麻将', true));
row1.appendChild(mkChip('斗地主', false));
row1.appendChild(mkChip('掼蛋', false));
const row2 = al('HORIZONTAL', { name: 'ChipRow', itemSpacing: 10 });
gt.appendChild(row2); row2.layoutSizingHorizontal = 'FILL';
row2.appendChild(mkChip('德州', false));

const pl = al('VERTICAL', { name: 'Section/玩家', itemSpacing: 12 });
pl.paddingLeft = 20; pl.paddingRight = 20; pl.paddingTop = 16; pl.paddingBottom = 8;
root.appendChild(pl); pl.layoutSizingHorizontal = 'FILL';
const plHead = al('HORIZONTAL', { name: 'Row', itemSpacing: 8 });
plHead.primaryAxisAlignItems = 'SPACE_BETWEEN'; plHead.counterAxisAlignItems = 'CENTER';
pl.appendChild(plHead); plHead.layoutSizingHorizontal = 'FILL';
plHead.appendChild(mkText('玩家', 'label', 'color/text/dim'));
plHead.appendChild(mkText('4 人', 'label', 'color/score/win'));
const avRow = al('HORIZONTAL', { name: 'AvatarRow', itemSpacing: 16 });
avRow.primaryAxisAlignItems = 'SPACE_BETWEEN';
pl.appendChild(avRow); avRow.layoutSizingHorizontal = 'FILL';
const players = [['王', '老王'], ['李', '李姐'], ['强', '阿强'], ['我', '我']];
for (const [ini, nm] of players) {
  const col = al('VERTICAL', { name: 'Player/' + nm, itemSpacing: 6 });
  col.counterAxisAlignItems = 'CENTER';
  const av = al('HORIZONTAL', { name: 'Avatar' });
  av.primaryAxisAlignItems = 'CENTER'; av.counterAxisAlignItems = 'CENTER';
  av.resize(56, 56); av.primaryAxisSizingMode = 'FIXED'; av.counterAxisSizingMode = 'FIXED';
  bindFill(av, V('color/bg/pressed')); bindRadius(av, V('radius/full'));
  av.appendChild(mkText(ini, 'h2', nm === '我' ? 'color/score/win' : 'color/text/primary'));
  col.appendChild(av);
  col.appendChild(mkText(nm, 'caption', 'color/text/dim'));
  avRow.appendChild(col);
}
const addCol = al('VERTICAL', { name: 'Player/添加', itemSpacing: 6 });
addCol.counterAxisAlignItems = 'CENTER';
const addAv = al('HORIZONTAL', { name: 'AddAvatar' });
addAv.primaryAxisAlignItems = 'CENTER'; addAv.counterAxisAlignItems = 'CENTER';
addAv.resize(56, 56); addAv.primaryAxisSizingMode = 'FIXED'; addAv.counterAxisSizingMode = 'FIXED';
addAv.fills = []; bindRadius(addAv, V('radius/full'));
bindStroke(addAv, V('color/border/default')); addAv.strokeWeight = 1.5;
addAv.dashPattern = [4, 4];
addAv.appendChild(mkText('＋', 'h2', 'color/text/dim'));
addCol.appendChild(addAv);
addCol.appendChild(mkText('加一位', 'caption', 'color/text/dim'));
avRow.appendChild(addCol);

const bs = al('VERTICAL', { name: 'Section/底分', itemSpacing: 12 });
bs.paddingLeft = 20; bs.paddingRight = 20; bs.paddingTop = 16; bs.paddingBottom = 8;
bs.counterAxisAlignItems = 'CENTER';
root.appendChild(bs); bs.layoutSizingHorizontal = 'FILL';
bs.appendChild(mkText('底分', 'label', 'color/text/dim'));
const bsc = (await figma.getNodeByIdAsync('15:11')).createInstance();
bs.appendChild(bsc);
bs.appendChild(mkText('每一分 ¥10，结束后按总分结算', 'caption', 'color/text/dim'));

const sp = figma.createFrame();
sp.name = 'Spacer'; sp.fills = []; sp.resize(10, 10);
root.appendChild(sp); sp.layoutGrow = 1; sp.layoutSizingHorizontal = 'FILL';

const cta = al('VERTICAL', { name: 'Section/CTA', itemSpacing: 10 });
cta.paddingLeft = 20; cta.paddingRight = 20; cta.paddingTop = 8; cta.paddingBottom = 28;
root.appendChild(cta); cta.layoutSizingHorizontal = 'FILL';
const start = (await figma.getNodeByIdAsync('15:19')).createInstance();
cta.appendChild(start); start.layoutSizingHorizontal = 'FILL';
start.findAllWithCriteria({ types: ['TEXT'] })[0].characters = '开始牌局';
const hint = mkText('4 人 · 麻将 · 底分 ¥10', 'caption', 'color/text/dim');
hint.textAlignHorizontal = 'CENTER';
cta.appendChild(hint);
hint.layoutSizingHorizontal = 'FILL';

const all = root.findAll(() => true).map(n => n.id);
return { screenId: root.id, x: root.x, createdNodeIds: [root.id, ...all], count: all.length + 1 };
