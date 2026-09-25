<script setup lang="ts">
import { computed } from 'vue';
import { STATUS_COLORS, TASK_PENDING, TASK_DISPATCHED, TASK_RESCHEDULED } from '../constants/app.constants';
import type { FarmTask, Machine } from '../types/domain';

const props = defineProps<{
  task: FarmTask;
  machines: Machine[];
  busy: boolean;
}>();

const emit = defineEmits<{
  (e: 'dispatch', task: FarmTask): void;
  (e: 'reschedule', task: FarmTask): void;
  (e: 'cancel', task: FarmTask): void;
}>();

// 改期时可选择的空闲农机。
const idleMachines = computed(() => props.machines.filter((m) => m.status === '空闲'));

const canDispatch = computed(() => props.task.status === TASK_PENDING);
const canReschedule = computed(
  () => props.task.status === TASK_DISPATCHED || props.task.status === TASK_RESCHEDULED,
);
const canCancel = computed(() => canDispatch.value || canReschedule.value);

// 最终农机优先取改派结果，没有时回退推荐农机展示。
const finalMachine = computed(
  () => props.task.assignedMachine || props.task.recommendedMachine || '-',
);
const finalDriver = computed(() => props.task.assignedDriver || props.task.recommendedDriver || '-');
</script>

<template>
  <article class="rounded-md border border-slate-200 p-3">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <strong>{{ task.type }} · {{ task.field }}</strong>
        </div>
        <p class="mt-1 text-sm text-slate-600">
          {{ task.areaMu }} 亩 · 预计 {{ task.estimatedHours }} 小时 · {{ task.plannedWindow }}
        </p>
        <p class="mt-1 text-sm text-emerald-700">
          推荐 {{ task.recommendedMachine || '-' }} / {{ task.recommendedDriver || '-' }}
        </p>
        <!-- 最终农机与状态：派单/改期后直接展示结果 -->
        <p v-if="task.assignedMachine" class="mt-1 text-sm font-semibold text-slate-800">
          最终农机：{{ finalMachine }} / 驾驶员：{{ finalDriver }}
        </p>
        <!-- 失败原因：派单/改期无空闲农机等情况直接显示在任务卡上 -->
        <el-alert
          v-if="task.failureReason"
          class="mt-2"
          :title="`失败原因：${task.failureReason}`"
          type="error"
          :closable="false"
          show-icon
        />
      </div>
      <div class="flex shrink-0 flex-col items-end gap-2">
        <el-tag :type="STATUS_COLORS[task.status]" size="small">{{ task.status }}</el-tag>
        <div class="flex flex-wrap justify-end gap-2">
          <el-button
            v-if="canDispatch"
            size="small"
            type="primary"
            :loading="busy"
            @click="emit('dispatch', task)"
          >
            一键派单
          </el-button>
          <el-button
            v-if="canReschedule"
            size="small"
            type="warning"
            :loading="busy"
            @click="emit('reschedule', task)"
          >
            改期
          </el-button>
          <el-button
            v-if="canCancel"
            size="small"
            type="danger"
            plain
            :loading="busy"
            @click="emit('cancel', task)"
          >
            撤单
          </el-button>
        </div>
      </div>
    </div>
  </article>
</template>
