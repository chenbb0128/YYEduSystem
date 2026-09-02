<script lang="ts" setup>
import type { LeaveRequestRecord } from '#/api/leave-requests';
import type { StudentRecord } from '#/api/master-data';

import { onMounted, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElEmpty,
  ElMessage,
  ElMessageBox,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import {
  getLeaveRequestsApi,
  reviewLeaveRequestApi,
} from '#/api/leave-requests';
import { getStudentsApi } from '#/api/master-data';

defineOptions({ name: 'LeaveRequests' });

const loading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const requests = ref<LeaveRequestRecord[]>([]);
const students = ref<StudentRecord[]>([]);

function studentName(id: number) {
  return students.value.find((item) => item.id === id)?.name || `学生 ${id}`;
}

function statusLabel(status: LeaveRequestRecord['status']) {
  return {
    approved: '已同意',
    cancelled: '已撤销',
    pending: '待确认',
    rejected: '未同意',
  }[status];
}

function statusType(status: LeaveRequestRecord['status']) {
  if (status === 'approved') return 'success';
  if (status === 'rejected') return 'danger';
  if (status === 'pending') return 'warning';
  return 'info';
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const [requestResult, studentResult] = await Promise.all([
      getLeaveRequestsApi(),
      getStudentsApi(),
    ]);
    requests.value = requestResult.items;
    students.value = studentResult.items;
  } catch {
    requests.value = [];
    loadError.value = '请假申请加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

async function review(
  row: LeaveRequestRecord,
  status: 'approved' | 'rejected',
) {
  try {
    await ElMessageBox.confirm(
      `${status === 'approved' ? '同意' : '不予同意'} ${studentName(row.student_id)} ${row.leave_date} 的请假申请？`,
      '确认审核',
      { type: status === 'approved' ? 'success' : 'warning' },
    );
  } catch {
    return;
  }
  submitting.value = true;
  try {
    await reviewLeaveRequestApi(row.id, { status });
    ElMessage.success(
      status === 'approved' ? '请假申请已同意' : '请假申请已拒绝',
    );
    await loadData();
  } finally {
    submitting.value = false;
  }
}

onMounted(loadData);
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">家校协同 · 请假申请</p>
        <h1 class="sprout-page-title">请假审核</h1>
        <p class="sprout-page-description">
          家长提交的请假申请在这里确认，审核结果会同步回家长端。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton :loading="loading" @click="loadData">刷新</ElButton>
      </div>
    </div>

    <ElAlert
      v-if="loadError"
      class="mb-4"
      :closable="false"
      show-icon
      :title="loadError"
      type="error"
    />

    <ElCard class="sprout-table-card" shadow="never">
      <div class="sprout-table-toolbar mb-4">
        <div>
          <h2 class="sprout-section-title">请假申请列表</h2>
          <p class="sprout-section-caption">
            优先处理“待确认”申请，结果会通知提交人。
          </p>
        </div>
        <ElTag class="sprout-status" type="warning">
          待确认
          {{ requests.filter((item) => item.status === 'pending').length }}
        </ElTag>
      </div>
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="requests" stripe>
          <ElTableColumn label="日期" width="125" prop="leave_date" />
          <ElTableColumn label="学生" min-width="120">
            <template #default="{ row }">
              {{ studentName((row as LeaveRequestRecord).student_id) }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="提交人" width="90">
            <template #default="{ row }">
              {{
                (row as LeaveRequestRecord).submitted_by_type === 'parent'
                  ? '家长'
                  : '老师'
              }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="请假原因" min-width="220" prop="reason" />
          <ElTableColumn label="状态" width="100">
            <template #default="{ row }">
              <ElTag :type="statusType((row as LeaveRequestRecord).status)">
                {{ statusLabel((row as LeaveRequestRecord).status) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="老师备注" min-width="160" prop="teacher_note" />
          <ElTableColumn label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <template v-if="(row as LeaveRequestRecord).status === 'pending'">
                <ElButton
                  :loading="submitting"
                  link
                  type="success"
                  @click="review(row as LeaveRequestRecord, 'approved')"
                >
                  同意 </ElButton
                ><ElButton
                  :loading="submitting"
                  link
                  type="danger"
                  @click="review(row as LeaveRequestRecord, 'rejected')"
                >
                  拒绝
                </ElButton> </template
              ><span v-else class="text-gray-400">已处理</span>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="暂时没有请假申请" :image-size="90" />
          </template>
        </ElTable>
      </div>
    </ElCard>
  </div>
</template>
