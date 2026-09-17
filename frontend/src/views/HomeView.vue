<template>
  <el-container class="app-layout">
    <!-- 顶栏 -->
    <el-header class="app-header">
      <div class="header-left">
        <el-icon size="22"><Van /></el-icon>
        <span class="title">停车违停助手</span>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" @click="showCreateDialog = true">上传违停照片</el-button>
        <el-button :icon="List" @click="$router.push('/records')">全部记录</el-button>
      </div>
    </el-header>

    <el-main class="app-main">
      <!-- 统计面板 -->
      <div class="stats-row">
        <el-card
          class="stat-card stat-remind"
          role="button"
          tabindex="0"
          @click="goStatus('待处理')"
          @keyup.enter="goStatus('待处理')"
          @keyup.space.prevent="goStatus('待处理')"
        >
          <div class="stat-num">{{ dashboard.pending_reminder }}</div>
          <div class="stat-label">待处理</div>
          <div class="stat-hint">（可执行提醒移车）</div>
        </el-card>
        <el-card
          class="stat-card stat-confirm"
          role="button"
          tabindex="0"
          @click="goStatus('待确认')"
          @keyup.enter="goStatus('待确认')"
          @keyup.space.prevent="goStatus('待确认')"
        >
          <div class="stat-num">{{ dashboard.pending_confirm }}</div>
          <div class="stat-label">待确认是否移车</div>
          <div class="stat-hint">（可确认为违停或已挪车）</div>
        </el-card>
      </div>

      <!-- 查询表单 -->
      <el-card class="query-card">
        <el-form :model="query" inline class="query-form">
          <el-form-item label="车牌号">
            <el-input
              v-model="query.plate"
              placeholder="模糊搜索"
              clearable
              style="width:160px"
              @keyup.enter="handleSearch"
            />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" style="width:140px">
              <el-option label="全部状态" :value="STATUS_ALL" />
              <el-option v-for="s in statuses" :key="s" :label="s" :value="s" />
            </el-select>
          </el-form-item>
          <el-form-item label="预警筛选">
            <div class="warning-filter">
              <el-checkbox v-model="query.over_three_warning">仅看</el-checkbox>
              <el-input-number
                v-model="query.warning_threshold"
                :min="1"
                :max="999"
                :step="1"
                controls-position="right"
                style="width:100px"
              />
              <span>次及以上预警车辆</span>
            </div>
          </el-form-item>
          <el-form-item label="时间范围">
            <el-date-picker
              v-model="dateRange"
              type="daterange"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              style="width:240px"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
            <el-button @click="handleReset">重置</el-button>
          </el-form-item>
          <el-form-item class="export-btns">
            <el-button
              :icon="Download"
              :loading="exporting === 'summary'"
              @click="handleExport('summary')"
            >
              导出统计
            </el-button>
            <el-button
              :icon="Download"
              :loading="exporting === 'detail'"
              @click="handleExport('detail')"
            >
              导出详细
            </el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 统计表格 -->
      <el-card class="table-card">
        <el-skeleton v-if="firstLoad" :rows="6" animated />
        <template v-else>
          <el-table
            :data="tableData"
            v-loading="loading"
            stripe
            border
            style="width:100%"
            :row-class-name="rowClassName"
            @row-click="row => $router.push(`/plates/${row.plate_number}`)"
          >
            <el-table-column label="车牌号" prop="plate_number" min-width="120">
              <template #default="{ row }">
                <el-tag :type="row.is_high_frequency ? 'danger' : ''">{{ row.plate_number }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="违停次数" prop="violation_count" width="120" sortable />
            <el-table-column label="最后违停时间" prop="last_violation" min-width="160" sortable />
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" size="small" link @click.stop="$router.push(`/plates/${row.plate_number}`)">详情</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="暂无符合条件的违停车牌">
                <el-button type="primary" :icon="Plus" @click="showCreateDialog = true">上传违停照片</el-button>
              </el-empty>
            </template>
          </el-table>
          <div class="pagination">
            <el-pagination
              v-model:current-page="query.page"
              v-model:page-size="query.page_size"
              :total="total"
              :page-sizes="[10, 20, 50]"
              layout="total, sizes, prev, pager, next"
              @change="handlePageChange"
            />
          </div>
        </template>
      </el-card>
    </el-main>
  </el-container>

  <!-- 上传违停照片弹窗 -->
  <CreateRecordDialog v-model="showCreateDialog" @created="handleCreated" />
</template>

<script setup>
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus, Search, Download, List, Van } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getPlateStats, getDashboard, exportDetail, exportSummary } from '@/api/index.js'
import { firstDayOfMonth, lastDayOfMonth } from '@/utils/datetime.js'
import CreateRecordDialog from '@/components/CreateRecordDialog.vue'

