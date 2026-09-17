<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="$emit('update:modelValue', $event)"
    title="上传违停照片"
    width="560px"
    @close="handleClose"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-form-item label="照片" prop="image_path">
        <el-upload
          ref="uploadRef"
          v-model:file-list="uploadFileList"
          action="/api/upload"
          name="file"
          :multiple="false"
          :limit="1"
          :on-success="onUploadSuccess"
          :on-error="onUploadError"
          :on-exceed="onUploadExceed"
          :on-preview="onUploadPreview"
          :on-remove="handleUploadRemove"
          accept="image/*"
          list-type="picture-card"
        >
          <el-icon><Plus /></el-icon>
          <template #tip>
            <div class="upload-tip">支持 JPG/PNG/BMP，最大 20MB</div>
          </template>
        </el-upload>
      </el-form-item>

      <el-form-item v-if="form.image_path" label="识别文字">
        <div class="ocr-block">
          <el-text v-if="ocrLoading" type="info">OCR 识别中，请稍候...</el-text>
          <el-input
            v-else-if="ocrLines.length > 0"
            :model-value="ocrLines.join('\n')"
            type="textarea"
            :rows="6"
            readonly
          />
          <el-text v-else type="info">未识别到文字，可点击「重新识别」或手动填写</el-text>

          <div class="ocr-actions">
            <el-button
              v-if="ocrLines.length > 0"
              size="small"
              :disabled="ocrLoading"
              @click="copyOCRText"
            >
              复制识别文字
            </el-button>
            <el-button
              size="small"
              :icon="Refresh"
              :loading="ocrLoading"
              @click="runOCRImage(form.image_path)"
            >
              重新识别
            </el-button>
          </div>
        </div>
      </el-form-item>

      <el-form-item label="车牌号" prop="plate_number">
        <el-input
          v-model="form.plate_number"
          placeholder="例：粤A12345"
          clearable
          @blur="normalizePlateInput"
        />
      </el-form-item>

      <el-form-item label="停车时间" prop="parking_time">
        <el-date-picker
          v-model="form.parking_time"
          type="datetime"
          placeholder="选择或手动输入"
          value-format="YYYY-MM-DD HH:mm:ss"
          style="width:100%"
        />
      </el-form-item>

      <el-form-item label="备注">
        <el-input v-model="form.notes" type="textarea" rows="2" placeholder="可选" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">提交</el-button>
    </template>
  </el-dialog>

  <el-dialog
    v-model="previewVisible"
    title="图片预览"
    width="840px"
    append-to-body
  >
    <img v-if="previewImageUrl" :src="previewImageUrl" class="preview-image" alt="preview" />
  </el-dialog>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import { createRecord, runOCR, discardUpload } from '@/api/index.js'
import { normalizePlate, plateValidator } from '@/utils/plate.js'

const emit = defineEmits(['update:modelValue', 'created'])
const props = defineProps({ modelValue: Boolean })

const formRef = ref(null)
const uploadRef = ref(null)
const uploadFileList = ref([])
const submitting = ref(false)
const ocrLoading = ref(false)
const ocrLines = ref([])
const previewVisible = ref(false)
const previewImageUrl = ref('')
// 本次会话已上传、但尚未被任何记录引用的图片；关闭弹窗时需要丢弃
const pendingUploadUrl = ref('')

const form = reactive({
  plate_number: '',
  image_path: '',
  parking_time: '',
  notes: ''
})

const rules = {
  plate_number: [
    { required: true, message: '请输入车牌号', trigger: 'blur' },
    { validator: plateValidator, trigger: 'blur' }
  ],
  image_path: [{ required: true, message: '请上传照片', trigger: 'change' }]
}

function normalizePlateInput() {
  form.plate_number = normalizePlate(form.plate_number)
}

async function onUploadSuccess(response, uploadFile) {
  // Go 后端响应 {"code":0,"data":{"url":"..."}}
  const url = response?.url || response?.data?.url || (response?.data && response.data)
  if (!url) {
    ElMessage.error('上传失败，请重试')
    return
  }

  form.image_path = url
  pendingUploadUrl.value = url
  ocrLines.value = []
  if (uploadFile) {
    uploadFile.url = url
  }
  // 直接复用已上传的图片路径，避免重复上传同一张图
  await runOCRImage(url)
}

