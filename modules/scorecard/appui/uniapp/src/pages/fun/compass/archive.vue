<script setup lang="ts">
import type { FunCompassArchive } from '@/store/funCompass/archives'
/**
 * 出生档案列表页 pages/fun/compass/archive（2026-11-06 修订轮）
 *
 * - 列表来自 fun_compass_archives store（本机持久化，只读写本键；schema v2）
 * - 每卡：称呼 + 生辰行（年月日 时间 · 性别）+ 编辑/删除（五行标签随本地排盘废止）
 * - 删除走 useDialog('globalDialog') 确认（项目惯例，msg 含「只删本机数据」）
 * - 编辑 → bazi?id=（首次 onShow 预填）；CTA 新增 → bazi；空态写行动指引
 */
import ModDisclaimer from '@/components/funCompass/ModDisclaimer.vue'
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { useFunCompassArchivesStore } from '@/store/funCompass/archives'
import { ARCHIVE_EMPTY_HINT, LOCAL_ONLY_LIST } from '@/utils/funCompass/copy'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
// 裸 useDialog() inject 不到该 key 会自建孤儿 ref，confirm 永不弹且 promise 挂起。
const dialog = useDialog('globalDialog')
const { pageStyle } = useSystemBar()
const store = useFunCompassArchivesStore()

const archives = computed(() => store.sorted)

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

function birthLine(a: FunCompassArchive): string {
  const gender = a.gender === 'female' ? '女' : '男'
  return `${a.year}年${a.month}月${a.day}日 ${pad2(a.hour)}:${pad2(a.minute)} · ${gender}`
}

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/fun/compass/index' })
}

function goAdd() {
  router.push({ path: '/pages/fun/compass/bazi' })
}

function goEdit(a: FunCompassArchive) {
  router.push({ path: '/pages/fun/compass/bazi', query: { id: a.id } })
}

const deleting = ref(false)

async function onDelete(a: FunCompassArchive) {
  if (deleting.value)
    return
  deleting.value = true
  try {
    try {
      await dialog.confirm({
        title: '删除档案',
        msg: `删除「${a.name || '未命名'}」的出生档案？只删本机数据，不影响账本记录。`,
        confirmButtonText: '确认删除',
        cancelButtonText: '再想想',
      })
    }
    catch {
      return
    }
    store.remove(a.id)
    toast.success({ msg: '已删除' })
  }
  finally {
    deleting.value = false
  }
}
</script>

<template>
  <view
    class="fc-page"
    :style="pageStyle"
  >
    <ModNav
      title="出生档案"
      @back="goBack"
    />

    <view class="fc-priv">
      <view class="fc-priv-dot" />
      <text class="fc-priv-t">
        {{ LOCAL_ONLY_LIST }}
      </text>
    </view>

    <!-- 空态：行动指引 -->
    <view
      v-if="archives.length === 0"
      class="fc-empty"
    >
      <text class="fc-empty-t">
        {{ ARCHIVE_EMPTY_HINT }}
      </text>
    </view>

    <!-- 档案卡列表 -->
    <view
      v-for="a in archives"
      :key="a.id"
      class="fc-acard"
    >
      <view class="fc-acard-head">
        <text class="fc-acard-name">
          {{ a.name || '未命名' }}
        </text>
        <wd-icon
          css-icon
          class="fc-chev"
          name="i-carbon-chevron-right"
        />
      </view>
      <text class="fc-acard-birth">
        {{ birthLine(a) }}
      </text>
      <view class="fc-acard-tags">
        <text class="fc-tag">
          {{ a.gender === 'female' ? '女' : '男' }}
        </text>
        <view class="fc-acard-acts">
          <text
            class="fc-act edit"
            @click="goEdit(a)"
          >
            编辑
          </text>
          <text class="fc-act-sep">
            ·
          </text>
          <text
            class="fc-act del"
            @click="onDelete(a)"
          >
            删除
          </text>
        </view>
      </view>
    </view>

    <view class="fc-spacer" />

    <view
      class="fc-btn primary"
      @click="goAdd"
    >
      + 新增档案
    </view>

    <ModDisclaimer />
  </view>
</template>

<style lang="scss" scoped>
.fc-page {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

.fc-priv {
  display: flex;
  gap: 6px;
  align-items: center;
  margin: 2px 0 12px;
}

.fc-priv-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--jade-hi);
}

.fc-priv-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
}

.fc-empty {
  padding: 28px 16px;
  border: 1px dashed var(--on-ink-a28);
  border-radius: var(--r-tile);
  text-align: center;
}

.fc-empty-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 13px;
  line-height: 1.6;
}

/* 骨白档案卡 */
.fc-acard {
  margin-bottom: 12px;
  padding: 14px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.fc-acard-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.fc-acard-name {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 19px;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.fc-chev {
  flex: none;
  color: var(--jet);
  font-size: 18px;
  opacity: 0.4;
}

.fc-acard-birth {
  display: block;
  margin-top: 4px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  opacity: 0.65;
}

.fc-acard-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-top: 10px;
}

.fc-tag {
  padding: 2px 10px;
  border: 1px solid var(--tile-shade);
  border-radius: 999px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
}

.fc-acard-acts {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-left: auto;
}

.fc-act {
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  font-weight: 500;
}

.fc-act.edit {
  color: var(--indigo);
}

.fc-act.del {
  color: var(--vermillion-600);
}

.fc-act-sep {
  color: var(--jet);
  font-size: 12px;
  opacity: 0.4;
}

.fc-spacer {
  flex: 1;
  min-height: 16px;
}

.fc-btn {
  display: flex;
  height: 48px;
  align-items: center;
  justify-content: center;
  margin-bottom: 6px;
  border-radius: 999px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15.5px;
  font-weight: 500;
}

.fc-btn.primary {
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}
</style>
