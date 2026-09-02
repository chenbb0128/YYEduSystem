<script lang="ts" setup>
import type {
  DeliveryStatus,
  NotificationDeliveryLogRecord,
  NotificationMessageKind,
} from '#/api/notification-delivery';

import { computed, onMounted, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElEmpty,
  ElMessage,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import {
  getNotificationDeliveryLogsApi,
  retryNotificationApi,
} from '#/api/notification-delivery';

defineOptions({ name: 'NotificationDeliveryLogs' });

const loading = ref(false);
const retryingID = ref(0);
const loadError = ref('');
const status = ref<'' | DeliveryStatus>('');
const messageKind = ref<'' | NotificationMessageKind>('');
const records = ref<NotificationDeliveryLogRecord[]>([]);

const failedCount = computed(
  () => records.value.filter((item) => item.status === 'failed').length,
);

function statusLabel(value: DeliveryStatus) {
  return {
    failed: '发送失败',
    pending: '等待发送',
    sent: '已发送',
    skipped: '已跳过',
  }[value];
}

function statusType(value: DeliveryStatus) {
  if (value === 'sent') return 'success';
  if (value === 'failed') return 'danger';
  if (value === 'pending') return 'warning';
  return 'info';
}

function kindLabel(value: string) {
  return (
    {
      homework: '作业',
      leave: '请假',
      meal: '餐食',
      pickup: '接送',
      summary: '每日总结',
    }[value] || value
  );
}

function businessStatusLabel(value: string) {
  return (
    {
      pending: '业务待处理',
      sent: '业务已处理',
      failed: '业务处理失败',
    }[value] ||
    value ||
    '未记录'
  );
}

function businessStatusType(value: string) {
  if (value === 'failed') return 'danger';
  if (value === 'pending') return 'warning';
  return 'success';
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const result = await getNotificationDeliveryLogsApi(
      status.value || messageKind.value
        ? {
            ...(status.value ? { status: status.value } : {}),
            ...(messageKind.value ? { message_kind: messageKind.value } : {}),
          }
        : undefined,
    );
    records.value = result.items;
  } catch {
    records.value = [];
    loadError.value = '通知投递日志加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

async function retry(row: NotificationDeliveryLogRecord) {
  if (retryingID.value) return;
  retryingID.value = row.notification_id;
  try {
    await retryNotificationApi(row.notification_id);
    ElMessage.success('已加入通知重试队列');
    await loadData();
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '通知重试失败，请稍后再试',
    );
  } finally {
    retryingID.value = 0;
  }
}

onMounted(loadData);
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">可靠通知 · 异常闭环</p>
        <h1 class="sprout-page-title">通知投递</h1>
        <p class="sprout-page-description">
          微信通知失败不会阻断接送、作业和餐食流程，可在这里核对每位家长的投递结果并重试。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElSelect
          v-model="status"
          clearable
          placeholder="全部状态"
          style="width: 140px"
          @change="loadData"
        >
          <ElOption label="发送失败" value="failed" />
          <ElOption label="等待发送" value="pending" />
          <ElOption label="已发送" value="sent" />
          <ElOption label="已跳过" value="skipped" />
        </ElSelect>
        <ElSelect
          v-model="messageKind"
          clearable
          placeholder="全部通知类型"
          style="width: 140px"
          @change="loadData"
        >
          <ElOption label="接送" value="pickup" />
          <ElOption label="餐食" value="meal" />
          <ElOption label="作业" value="homework" />
          <ElOption label="请假" value="leave" />
          <ElOption label="每日总结" value="summary" />
        </ElSelect>
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

    <div class="mb-4 grid gap-4 md:grid-cols-3">
      <div class="sprout-metric-card">
        <div class="text-sm text-gray-500">当前日志</div>
        <div class="sprout-metric-value">{{ records.length }}</div>
        <div class="sprout-metric-note">按家长逐条记录</div>
      </div>
      <div class="sprout-metric-card tone-sun">
        <div class="text-sm text-gray-500">当前失败</div>
        <div class="sprout-metric-value">{{ failedCount }}</div>
        <div class="sprout-metric-note">可人工发起重试</div>
      </div>
      <div class="sprout-metric-card tone-blue">
        <div class="text-sm text-gray-500">业务保障</div>
        <div class="sprout-metric-value">✓</div>
        <div class="sprout-metric-note">消息失败不影响业务记录</div>
      </div>
    </div>

    <ElCard class="sprout-table-card" shadow="never">
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="records" stripe>
          <ElTableColumn label="通知" min-width="220">
            <template #default="{ row }">
              <div>
                <div class="font-medium">
                  {{
                    (row as NotificationDeliveryLogRecord).notification_title ||
                    `通知 #${(row as NotificationDeliveryLogRecord).notification_id}`
                  }}
                </div>
                <div class="text-xs text-gray-400">
                  #{{ (row as NotificationDeliveryLogRecord).notification_id }}
                </div>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="孩子" width="120">
            <template #default="{ row }">
              <span>
                {{
                  (row as NotificationDeliveryLogRecord).student_name ||
                  `学生 #${(row as NotificationDeliveryLogRecord).student_id}`
                }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn
            label="家长账号"
            width="110"
            prop="parent_account_id"
          />
          <ElTableColumn label="类型" width="110">
            <template #default="{ row }">
              {{
                kindLabel((row as NotificationDeliveryLogRecord).message_kind)
              }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="110">
            <template #default="{ row }">
              <ElTag
                :type="
                  statusType((row as NotificationDeliveryLogRecord).status)
                "
              >
                {{ statusLabel((row as NotificationDeliveryLogRecord).status) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="业务状态" width="125">
            <template #default="{ row }">
              <ElTag
                :type="
                  businessStatusType(
                    (row as NotificationDeliveryLogRecord).notification_status,
                  )
                "
              >
                {{
                  businessStatusLabel(
                    (row as NotificationDeliveryLogRecord).notification_status,
                  )
                }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="尝试次数" width="95" prop="attempts" />
          <ElTableColumn
            label="最近错误"
            min-width="220"
            prop="delivery_error"
            show-overflow-tooltip
          />
          <ElTableColumn label="更新时间" min-width="180" prop="updated_at" />
          <ElTableColumn label="操作" width="110" fixed="right">
            <template #default="{ row }">
              <ElButton
                v-if="
                  (row as NotificationDeliveryLogRecord).status === 'failed'
                "
                v-access:code="'notification:retry'"
                link
                type="primary"
                :loading="
                  retryingID ===
                  (row as NotificationDeliveryLogRecord).notification_id
                "
                @click="retry(row as NotificationDeliveryLogRecord)"
              >
                重试
              </ElButton>
              <span v-else class="text-gray-400">—</span>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="暂时没有通知投递记录" :image-size="90" />
          </template>
        </ElTable>
      </div>
    </ElCard>
  </div>
</template>