function onUploadError() {
  ElMessage.error('上传失败，请重试')
}

function onUploadExceed() {
  ElMessage.warning('仅允许上传一张图片，请先移除当前图片后再上传')
}

async function handleUploadRemove() {
  form.image_path = ''
  ocrLines.value = []
  previewImageUrl.value = ''
  previewVisible.value = false
  await discardPendingUpload()
}

// 丢弃尚未被记录引用的上传图片，避免留下孤儿文件
async function discardPendingUpload() {
  const url = pendingUploadUrl.value
  pendingUploadUrl.value = ''
  if (!url) return
  try {
    await discardUpload(url)
  } catch {
    // 清理失败不影响用户操作，文件会留在上传目录
  }
}

function onUploadPreview(file) {
  const url = resolvePreviewURL(file)
  if (!url) {
    ElMessage.warning('当前图片无法预览')
    return
  }
  previewImageUrl.value = url
  previewVisible.value = true
}

function resolvePreviewURL(file) {
  if (!file) return ''
  if (file.url) return file.url
  if (file.response?.url) return file.response.url
  if (file.response?.data?.url) return file.response.data.url
  return ''
}

async function copyOCRText() {
  const text = ocrLines.value.join('\n').trim()
  if (!text) {
    ElMessage.warning('暂无可复制的识别文字')
    return
  }

  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('识别文字已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

async function runOCRImage(imageInput) {
  if (!imageInput) return
  ocrLoading.value = true
  try {
    const result = await runOCR(imageInput)
    ocrLines.value = normalizeOCRResult(result)
    const filled = applyOCRAutoFill(result)

    if (ocrLines.value.length === 0) {
      ElMessage.warning('未识别到文字，可点击「重新识别」或手动填写')
    } else if (filled.plate_number || filled.parking_time) {
      ElMessage.success('识别成功，已自动回填表单')
    } else {
      ElMessage.success('识别成功，可复制下方文字手动填写')
    }
  } catch (err) {
    // 响应拦截器已经弹出后端返回的具体原因，这里不再重复提示
    if (!err?.handled) {
      ElMessage.warning('OCR 识别失败，请重试')
    }
  } finally {
    ocrLoading.value = false
  }
}

function normalizeOCRResult(result) {
  if (!result) {
    return []
  }

  if (Array.isArray(result.lines)) {
    return result.lines
  }

  const rawText = result.raw_text || result.text || ''
  return String(rawText)
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .split('\n')
    .map(line => line.trim())
    .filter(Boolean)
}

// 车牌与时间统一以后端 OCR 结果为准，前端不再重复实现提取规则
function applyOCRAutoFill(result) {
  const filled = {
    plate_number: '',
    parking_time: ''
  }
  if (!result) return filled

  const plate = normalizePlate(result.plate_number || '')
  if (plate) {
    form.plate_number = plate
    filled.plate_number = plate
  }

  const parkingTime = String(result.parking_time || '').trim()
  if (parkingTime) {
    form.parking_time = parkingTime
    filled.parking_time = parkingTime
  }

  return filled
}

async function handleSubmit() {
  await formRef.value.validate()
  submitting.value = true
  try {
    await createRecord({ ...form, plate_number: normalizePlate(form.plate_number) })
    // 图片已被记录引用，不再是待丢弃文件
    pendingUploadUrl.value = ''
    ElMessage.success('上传成功')
    emit('update:modelValue', false)
    emit('created')
    resetForm()
  } finally {
    submitting.value = false
  }
}

async function handleClose() {
  await discardPendingUpload()
  resetForm()
}

function resetForm() {
  uploadFileList.value = []
  uploadRef.value?.clearFiles()
  form.plate_number = ''
  form.image_path = ''
  form.parking_time = ''
  form.notes = ''
  ocrLines.value = []
  ocrLoading.value = false
  previewImageUrl.value = ''
  previewVisible.value = false
  formRef.value?.resetFields()
}

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      resetForm()
    }
  }
)
</script>

<style scoped>
.upload-tip { color: #999; font-size: 12px; margin-top: 4px; }
.ocr-block { width: 100%; }
.ocr-actions { display: flex; gap: 8px; margin-top: 8px; }
.preview-image {
  display: block;
  width: 100%;
  max-height: 75vh;
  object-fit: contain;
}
</style>
