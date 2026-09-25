<script setup lang="ts">
import { computed, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { STATUS_COLORS } from '../constants/app.constants';
import { AppException } from '../errors/AppException';
import { cancelTask, dispatchTask, rescheduleTask } from '../services/storage.service';
import type { FarmTask, Machine } from '../types/domain';

const props = defineProps<{ tasks: FarmTask[]; machines: Machine[] }>();
const emit = defineEmits<{ (e: 'changed'): void }>();

// 当前正在操作的任务（按钮 loading）与本次操作的临时失败原因。
const actingId = ref('');
const localErrors = reactive<Record<string, string>>({});

// 改期弹窗状态。
const dialogVisible = ref(false);
const rescheduleForm = reactive({ taskId: '', targetMachine: '', plannedWindow: '' });
const submitting = ref(false);

const idleMachines = computed(() => props.machines.filter((m) => m.status === '空闲'));

const errorMessageOf = (err: unknown): string =>
  err instanceof AppException ? err.message : '操作失败，请稍后重试';

// 操作失败：消息条提示 + 卡片内联展示，并刷新一次以同步后端持久化的失败原因。
const handleActionError = async (taskId: string, err: unknown) => {
  const message = errorMessageOf(err);
  localErrors[taskId] = message;
  ElMessage.error(message);
  emit('changed');
};

const handleDispatch = async (task: FarmTask) => {
  actingId.value = task.id;
  localErrors[task.id] = '';
  try {
    const result = await dispatchTask(task.id);
    ElMessage.success(`${result.message}：${result.assignedMachine}`);
    emit('changed');
  } catch (err) {
    await handleActionError(task.id, err);
  } finally {
    actingId.value = '';
  }
};

const openReschedule = (task: FarmTask) => {
  localErrors[task.id] = '';
  rescheduleForm.taskId = task.id;
  // 默认选当前推荐农机（若空闲），否则留空由系统自动改派。
  const recommendedIdle = idleMachines.value.find((m) => m.code === task.recommendedMachine);
  rescheduleForm.targetMachine = recommendedIdle ? recommendedIdle.code : '';
  rescheduleForm.plannedWindow = task.plannedWindow;
  dialogVisible.value = true;
};

const handleRescheduleConfirm = async () => {
  const taskId = rescheduleForm.taskId;
  submitting.value = true;
  localErrors[taskId] = '';
  try {
    const result = await rescheduleTask(
      taskId,
      rescheduleForm.targetMachine,
      rescheduleForm.plannedWindow,
    );
    ElMessage.success(`${result.message}：${result.assignedMachine}`);
    dialogVisible.value = false;
    emit('changed');
  } catch (err) {
    // 改期失败时原任务保持不变（后端事务保证），原因内联显示。
    localErrors[taskId] = errorMessageOf(err);
    ElMessage.error(localErrors[taskId]);
    emit('changed');
  } finally {
    submitting.value = false;
  }
};

const handleCancel = async (task: FarmTask) => {
  try {
    await ElMessageBox.confirm(
      `确定撤单「${task.type} · ${task.field}」吗？农机将在无其他在途任务时回到空闲。`,
      '撤单确认',
      { type: 'warning', confirmButtonText: '撤单', cancelButtonText: '再想想' },
    );
  } catch {
    return; // 用户取消
  }
  actingId.value = task.id;
  localErrors[task.id] = '';
  try {
    const result = await cancelTask(task.id);
    ElMessage.success(result.message);
    emit('changed');
  } catch (err) {
    await handleActionError(task.id, err);
  } finally {
    actingId.value = '';
  }
};
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <h2 class="mb-3 text-lg font-black">作业任务调度</h2>
    <div class="grid gap-3">
      <article v-for="task in tasks" :key="task.id" class="rounded-md border border-slate-200 p-3">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <strong>{{ task.type }} · {{ task.field }}</strong>
              <el-tag :type="STATUS_COLORS[task.status]" size="small">{{ task.status }}</el-tag>
              <el-tag size="small" type="info">优先级 {{ task.priority }}</el-tag>
            </div>
            <p class="mt-1 text-sm text-slate-600">
              {{ task.areaMu }} 亩 · 预计 {{ task.estimatedHours }} 小时 · {{ task.plannedWindow }}
            </p>
            <p class="mt-1 text-sm text-slate-600">
              推荐 {{ task.recommendedMachine }} / {{ task.recommendedDriver }}
            </p>
            <p class="mt-1 text-sm font-semibold" :class="task.assignedMachine ? 'text-emerald-700' : 'text-slate-400'">
              最终农机：{{ task.assignedMachine || '尚未占用' }}
            </p>
            <!-- 失败原因：后端持久化的 failReason 或本次操作的临时错误 -->
            <el-alert
              v-if="task.failReason || localErrors[task.id]"
              class="mt-2"
              :title="localErrors[task.id] || task.failReason"
              type="error"
              show-icon
              :closable="false"
            />
          </div>
          <div class="flex shrink-0 flex-col gap-2">
            <el-button
              v-if="task.status === '待派单'"
              size="small"
              type="primary"
              :loading="actingId === task.id"
              @click="handleDispatch(task)"
            >
              一键派单
            </el-button>
            <el-button
              v-if="task.status === '已派单'"
              size="small"
              type="warning"
              :loading="actingId === task.id"
              @click="openReschedule(task)"
            >
              改期
            </el-button>
            <el-button
              v-if="task.status === '待派单' || task.status === '已派单'"
              size="small"
              type="danger"
              plain
              :loading="actingId === task.id"
              @click="handleCancel(task)"
            >
              撤单
            </el-button>
          </div>
        </div>
      </article>
    </div>

    <!-- 改期：先选好新的空闲农机（或留空由系统自动改派），可同时调整作业时间 -->
    <el-dialog v-model="dialogVisible" title="任务改期" width="26rem">
      <el-form label-width="5.5rem">
        <el-form-item label="新农">
          <el-select v-model="rescheduleForm.targetMachine" placeholder="自动选择空闲农机" clearable class="w-full">
            <el-option
              v-for="m in idleMachines"
              :key="m.code"
              :label="`${m.code} ${m.name}`"
              :value="m.code"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="作业时间">
          <el-input v-model="rescheduleForm.plannedWindow" placeholder="如：明日 07:30-14:00" />
        </el-form-item>
      </el-form>
      <p class="text-xs text-slate-500">新农选择在事务内完成切换，原任务不会丢失；没有空闲农机会直接提示失败原因。</p>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleRescheduleConfirm">确认改期</el-button>
      </template>
    </el-dialog>
  </section>
</template>
