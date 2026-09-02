<script lang="ts" setup>
import type { AuditLogRecord } from '#/api/audit';

import { computed, onMounted, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElEmpty,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import { getAuditLogsApi } from '#/api/audit';

defineOptions({ name: 'AuditLogs' });

const loading = ref(false);
const loadError = ref('');
const records = ref<AuditLogRecord[]>([]);

const staffCount = computed(
  () => records.value.filter((item) => item.actor_type === 'staff').length,
);

function actorLabel(value: AuditLogRecord['actor_type']) {
  return { anonymous: '未登录', parent: '家长', staff: '员工', system: '系统' }[
    value
  ];
}

function formatMetadata(value: unknown) {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch {
    return '—';
  }
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const result = await getAuditLogsApi({ limit: 100 });
    records.value = result.items;
  } catch {
    records.value = [];
    loadError.value = '审计日志加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

onMounted(loadData);
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">可追溯 · 关键操作</p>
        <h1 class="sprout-page-title">操作审计</h1>
        <p class="sprout-page-description">
          留存接送、请假、餐食、作业和每日总结的关键变更，便于异常核对与责任追溯。
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

    <div class="mb-4 grid gap-4 md:grid-cols-2">
      <div class="sprout-metric-card">
        <div class="text-sm text-gray-500">当前记录</div>
        <div class="sprout-metric-value">{{ records.length }}</div>
        <div class="sprout-metric-note">最近 100 条关键操作</div>
      </div>
      <div class="sprout-metric-card tone-blue">
        <div class="text-sm text-gray-500">员工操作</div>
        <div class="sprout-metric-value">{{ staffCount }}</div>
        <div class="sprout-metric-note">包括教师和管理人员</div>
      </div>
    </div>

    <ElCard class="sprout-table-card" shadow="never">
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="records" stripe>
          <ElTableColumn label="时间" min-width="170" prop="created_at" />
          <ElTableColumn label="操作者" width="110">
            <template #default="{ row }">
              <ElTag>{{
                actorLabel((row as AuditLogRecord).actor_type)
              }}</ElTag>
              <span class="ml-1 text-gray-500"
                >#{{ (row as AuditLogRecord).actor_id || '—' }}</span
              >
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" min-width="220" prop="action" />
          <ElTableColumn label="对象" min-width="180">
            <template #default="{ row }">
              {{ (row as AuditLogRecord).resource_type }} #{{
                (row as AuditLogRecord).resource_id || '—'
              }}
            </template>
          </ElTableColumn>
          <ElTableColumn
            label="请求 ID"
            min-width="150"
            prop="request_id"
            show-overflow-tooltip
          />
          <ElTableColumn label="附加信息" min-width="220" show-overflow-tooltip>
            <template #default="{ row }">
              {{ formatMetadata((row as AuditLogRecord).metadata) }}
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="暂时没有审计记录" :image-size="90" />
          </template>
        </ElTable>
      </div>
    </ElCard>
  </div>
</template>
