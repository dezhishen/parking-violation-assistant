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
        <el-card class="stat-card stat-remind" @click="goStatus('待处理')">
          <div class="stat-num">{{ dashboard.pending_reminder }}</div>
          <div class="stat-label">待处理</div>
          <div class="stat-hint">（可执行提醒移车）</div>
        </el-card>
        <el-card class="stat-card stat-confirm" @click="goStatus('待确认')">
          <div class="stat-num">{{ dashboard.pending_confirm }}</div>
          <div class="stat-label">待确认是否移车</div>
          <div class="stat-hint">（可确认为违停或已挪车）</div>
        </el-card>
      </div>

      <!-- 查询表单 -->
      <el-card class="query-card">
        <el-form :model="query" inline class="query-form">
          <el-form-item label="车牌号">
            <el-input v-model="query.plate" placeholder="模糊搜索" clearable style="width:160px" />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" clearable placeholder="默认仅违停" style="width:140px">
              <el-option v-for="s in statuses" :key="s" :label="s" :value="s" />
            </el-select>
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
            <el-button :icon="Download" @click="handleExportSummary">导出统计</el-button>
            <el-button :icon="Download" @click="handleExportDetail">导出详细</el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 统计表格 -->
      <el-card class="table-card">
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
        </el-table>
        <div class="pagination">
          <el-pagination
            v-model:current-page="query.page"
            v-model:page-size="query.page_size"
            :total="total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next"
            @change="loadData"
          />
        </div>
      </el-card>
    </el-main>
  </el-container>

  <!-- 上传违停照片弹窗 -->
  <CreateRecordDialog v-model="showCreateDialog" @created="handleCreated" />
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Search, Download, List } from '@element-plus/icons-vue'
import { getPlateStats, getDashboard, exportDetail, exportSummary } from '@/api/index.js'
import CreateRecordDialog from '@/components/CreateRecordDialog.vue'

const router = useRouter()

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const showCreateDialog = ref(false)
const dashboard = ref({ pending_reminder: 0, pending_confirm: 0 })

// 当前自然月
const now = new Date()
const firstDay = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-01`
const lastDay = new Date(now.getFullYear(), now.getMonth() + 1, 0)
  .toISOString().slice(0, 10)

const dateRange = ref([firstDay, lastDay])
const query = reactive({
  plate: '',
  status: '',
  page: 1,
  page_size: 20
})

const statuses = ['待处理', '待确认', '违停', '已挪车']

const exportParams = computed(() => {
  const p = { plate: query.plate, status: query.status }
  if (dateRange.value?.length === 2) {
    p.start_date = dateRange.value[0]
    p.end_date = dateRange.value[1]
  }
  return p
})

async function loadData() {
  loading.value = true
  try {
    const params = { ...exportParams.value, page: query.page, page_size: query.page_size }
    const res = await getPlateStats(params)
    tableData.value = res.stats || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

async function loadDashboard() {
  dashboard.value = await getDashboard()
}

function handleSearch() {
  query.page = 1
  loadData()
}

function handleReset() {
  query.plate = ''
  query.status = ''
  dateRange.value = [firstDay, lastDay]
  query.page = 1
  loadData()
}

function handleExportDetail() {
  exportDetail(exportParams.value)
}

function handleExportSummary() {
  exportSummary(exportParams.value)
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

onMounted(() => {
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
.stat-num { font-size: 40px; font-weight: bold; color: #e74c3c; }
.stat-remind .stat-num { color: #e67e22; }
.stat-confirm .stat-num { color: #e74c3c; }
.stat-label { font-size: 16px; margin-top: 4px; }
.stat-hint { font-size: 12px; color: #999; }
.query-card { margin-bottom: 16px; }
.query-form { flex-wrap: wrap; }
.export-btns { margin-left: auto; }
.table-card {}
.pagination { margin-top: 16px; display: flex; justify-content: flex-end; }
</style>

<style>
.high-frequency-row { background-color: #fff0f0 !important; }
</style>
