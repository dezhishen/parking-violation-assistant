<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="$emit('update:modelValue', $event)"
    title="确认违停并上传现场照片"
    width="560px"
    @close="resetForm"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-form-item label="现场照片" prop="violation_image">
        <el-upload
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

      <el-form-item label="备注">
        <el-input v-model="form.notes" type="textarea" rows="2" placeholder="可选" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="danger" :loading="submitting" @click="handleSubmit">确认违停</el-button>
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
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'

const emit = defineEmits(['update:modelValue', 'confirmed'])
defineProps({ modelValue: Boolean })

const formRef = ref(null)
const submitting = ref(false)
const previewVisible = ref(false)
const previewImageUrl = ref('')

const form = reactive({
  violation_image: '',
  notes: ''
})

const rules = {
  violation_image: [{ required: true, message: '请上传现场照片', trigger: 'change' }]
}

function onUploadSuccess(response, uploadFile) {
  const url = response?.url || response?.data?.url || (response?.data && response.data)
  if (url) {
    form.violation_image = url
    if (uploadFile) {
      uploadFile.url = url
    }
    ElMessage.success('照片上传成功')
  } else {
    ElMessage.error('上传失败，请重试')
  }
}

function onUploadError() {
  ElMessage.error('上传失败，请重试')
}

function onUploadExceed() {
  ElMessage.warning('仅允许上传一张图片')
}

function handleUploadRemove() {
  form.violation_image = ''
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

function resetForm() {
  form.violation_image = ''
  form.notes = ''
  previewImageUrl.value = ''
  previewVisible.value = false
  formRef.value?.resetFields()
}

async function handleSubmit() {
  await formRef.value.validate()
  submitting.value = true
  try {
    emit('confirmed', {
      second_image_path: form.violation_image,
      notes: form.notes || undefined
    })
    emit('update:modelValue', false)
    resetForm()
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.upload-tip { color: #999; font-size: 12px; margin-top: 4px; }
.preview-image {
  display: block;
  width: 100%;
  max-height: 75vh;
  object-fit: contain;
}
</style>
