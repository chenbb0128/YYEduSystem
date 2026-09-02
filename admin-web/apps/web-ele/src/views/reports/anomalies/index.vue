<script lang="ts" setup>
import type { DailyOverviewRecord } from '#/api/reports';

import { computed, onMounted, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElDatePicker,
  ElEmpty,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import { getDailyOverviewApi } from '#/api/reports';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'AnomalyOverview' });

const selectedDate = ref(businessToday());
const loading = ref(false);
const loadError = ref('');
const overview = ref<DailyOverviewRecord | null>(null);

const resolvedPercent = computed(() => {
  const pickup = overview.value?.pickup;
  if (!pickup?.students) return 0;
  return Math.round((pickup.resolved / pickup.students) * 100);
});

const statusLabels: Record<string, string> = {
  absent: '未找到',
  abnormal: '异常',
  arrived: '已到班',
  leave: '请假',
  left: '已离班',
  not_arrived: '到班异常',
  parent_picked_up: '家长接走',
  picked_up: '校门口接到',
  planned: '待确认',
  published: '已发布',
  closed: '已结束',
  self_arrived: '自行到班',
  未生成: '未生成',
};

function statusLabel(value: string) {
  return statusLabels[value] || value;
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    overview.value = await getDailyOverviewApi({ date: selectedDate.value });
  } catch {
    overview.value = null;
    loadError.value = '异常数据加载失败，请稍后重试。';
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
        <p class="sprout-page-kicker">按日监督 · 异常优先</p>
        <h1 class="sprout-page-title">异常看板</h1>
        <p class="sprout-page-description">
          汇总接送、作业、餐食和待办申请，帮助管理端快速发现需要跟进的事项。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElDatePicker
          v-model="selectedDate"
          type="date"
          value-format="YYYY-MM-DD"
          @change="loadData"
        />
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

    <div v-if="overview" v-loading="loading">
      <div class="mb-4 grid gap-4 md:grid-cols-4">
        <div class="sprout-metric-card">
          <div class="sprout-metric-label">接送完成度</div>
          <div class="sprout-metric-value">
            {{ resolvedPercent }}<small>%</small>
          </div>
          <div class="sprout-metric-note">
            {{ overview.pickup.resolved }}/{{ overview.pickup.students }}
            人已处理
          </div>
        </div>
        <div class="sprout-metric-card tone-danger">
          <div class="sprout-metric-label">接送异常</div>
          <div class="sprout-metric-value">
            {{
              overview.pickup.photo_missing +
              (overview.pickup.statuses.absent || 0) +
              (overview.pickup.statuses.not_arrived || 0) +
              (overview.pickup.statuses.abnormal || 0)
            }}
          </div>
          <div class="sprout-metric-note">含未找到、到班异常和待补照片</div>
        </div>
        <div class="sprout-metric-card tone-orange">
          <div class="sprout-metric-label">待处理申请</div>
          <div class="sprout-metric-value">
            {{
              overview.pending_applications + overview.pending_leave_requests
            }}
          </div>
          <div class="sprout-metric-note">
            入班 {{ overview.pending_applications }} · 请假
            {{ overview.pending_leave_requests }}
          </div>
        </div>
        <div class="sprout-metric-card tone-green">
          <div class="sprout-metric-label">今日餐食</div>
          <div class="sprout-metric-value">
            {{ overview.meal_recorded ? '已登记' : '待登记' }}
          </div>
          <div class="sprout-metric-note">
            {{ overview.meal_plans }} 条餐食记录
          </div>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <ElCard class="sprout-table-card" shadow="never">
          <template #header>
            <div class="sprout-table-toolbar">
              <div>
                <h2 class="sprout-section-title">待跟进异常</h2>
                <p class="sprout-section-caption">
                  {{ overview.date }} · 只显示有数量的项目
                </p>
              </div>
              <ElTag :type="overview.anomalies.length ? 'danger' : 'success'">
                {{
                  overview.anomalies.length
                    ? `${overview.anomalies.length} 项`
                    : '正常'
                }}
              </ElTag>
            </div>
          </template>
          <div v-if="overview.anomalies.length" class="space-y-3">
            <div
              v-for="item in overview.anomalies"
              :key="item.code"
              class="flex items-center justify-between rounded-lg bg-slate-50 p-3"
            >
              <span class="font-medium text-slate-700">{{ item.label }}</span>
              <ElTag type="danger">{{ item.count }}</ElTag>
            </div>
          </div>
          <ElEmpty
            v-else
            description="今天暂时没有待跟进异常"
            :image-size="90"
          />
        </ElCard>

        <ElCard class="sprout-table-card" shadow="never">
          <template #header>
            <div class="sprout-table-toolbar">
              <div>
                <h2 class="sprout-section-title">接送状态分布</h2>
                <p class="sprout-section-caption">按孩子当前状态汇总</p>
              </div>
              <ElTag type="info">{{ overview.pickup.operations }} 个任务</ElTag>
            </div>
          </template>
          <div
            v-if="Object.keys(overview.pickup.statuses).length"
            class="space-y-3"
          >
            <div
              v-for="(count, status) in overview.pickup.statuses"
              :key="status"
              class="flex items-center justify-between rounded-lg bg-slate-50 p-3 text-sm"
            >
              <span>{{ statusLabel(status) }}</span>
              <strong>{{ count }} 人</strong>
            </div>
          </div>
          <ElEmpty v-else description="当天还没有接送记录" :image-size="90" />
        </ElCard>
      </div>

      <ElCard class="sprout-table-card mt-4" shadow="never">
        <template #header>
          <div class="sprout-table-toolbar">
            <div>
              <h2 class="sprout-section-title">按班级查看</h2>
              <p class="sprout-section-caption">
                教师只会看到自己负责班级的数据
              </p>
            </div>
            <ElTag
              :type="overview.summary_status === 'closed' ? 'info' : 'warning'"
            >
              总结：{{ statusLabel(overview.summary_status || '未生成') }}
            </ElTag>
          </div>
        </template>
        <ElTable :data="overview.classes" stripe>
          <ElTableColumn label="班级" min-width="180">
            <template #default="{ row }">{{
              row.class_name || `班级 #${row.school_class_id}`
            }}</template>
          </ElTableColumn>
          <ElTableColumn label="任务数" width="100" prop="operations" />
          <ElTableColumn label="孩子数" width="100" prop="students" />
          <ElTableColumn label="已处理" width="100" prop="resolved" />
          <ElTableColumn label="异常" width="100">
            <template #default="{ row }">
              <ElTag :type="row.abnormal ? 'danger' : 'success'">{{
                row.abnormal
              }}</ElTag>
            </template>
          </ElTableColumn>
          <template #empty
            ><ElEmpty description="当天还没有班级接送数据" :image-size="80"
          /></template>
        </ElTable>
      </ElCard>
    </div>
    <ElCard v-else class="sprout-table-card" shadow="never">
      <ElEmpty description="暂无异常汇总数据" :image-size="110" />
    </ElCard>
  </div>
</template>
