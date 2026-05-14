<template>
  <el-container class="app-layout">
    <el-header class="app-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" @click="$router.back()" text style="color:#fff">返回</el-button>
        <el-icon size="20"><Van /></el-icon>
        <span class="title">车牌详情：{{ plate }}</span>
      </div>
    </el-header>
    <el-main class="app-main">
      <el-card v-loading="loading">
        <template #header>
          <span>{{ plate }} 的所有违停记录（共 {{ records.length }} 条）</span>
        </template>
        <el-empty v-if="!loading && records.length === 0" description="暂无记录" />
        <div v-for="rec in records" :key="rec.id" class="record-card">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="记录ID">{{ rec.id }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <StatusTag :status="rec.status" />
            </el-descriptions-item>
            <el-descriptions-item label="停车时间">{{ rec.parking_time || '-' }}</el-descriptions-item>
            <el-descriptions-item label="提醒时间">{{ rec.reminder_time || '-' }}</el-descriptions-item>
            <el-descriptions-item label="第二次检查时间">{{ rec.second_check_time || '-' }}</el-descriptions-item>
            <el-descriptions-item label="备注">{{ rec.notes || '-' }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ rec.created_at }}</el-descriptions-item>
          </el-descriptions>
          <div class="photos">
            <div v-if="rec.image_path" class="photo-item">
              <div class="photo-label">第一张照片</div>
              <el-image :src="rec.image_path" fit="cover" :preview-src-list="[rec.image_path]" class="photo" />
            </div>
            <div v-if="rec.second_image_path" class="photo-item">
              <div class="photo-label">第二张照片</div>
              <el-image :src="rec.second_image_path" fit="cover" :preview-src-list="[rec.second_image_path]" class="photo" />
            </div>
          </div>
          <div class="actions">
            <template v-if="rec.status === '待处理'">
              <el-button type="warning" size="small" @click="remind(rec)">提醒移车</el-button>
            </template>
            <template v-else-if="rec.status === '待确认'">
              <el-button type="danger" size="small" @click="confirmViolation(rec)">确认违停</el-button>
              <el-button type="success" size="small" @click="confirmMoved(rec)">确认已挪车</el-button>
            </template>
            <el-button type="danger" size="small" plain @click="handleDelete(rec)">删除</el-button>
          </div>
          <el-divider />
        </div>
      </el-card>
    </el-main>

    <ConfirmViolationDialog
      :model-value="confirmDialogVisible"
      @update:model-value="confirmDialogVisible = $event"
      @confirmed="handleConfirmViolationSubmit"
    />
  </el-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getPlateDetail, updateRecord, deleteRecord } from '@/api/index.js'
import StatusTag from '@/components/StatusTag.vue'
import ConfirmViolationDialog from '@/components/ConfirmViolationDialog.vue'

const route = useRoute()
const router = useRouter()
const plate = route.params.plate
const records = ref([])
const loading = ref(false)
const confirmDialogVisible = ref(false)
const currentRecord = ref(null)

async function load() {
  loading.value = true
  try {
    records.value = await getPlateDetail(plate)
  } finally {
    loading.value = false
  }
}

async function remind(rec) {
  const now = new Date().toISOString().slice(0, 19).replace('T', ' ')
  await updateRecord(rec.id, { status: '待确认', reminder_time: now })
  ElMessage.success('已提醒移车，状态变更为待确认')
  load()
}

async function confirmViolation(rec) {
  currentRecord.value = rec
  confirmDialogVisible.value = true
}

async function handleConfirmViolationSubmit(payload) {
  if (!currentRecord.value) return
  try {
    const updateData = {
      status: '违停',
      second_image_path: payload.second_image_path
    }
    if (payload.notes) {
      updateData.notes = payload.notes
    }
    await updateRecord(currentRecord.value.id, updateData)
    ElMessage.success('已确认为违停并上传照片')
    load()
  } finally {
    currentRecord.value = null
  }
}

async function confirmMoved(rec) {
  await updateRecord(rec.id, { status: '已挪车' })
  ElMessage.success('已确认为已挪车')
  load()
}

async function handleDelete(rec) {
  await ElMessageBox.confirm(`确认删除此记录？`, '删除确认', { type: 'warning' })
  await deleteRecord(rec.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<style scoped>
.app-layout { min-height: 100vh; background: #f5f7fa; }
.app-header {
  display: flex; align-items: center; background: #409eff; color: #fff; padding: 0 24px;
}
.header-left { display: flex; align-items: center; gap: 10px; }
.title { font-size: 18px; font-weight: bold; }
.app-main { padding: 20px; }
.record-card { margin-bottom: 8px; }
.photos { display: flex; gap: 16px; margin-top: 12px; }
.photo-item { display: flex; flex-direction: column; align-items: center; }
.photo-label { font-size: 12px; color: #666; margin-bottom: 4px; }
.photo { width: 160px; height: 120px; border-radius: 4px; }
.actions { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
</style>
