<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import type { FormInstance, FormRules } from 'element-plus';
import type { FarmTask, Machine } from '../types/domain';

const props = defineProps<{
  visible: boolean;
  task: FarmTask | null;
  machines: Machine[];
  busy: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void;
  (e: 'confirm', payload: { taskId: string; plannedWindow: string; machineCode: string }): void;
}>();

const formRef = ref<FormInstance>();
const form = reactive({ plannedWindow: '', machineCode: '' });

const rules: FormRules = {
  plannedWindow: [{ required: true, message: '请填写新的作业时间窗', trigger: 'blur' }],
};

// 弹窗打开时用任务当前时间窗与推荐农机预填。
watch(
  () => props.visible,
  (visible) => {
    if (visible && props.task) {
      form.plannedWindow = props.task.plannedWindow;
      form.machineCode = '';
      formRef.value?.clearValidate();
    }
  },
);

const idleMachines = props.machines;

const close = () => emit('update:visible', false);

const handleConfirm = async () => {
  if (!props.task || !formRef.value) return;
  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) return;
  emit('confirm', {
    taskId: props.task.id,
    plannedWindow: form.plannedWindow.trim(),
    machineCode: form.machineCode,
  });
};
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="任务改期"
    width="420px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
  >
    <div v-if="task" class="mb-3 rounded bg-slate-50 p-2 text-sm text-slate-600">
      {{ task.type }} · {{ task.field }} · 当前农机 {{ task.assignedMachine || task.recommendedMachine }}
    </div>
    <el-form ref="formRef" :model="form" :rules="rules" label-width="92px">
      <el-form-item label="新时间窗" prop="plannedWindow">
        <el-input v-model="form.plannedWindow" placeholder="如：明日 08:00-16:00" clearable />
      </el-form-item>
      <el-form-item label="新农机">
        <el-select v-model="form.machineCode" placeholder="留空则自动选空闲农机" clearable class="w-full">
          <el-option
            v-for="m in idleMachines"
            :key="m.code"
            :label="`${m.code} ${m.name}（空闲）`"
            :value="m.code"
          />
        </el-select>
      </el-form-item>
    </el-form>
    <p class="text-xs text-slate-400">仅可选择空闲农机；指定农机被占用时会自动改派另一台空闲农机。</p>
    <template #footer>
      <el-button @click="close">取消</el-button>
      <el-button type="primary" :loading="busy" @click="handleConfirm">确认改期</el-button>
    </template>
  </el-dialog>
</template>