const route = useRoute()
const router = useRouter()

const statuses = ['待处理', '待确认', '违停', '已挪车']
// 「全部状态」：后端识别 all 表示不按状态筛选
const STATUS_ALL = 'all'
const DEFAULT_STATUS = '违停'
const DEFAULT_THRESHOLD = 3
const DEFAULT_PAGE_SIZE = 20

const loading = ref(false)
const firstLoad = ref(true)
const tableData = ref([])
const total = ref(0)
const showCreateDialog = ref(false)
const exporting = ref('')
const dashboard = ref({ pending_reminder: 0, pending_confirm: 0 })

const dateRange = ref([firstDayOfMonth(), lastDayOfMonth()])
const query = reactive({
  plate: '',
  status: DEFAULT_STATUS,
  over_three_warning: false,
  warning_threshold: DEFAULT_THRESHOLD,
  page: 1,
  page_size: DEFAULT_PAGE_SIZE
})

function currentThreshold() {
  return Number(query.warning_threshold) > 0 ? Number(query.warning_threshold) : DEFAULT_THRESHOLD
}

function buildParams(extra = {}) {
  const p = {
    plate: query.plate,
    status: query.status,
    over_three_warning: query.over_three_warning ? 1 : 0,
    warning_threshold: currentThreshold(),
    ...extra
  }
  if (dateRange.value?.length === 2) {
    p.start_date = dateRange.value[0]
    p.end_date = dateRange.value[1]
  }
  return p
}

const exportParams = computed(() => buildParams())

// ---- 查询条件与 URL 同步，从详情返回时能保留筛选与页码 ----
function buildRouteQuery() {
  const q = {}
  if (query.plate) q.plate = query.plate
  if (query.status !== DEFAULT_STATUS) q.status = query.status
  if (query.over_three_warning) q.warn = '1'
  if (currentThreshold() !== DEFAULT_THRESHOLD) q.threshold = String(currentThreshold())
  if (dateRange.value?.length === 2) {
    q.start_date = dateRange.value[0]
    q.end_date = dateRange.value[1]
  }
  if (query.page > 1) q.page = String(query.page)
  if (query.page_size !== DEFAULT_PAGE_SIZE) q.size = String(query.page_size)
  return q
}

function applyRouteQuery(q) {
  query.plate = q.plate || ''
  query.status = q.status || DEFAULT_STATUS
  query.over_three_warning = q.warn === '1'
  query.warning_threshold = Number(q.threshold) > 0 ? Number(q.threshold) : DEFAULT_THRESHOLD
  query.page = Number(q.page) > 0 ? Number(q.page) : 1
  query.page_size = Number(q.size) > 0 ? Number(q.size) : DEFAULT_PAGE_SIZE
  dateRange.value = q.start_date && q.end_date
    ? [q.start_date, q.end_date]
    : [firstDayOfMonth(), lastDayOfMonth()]
}

function syncRoute() {
  router.replace({ query: buildRouteQuery() })
}

// 判断路由参数是否与当前表单状态一致，避免自身 replace 触发重复加载
function routeQueryMatchesState(q) {
  const current = buildRouteQuery()
  const keys = new Set([...Object.keys(q), ...Object.keys(current)])
  for (const key of keys) {
    if (String(q[key] ?? '') !== String(current[key] ?? '')) {
      return false
    }
  }
  return true
}

