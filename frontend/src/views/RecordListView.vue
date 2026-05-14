<template>
  <el-container class="app-layout">
    <el-header class="app-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" @click="$router.push('/')" text style="color:#fff">返回主页</el-button>
        <el-icon size="20"><List /></el-icon>
        <span class="title">全部违停记录</span>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" @click="showCreateDialog = true">上传违停照片</el-button>
      </div>
    </el-header>
    <el-main class="app-main">
      <!-- 查询表单 -->
      <el-card class="query-card">
        <el-form :model="query" inline>
          <el-form-item label="车牌号">
            <el-input v-model="query.plate" placeholder="模糊搜索" clearable style="width:140px" />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" clearable placeholder="全部" style="width:130px">
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
        </el-form>
      </el-card>

      <!-- 记录表格 -->
      <el-card>
        <el-table :data="tableData" v-loading="loading" stripe border style="width:100%">
          <el-table-column label="ID" prop="id" width="70" />
          <el-table-column label="车牌号" prop="plate_number" width="130">
            <template #default="{ row }">
              <el-button type="primary" link @click="$router.push(`/plates/${row.plate_number}`)">{{ row.plate_number }}</el-button>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="120">
            <template #default="{ row }"><StatusTag :status="row.status" /></template>
          </el-table-column>
          <el-table-column label="停车时间" prop="parking_time" min-width="150" />
          <el-table-column label="备注" prop="notes" min-width="120" show-overflow-tooltip />
          <el-table-column label="创建时间" prop="created_at" width="160" />
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" size="small" link @click="$router.push(`/plates/${row.plate_number}`)">详情</el-button>
              <el-button type="danger" size="small" link @click="handleDelete(row)">删除</el-button>
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

  <CreateRecordDialog v-model="showCreateDialog" @created="loadData" />
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Plus, Search, List } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getRecords, deleteRecord } from '@/api/index.js'
import StatusTag from '@/components/StatusTag.vue'
import CreateRecordDialog from '@/components/CreateRecordDialog.vue'

const route = useRoute()
const router = useRouter()

const statuses = ['待处理', '待确认', '违停', '已挪车']
const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const showCreateDialog = ref(false)
const dateRange = ref(null)
const query = reactive({
  plate: '',
  status: route.query.status || '',
  page: 1,
  page_size: 20
})

async function loadData() {
  loading.value = true
  try {
    const params = {
      plate: query.plate,
      status: query.status,
      page: query.page,
      page_size: query.page_size
    }
    if (dateRange.value?.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    const res = await getRecords(params)
    tableData.value = res.records || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  loadData()
}

function handleReset() {
  query.plate = ''
  query.status = ''
  dateRange.value = null
  query.page = 1
  loadData()
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除记录 #${row.id}？`, '删除确认', { type: 'warning' })
  await deleteRecord(row.id)
  ElMessage.success('已删除')
  loadData()
}

onMounted(loadData)
</script>

<style scoped>
.app-layout { min-height: 100vh; background: #f5f7fa; }
.app-header {
  display: flex; align-items: center; justify-content: space-between;
  background: #409eff; color: #fff; padding: 0 24px;
}
.header-left { display: flex; align-items: center; gap: 10px; }
.header-right { display: flex; gap: 8px; }
.title { font-size: 18px; font-weight: bold; }
.app-main { padding: 20px; }
.query-card { margin-bottom: 16px; }
.pagination { margin-top: 16px; display: flex; justify-content: flex-end; }
</style>
