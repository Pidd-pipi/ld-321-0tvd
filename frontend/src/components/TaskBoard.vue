<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import TaskCard from './TaskCard.vue';
import TaskRescheduleDialog from './TaskRescheduleDialog.vue';
import { cancelTask, dispatchTask, rescheduleTask } from '../services/storage.service';
import type { FarmTask, Machine, TaskActionResult } from '../types/domain';

const props = defineProps<{
  tasks: FarmTask[];
  machines: Machine[];
}>();

const emit = defineEmits<{ (e: 'refresh'): void }>();

const busyId = ref('');
const dialogVisible = ref(false);
const dialogTask = ref<FarmTask | null>(null);

// 操作成功后把后端返回的最终农机/状态回填到任务卡。
const applyResult = (task: FarmTask, result: TaskActionResult) => {
  task.status = result.status ?? task.status;
  if (result.assignedMachine !== undefined) task.assignedMachine = result.assignedMachine;
  if (result.assignedDriver !== undefined) task.assignedDriver = result.assignedDriver;
  if (result.plannedWindow !== undefined) task.plannedWindow = result.plannedWindow;
  task.failureReason = result.failureReason ?? '';
};

const showFailure = (task: FarmTask, err: unknown) => {
  const reason = err instanceof Error ? err.message : '操作失败';
  task.failureReason = reason;
  ElMessage.error(reason);
};

const handleDispatch = async (task: FarmTask) => {
  busyId.value = task.id;
  try {
    const result = await dispatchTask(task.id);
    applyResult(task, result);
    ElMessage.success(result.message);
    emit('refresh');
  } catch (err) {
    showFailure(task, err);
    emit('refresh');
  } finally {
    busyId.value = '';
  }
};

const openReschedule = (task: FarmTask) => {
  dialogTask.value = task;
  dialogVisible.value = true;
};

const handleRescheduleConfirm = async (payload: {
  taskId: string;
  plannedWindow: string;
  machineCode: string;
}) => {
  const task = props.tasks.find((t) => t.id === payload.taskId);
  if (!task) return;
  busyId.value = task.id;
  try {
    const result = await rescheduleTask(payload.taskId, payload.plannedWindow, payload.machineCode);
    applyResult(task, result);
    dialogVisible.value = false;
    ElMessage.success(result.message);
    emit('refresh');
  } catch (err) {
    showFailure(task, err);
    emit('refresh');
  } finally {
    busyId.value = '';
  }
};

const handleCancel = async (task: FarmTask) => {
  try {
    await ElMessageBox.confirm(
      `确认撤单「${task.type} · ${task.field}」？撤单后农机将在无其他在途任务时回到空闲。`,
      '撤单确认',
      { type: 'warning', confirmButtonText: '撤单', cancelButtonText: '再想想' },
    );
  } catch {
    return;
  }
  busyId.value = task.id;
  try {
    const result = await cancelTask(task.id);
    applyResult(task, result);
    ElMessage.success(result.message);
    emit('refresh');
  } catch (err) {
    showFailure(task, err);
    emit('refresh');
  } finally {
    busyId.value = '';
  }
};

// 仅空闲农机可作为改期目标。
const idleMachines = props.machines.filter((m) => m.status === '空闲');
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <h2 class="mb-3 text-lg font-black">作业任务调度</h2>
    <div class="grid gap-3">
      <TaskCard
        v-for="task in tasks"
        :key="task.id"
        :task="task"
        :machines="idleMachines"
        :busy="busyId === task.id"
        @dispatch="handleDispatch"
        @reschedule="openReschedule"
        @cancel="handleCancel"
      />
    </div>

    <TaskRescheduleDialog
      v-model:visible="dialogVisible"
      :task="dialogTask"
      :machines="idleMachines"
      :busy="busyId !== ''"
      @confirm="handleRescheduleConfirm"
    />
  </section>
</template>
