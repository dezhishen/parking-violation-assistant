<template>
  <el-tag :type="tagType" size="small">{{ status }}</el-tag>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({ status: String })

const tagType = computed(() => {
  const map = {
    '待处理': 'info',
    '待确认': 'warning',
    '违停': 'danger',
    '已挪车': 'success'
  }
  return map[props.status] || 'info'
})
</script>
