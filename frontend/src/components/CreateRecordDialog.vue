<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="$emit('update:modelValue', $event)"
    title="上传违停照片"
    width="560px"
    @close="resetForm"
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

      <el-form-item v-if="ocrLoading" label="识别状态">
        <el-text type="info">OCR识别中，请稍候...</el-text>
      </el-form-item>

      <el-form-item v-if="ocrLines.length > 0" label="识别文字">
        <div style="width:100%">
          <el-input
            :model-value="ocrLines.join('\n')"
            type="textarea"
            :rows="6"
            readonly
          />
          <el-button class="copy-ocr-btn" size="small" @click="copyOCRText">
            复制识别文字
          </el-button>
        </div>
      </el-form-item>

      <el-form-item label="车牌号" prop="plate_number">
        <el-input v-model="form.plate_number" placeholder="例：粤A12345" clearable />
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
import { Plus } from '@element-plus/icons-vue'
import { createRecord, runOCR } from '@/api/index.js'

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

const form = reactive({
  plate_number: '',
  image_path: '',
  parking_time: '',
  notes: ''
})

const rules = {
  plate_number: [{ required: true, message: '请输入车牌号', trigger: 'blur' }],
  image_path: [{ required: true, message: '请上传照片', trigger: 'change' }]
}

const PROVINCE_CHARS = '京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼'
const PLATE_RE = new RegExp(`([${PROVINCE_CHARS}][A-Z][A-Z0-9]{5,6})`)
const FALLBACK_PLATE_RE = /([A-Z][A-Z0-9]{5,6})/
const TIME_RE = /(20\d{2})[-/年.](\d{1,2})[-/月.](\d{1,2})(?:日)?\s+([01]?\d|2[0-3])[:时](\d{1,2})(?:[:分](\d{1,2}))?/

async function onUploadSuccess(response, uploadFile) {
  // Go 后端响应 {"code":0,"data":{"url":"..."}}
  const url = response?.url || response?.data?.url || (response?.data && response.data)
  if (url) {
    form.image_path = url
    ocrLines.value = []
    if (uploadFile) {
      uploadFile.url = url
    }
    ElMessage.info('上传成功，正在识别中...')
    if (uploadFile?.raw instanceof Blob) {
      await runOCRImage(uploadFile.raw)
    } else {
      ElMessage.warning('无法读取上传文件，未执行 OCR')
    }
  } else {
    ElMessage.error('上传失败，请重试')
  }
}

function onUploadError() {
  ElMessage.error('上传失败，请重试')
}

function onUploadExceed() {
  ElMessage.warning('仅允许上传一张图片，请先移除当前图片后再上传')
}

function handleUploadRemove() {
  form.image_path = ''
  ocrLines.value = []
  previewImageUrl.value = ''
  previewVisible.value = false
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
  ocrLoading.value = true
  try {
    const result = await runOCR(imageInput)
    ocrLines.value = normalizeOCRResult(result)
    const filled = applyOCRAutoFill(ocrLines.value)

    if (ocrLines.value.length > 0) {
      if (filled.plate_number || filled.parking_time) {
        ElMessage.success('识别成功，已自动回填表单')
      } else {
        ElMessage.success('识别成功，可直接复制文字')
      }
    } else {
      ElMessage.warning('未识别到文字，请重试或手动输入')
    }
  } catch {
    ElMessage.warning('OCR识别失败，请重试')
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

function applyOCRAutoFill(lines) {
  const text = Array.isArray(lines) ? lines.join('\n') : ''
  const compact = text.replace(/[\s·•，,。]/g, '').toUpperCase()
  const plateMatch = compact.match(PLATE_RE) || compact.match(FALLBACK_PLATE_RE)
  const timeMatch = text.match(TIME_RE)

  const filled = {
    plate_number: '',
    parking_time: ''
  }

  if (plateMatch?.[1]) {
    form.plate_number = plateMatch[1]
    filled.plate_number = plateMatch[1]
  }

  const parsedTime = toDateTimeString(timeMatch)
  if (parsedTime) {
    form.parking_time = parsedTime
    filled.parking_time = parsedTime
  }

  return filled
}

function toDateTimeString(match) {
  if (!match) return ''

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const hour = Number(match[4])
  const minute = Number(match[5])
  const second = Number(match[6] || 0)

  const date = new Date(year, month - 1, day, hour, minute, second)
  if (
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day ||
    date.getHours() !== hour ||
    date.getMinutes() !== minute
  ) {
    return ''
  }

  const pad = n => String(n).padStart(2, '0')
  return `${year}-${pad(month)}-${pad(day)} ${pad(hour)}:${pad(minute)}:${pad(second)}`
}

async function handleSubmit() {
  await formRef.value.validate()
  submitting.value = true
  try {
    await createRecord({ ...form })
    ElMessage.success('上传成功')
    emit('update:modelValue', false)
    emit('created')
    resetForm()
  } finally {
    submitting.value = false
  }
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
.copy-ocr-btn { margin-top: 8px; }
.preview-image {
  display: block;
  width: 100%;
  max-height: 75vh;
  object-fit: contain;
}
</style>