async function loadData() {
  loading.value = true
  try {
    const res = await getPlateStats(buildParams({ page: query.page, page_size: query.page_size }))
    // 筛选或翻页后可能落到空页，自动回退
    if ((res.stats || []).length === 0 && res.total > 0 && query.page > 1) {
      query.page -= 1
      syncRoute()
      return await loadData()
    }
    tableData.value = res.stats || []
    total.value = res.total || 0
  } finally {
    loading.value = false
    firstLoad.value = false
  }
}

async function loadDashboard() {
  dashboard.value = await getDashboard()
}

function handleSearch() {
  query.page = 1
  syncRoute()
  loadData()
}

function handleReset() {
  query.plate = ''
  query.status = DEFAULT_STATUS
  query.over_three_warning = false
  query.warning_threshold = DEFAULT_THRESHOLD
  query.page = 1
  query.page_size = DEFAULT_PAGE_SIZE
  dateRange.value = [firstDayOfMonth(), lastDayOfMonth()]
  syncRoute()
  loadData()
}

function handlePageChange() {
  syncRoute()
  loadData()
}

async function handleExport(kind) {
  exporting.value = kind
  try {
    const task = kind === 'detail'
      ? exportDetail(exportParams.value)
      : exportSummary(exportParams.value)
    await task
    ElMessage.success('导出完成，请查看浏览器下载')
  } catch {
    // 失败提示由请求层统一处理
  } finally {
    exporting.value = ''
  }
}

function goStatus(status) {
  router.push({ name: 'records', query: { status } })
}

function rowClassName({ row }) {
  return row.is_high_frequency ? 'high-frequency-row' : ''
}

function handleCreated() {
  loadData()
  loadDashboard()
}

watch(
  () => route.query,
  (q) => {
    if (routeQueryMatchesState(q)) return
    applyRouteQuery(q)
    loadData()
  }
)

onMounted(() => {
  applyRouteQuery(route.query)
  syncRoute()
  loadData()
  loadDashboard()
})
</script>

<style scoped>
.app-layout { min-height: 100vh; background: #f5f7fa; }
.app-header {
  display: flex; align-items: center; justify-content: space-between;
  background: #409eff; color: #fff; padding: 0 24px;
}
.header-left { display: flex; align-items: center; gap: 10px; }
.header-right { display: flex; gap: 8px; }
.title { font-size: 20px; font-weight: bold; }
.app-main { padding: 20px; }
.stats-row { display: flex; gap: 16px; margin-bottom: 16px; }
.stat-card {
  flex: 1; text-align: center; cursor: pointer;
  transition: transform .2s; border-radius: 8px;
}
.stat-card:hover { transform: translateY(-2px); }
.stat-card:focus-visible { outline: 2px solid #409eff; outline-offset: 2px; }
.stat-num { font-size: 40px; font-weight: bold; color: #e74c3c; }
.stat-remind .stat-num { color: #e67e22; }
.stat-confirm .stat-num { color: #e74c3c; }
.stat-label { font-size: 16px; margin-top: 4px; }
.stat-hint { font-size: 12px; color: #999; }
.query-card { margin-bottom: 16px; }
.warning-filter { display: inline-flex; align-items: center; gap: 8px; }
.query-form { flex-wrap: wrap; }
.export-btns { margin-left: auto; }
.pagination { margin-top: 16px; display: flex; justify-content: flex-end; }

@media (max-width: 768px) {
  .app-header {
    height: auto; min-height: 60px; padding: 8px 12px;
    flex-wrap: wrap; gap: 8px;
  }
  .title { font-size: 16px; }
  .app-main { padding: 12px; }
  .stats-row { flex-direction: column; gap: 8px; }
  .stat-num { font-size: 28px; }
  .query-form :deep(.el-form-item) { margin-right: 0; width: 100%; }
  .query-form :deep(.el-input),
  .query-form :deep(.el-select),
  .query-form :deep(.el-date-editor) { width: 100% !important; }
  .export-btns { margin-left: 0; }
  .pagination { justify-content: center; }
}
</style>

<style>
.high-frequency-row { background-color: #fff0f0 !important; }
</style>
