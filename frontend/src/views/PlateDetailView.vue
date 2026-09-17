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
          <span>{{ plate }} 的违停记录（共 {{ total }} 条）</span>
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
              <el-image
                :src="rec.image_path"
                fit="cover"
                lazy
                :preview-src-list="previewList(rec)"
                :initial-index="0"
                class="photo"
              />
            </div>
            <div v-if="rec.second_image_path" class="photo-item">
              <div class="photo-label">第二张照片</div>
              <el-image
                :src="rec.second_image_path"
                fit="cover"
                lazy
                :preview-src-list="previewList(rec)"
                :initial-index="1"
                class="photo"
              />
            </div>
          </div>
          <div class="actions">
            <template v-if="rec.status === '待处理'">
              <el-button type="warning" size="small" @click="openRemindDialog(rec)">提醒移车</el-button>
            </template>
            <template v-else-if="rec.status === '待确认'">
              <el-button type="primary" size="small" plain @click="openEditReminderDialog(rec)">修改提醒时间</el-button>
              <el-button type="danger" size="small" @click="confirmViolation(rec)">确认违停</el-button>
              <el-button type="success" size="small" @click="confirmMoved(rec)">确认已挪车</el-button>
            </template>
            <el-button type="danger" size="small" plain @click="handleDelete(rec)">删除</el-button>
          </div>
          <el-divider />
        </div>
        <div class="pagination">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next"
            @change="load"
          />
        </div>
      </el-card>
    </el-main>

    <ConfirmViolationDialog
      :model-value="confirmDialogVisible"
      @update:model-value="confirmDialogVisible = $event"
      @confirmed="handleConfirmViolationSubmit"
    />

    <el-dialog
      v-model="reminderDialogVisible"
      :title="reminderDialogTitle"
      width="420px"
    >
      <el-form label-width="90px">
        <el-form-item label="提醒时间">
          <el-date-picker
            v-model="reminderForm.reminder_time"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="请选择提醒时间"
            style="width:100%"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="reminderDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitReminderDialog">确定</el-button>
      </template>
    </el-dialog>
  </el-container>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Van } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getPlateDetail, updateRecord, deleteRecord } from '@/api/index.js'
import { nowDateTimeString } from '@/utils/datetime.js'
import StatusTag from '@/components/StatusTag.vue'
import ConfirmViolationDialog from '@/components/ConfirmViolationDialog.vue'

const route = useRoute()
const router = useRouter()
const plate = route.params.plate
const records = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const confirmDialogVisible = ref(false)
const currentRecord = ref(null)
const reminderDialogVisible = ref(false)
const reminderDialogMode = ref('remind')
const reminderTargetRecord = ref(null)
const reminderForm = reactive({
  reminder_time: ''
})

const reminderDialogTitle = computed(() => (
  reminderDialogMode.value === 'edit' ? '修改提醒时间' : '提醒移车'
))

// 同一记录的两张照片放在同一个预览列表里，便于左右对比
function previewList(rec) {
  return [rec.image_path, rec.second_image_path].filter(Boolean)
}

async function load() {
  loading.value = true
  try {
    const res = await getPlateDetail(plate, { page: page.value, page_size: pageSize.value })
    // 删除记录后当前页可能为空，自动回退到上一页
    if ((res.records || []).length === 0 && res.total > 0 && page.value > 1) {
      page.value -= 1
      return load()
    }
    records.value = res.records || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

function openRemindDialog(rec) {
  reminderDialogMode.value = 'remind'
  reminderTargetRecord.value = rec
  reminderForm.reminder_time = rec.reminder_time || nowDateTimeString()
  reminderDialogVisible.value = true
}

function openEditReminderDialog(rec) {
  reminderDialogMode.value = 'edit'
  reminderTargetRecord.value = rec
  reminderForm.reminder_time = rec.reminder_time || nowDateTimeString()
  reminderDialogVisible.value = true
}

async function submitReminderDialog() {
  const rec = reminderTargetRecord.value
  if (!rec) return
  if (!reminderForm.reminder_time) {
    ElMessage.warning('请选择提醒时间')
    return
  }

  if (reminderDialogMode.value === 'edit') {
    await updateRecord(rec.id, { status: rec.status, reminder_time: reminderForm.reminder_time })
    ElMessage.success('提醒时间已修改')
  } else {
    await updateRecord(rec.id, { status: '待确认', reminder_time: reminderForm.reminder_time })
    ElMessage.success('已提醒移车，状态变更为待确认')
  }

  reminderDialogVisible.value = false
  reminderTargetRecord.value = null
  await load()
}

async function confirmViolation(rec) {
  currentRecord.value = rec
  confirmDialogVisible.value = true
}

async function handleConfirmViolationSubmit(payload) {
  if (!currentRecord.value) return
  try {
    const note = String(payload.notes || '').trim()
    const updateData = {
      status: '违停',
      second_image_path: payload.second_image_path,
      second_check_time: nowDateTimeString()
    }
    if (note) {
      // 追加而不是覆盖上传时填写的备注，避免信息丢失
      const original = String(currentRecord.value.notes || '').trim()
      updateData.notes = original ? `${original}\n${note}` : note
    }
    await updateRecord(currentRecord.value.id, updateData)
    ElMessage.success('已确认为违停并上传照片')
    load()
  } finally {
    currentRecord.value = null
  }
}

async function confirmMoved(rec) {
  try {
    await ElMessageBox.confirm(
      '确认该车辆已挪车？确认后状态将变更为「已挪车」，无法再修改。',
      '确认已挪车',
      { type: 'warning', confirmButtonText: '确认已挪车', cancelButtonText: '取消' }
    )
  } catch {
    return // 用户取消
  }

  await updateRecord(rec.id, {
    status: '已挪车',
    second_check_time: nowDateTimeString()
  })
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
.photos { display: flex; gap: 16px; margin-top: 12px; flex-wrap: wrap; }
.photo-item { display: flex; flex-direction: column; align-items: center; }
.photo-label { font-size: 12px; color: #666; margin-bottom: 4px; }
.photo { width: 160px; height: 120px; border-radius: 4px; }
.actions { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
.pagination { margin-top: 16px; display: flex; justify-content: flex-end; }

@media (max-width: 768px) {
  .app-main { padding: 12px; }
  .title { font-size: 16px; }
  .photos { gap: 8px; }
  .photo { width: 120px; height: 90px; }
}
</style>
