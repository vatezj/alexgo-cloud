// 05 战绩统计 — fileKey: dHMgzmrrWJ8ztjWytPXdIW, 页 📱 Screens (0:1)
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
  hero: 'S:5f7870f48f5e9cc6174ddc15354cca882c428498,', sm: 'S:f0cfc3d3c829ed1858dbd1b23274271e9852d537,',
  h1: 'S:51c7c69b43b3675ff5878d9897cfeac47cf4e243,', h2: 'S:e724ff93ae56f28ce2c7d9081ef81386b562f6d7,',
  body: 'S:07b03ff2335b2c026337a0685835cce71af9dcd5,', label: 'S:3e082326e7ed79e94f11756f1118fd7f85e4b87a,',
  caption: 'S:ebe8931ec6c3010f156a6fd75958f3addc62baf8,'
};
const al = (dir, props = {}) => { const f = figma.createAutoLayout(dir, props); f.fills = []; return f; };
const mkText = (chars, style, colorVar) => { const t = figma.createText(); t.textStyleId = TS[style]; t.characters = chars; bindFill(t, V(colorVar)); return t; };

const dup = page.children.find(c => c.name === '05 战绩统计');
if (dup) dup.remove();
let maxX = 0;
for (const c of page.children) maxX = Math.max(maxX, c.x + c.width);

const root = figma.createFrame();
root.name = '05 战绩统计';
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
sb.appendChild(mkText('21:40', 'label', 'color/text/primary'));
sb.appendChild(mkText('5G ▮100%', 'caption', 'color/text/dim'));

const hd = al('VERTICAL', { name: 'Header', itemSpacing: 2 });
hd.paddingLeft = 20; hd.paddingRight = 20; hd.paddingTop = 12; hd.paddingBottom = 4;
root.appendChild(hd); hd.layoutSizingHorizontal = 'FILL';
hd.appendChild(mkText('战绩', 'h1', 'color/text/primary'));
hd.appendChild(mkText('从 2026 年 6 月起', 'caption', 'color/text/dim'));

// Hero：累计净成绩
const hero = al('VERTICAL', { name: 'Hero/累计', itemSpacing: 4 });
hero.paddingTop = 16; hero.paddingBottom = 16;
root.appendChild(hero); hero.layoutSizingHorizontal = 'FILL';
hero.paddingLeft = 20;
hero.appendChild(mkText('累计净成绩', 'label', 'color/text/dim'));
hero.appendChild(mkText('+1,240', 'hero', 'color/score/win'));
hero.appendChild(mkText('32 局 · 胜率 56%', 'label', 'color/text/dim'));

// 按玩法分组（条形）
const bars = al('VERTICAL', { name: 'Section/按玩法', itemSpacing: 14 });
bars.paddingLeft = 20; bars.paddingRight = 20; bars.paddingTop = 12; bars.paddingBottom = 8;
root.appendChild(bars); bars.layoutSizingHorizontal = 'FILL';
bars.appendChild(mkText('按玩法', 'h2', 'color/text/primary'));
const games = [
  ['麻将', '+860', 280, 'color/score/win'],
  ['掼蛋', '+320', 104, 'color/score/win'],
  ['斗地主', '+100', 32, 'color/score/win'],
  ['德州', '−40', 14, 'color/score/lose']
];
for (const [name, amt, w, cv] of games) {
  const g = al('VERTICAL', { name: 'Bar/' + name, itemSpacing: 6 });
  bars.appendChild(g); g.layoutSizingHorizontal = 'FILL';
  const row = al('HORIZONTAL', { name: 'Row', itemSpacing: 8 });
  row.primaryAxisAlignItems = 'SPACE_BETWEEN'; row.counterAxisAlignItems = 'CENTER';
  g.appendChild(row); row.layoutSizingHorizontal = 'FILL';
  row.appendChild(mkText(name, 'body', 'color/text/primary'));
  row.appendChild(mkText(amt, 'sm', cv));
  const track = figma.createFrame();
  track.name = 'Track';
  track.resize(350, 8);
  bindFill(track, V('color/bg/pressed'));
  bindRadius(track, V('radius/full'));
  g.appendChild(track);
  track.layoutSizingHorizontal = 'FILL';
  const fillBar = figma.createRectangle();
  fillBar.name = 'Fill';
  fillBar.resize(Math.max(w, 8), 8);
  bindFill(fillBar, V(cv));
  bindRadius(fillBar, V('radius/full'));
  track.appendChild(fillBar);
  fillBar.x = 0; fillBar.y = 0;
}

// 最近牌局
const rec = al('VERTICAL', { name: 'Section/最近牌局', itemSpacing: 8 });
rec.paddingLeft = 20; rec.paddingRight = 20; rec.paddingTop = 12; rec.paddingBottom = 8;
root.appendChild(rec); rec.layoutSizingHorizontal = 'FILL';
rec.appendChild(mkText('最近牌局', 'h2', 'color/text/primary'));
const recent = [
  ['10月8日 · 麻将', '+180', 'color/score/win'],
  ['10月5日 · 掼蛋', '−60', 'color/score/lose'],
  ['10月2日 · 斗地主', '+45', 'color/score/win'],
  ['9月28日 · 麻将', '−120', 'color/score/lose']
];
for (const [info, amt, cv] of recent) {
  const row = al('HORIZONTAL', { name: 'RecentRow', itemSpacing: 8 });
  row.paddingLeft = 16; row.paddingRight = 16; row.paddingTop = 12; row.paddingBottom = 12;
  row.primaryAxisAlignItems = 'SPACE_BETWEEN'; row.counterAxisAlignItems = 'CENTER';
  bindFill(row, V('color/bg/surface')); bindRadius(row, V('radius/md'));
  rec.appendChild(row); row.layoutSizingHorizontal = 'FILL';
  row.appendChild(mkText(info, 'body', 'color/text/primary'));
  row.appendChild(mkText(amt, 'sm', cv));
}

const sp = figma.createFrame();
sp.name = 'Spacer'; sp.fills = []; sp.resize(10, 10);
root.appendChild(sp); sp.layoutGrow = 1; sp.layoutSizingHorizontal = 'FILL';

const tab = (await figma.getNodeByIdAsync('17:28')).createInstance();
root.appendChild(tab);
tab.layoutSizingHorizontal = 'FILL';

const all = root.findAll(() => true).map(n => n.id);
return { screenId: root.id, x: root.x, createdNodeIds: [root.id, ...all], count: all.length + 1 };