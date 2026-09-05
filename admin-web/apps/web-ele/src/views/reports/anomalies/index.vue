<script lang="ts" setup>
import type { DailyExceptionRecord, DailyOverviewRecord } from '#/api/reports';

import { computed, onMounted, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElDatePicker,
  ElDialog,
  ElEmpty,
  ElInput,
  ElMessage,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import {
  acknowledgeDailyExceptionApi,
  getDailyExceptionsApi,
  getDailyOverviewApi,
} from '#/api/reports';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'AnomalyOverview' });

const selectedDate = ref(businessToday());
const loading = ref(false);
const loadError = ref('');
const overview = ref<DailyOverviewRecord | null>(null);
const exceptions = ref<DailyExceptionRecord[]>([]);
const showAcknowledged = ref(false);
const acknowledgeVisible = ref(false);
const acknowledgeLoading = ref(false);
const acknowledgeNote = ref('');
const selectedException = ref<DailyExceptionRecord | null>(null);

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

const categoryLabels: Record<string, string> = {
  application: '入班申请',
  homework: '作业',
  leave: '请假',
  meal: '餐食',
  pickup: '接送',
  student: '学生档案',
  summary: '每日总结',
};

function categoryLabel(value: string) {
  return categoryLabels[value] || value;
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const [overviewResult, exceptionResult] = await Promise.all([
      getDailyOverviewApi({ date: selectedDate.value }),
      getDailyExceptionsApi({
        date: selectedDate.value,
        include_acknowledged: showAcknowledged.value,
      }),
    ]);
    overview.value = overviewResult;
    exceptions.value = exceptionResult.items;
  } catch {
    overview.value = null;
    loadError.value = '异常数据加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

function toggleExceptionHistory() {
  showAcknowledged.value = !showAcknowledged.value;
  void loadData();
}

function openAcknowledge(item: DailyExceptionRecord) {
  if (item.acknowledged || acknowledgeLoading.value) return;
  selectedException.value = item;
  acknowledgeNote.value = '';
  acknowledgeVisible.value = true;
}

async function submitAcknowledge() {
  const item = selectedException.value;
  if (!item || acknowledgeLoading.value) return;
  acknowledgeLoading.value = true;
  try {
    await acknowledgeDailyExceptionApi(item.id, {
      date: selectedDate.value,
      note: acknowledgeNote.value,
    });
    ElMessage.success('异常已确认并记录处理留痕');
    acknowledgeVisible.value = false;
    selectedException.value = null;
    await loadData();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '异常确认失败');
  } finally {
    acknowledgeLoading.value = false;
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
              <ElButton size="small" text @click="toggleExceptionHistory">
                {{ showAcknowledged ? '仅看待处理' : '查看已确认记录' }}
              </ElButton>
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
              <h2 class="sprout-section-title">
                {{ showAcknowledged ? '已确认异常' : '异常明细' }}
              </h2>
              <p class="sprout-section-caption">
                {{ overview.date }} · 管理端可查看全机构记录并保留处理说明
              </p>
            </div>
            <ElTag :type="exceptions.length ? 'warning' : 'success'">
              {{ exceptions.length ? `${exceptions.length} 项` : '暂无记录' }}
            </ElTag>
          </div>
        </template>
        <div v-if="exceptions.length" class="space-y-3">
          <div
            v-for="item in exceptions"
            :key="item.id"
            class="rounded-lg border border-slate-100 bg-slate-50 p-4"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <ElTag
                  :type="item.severity === 'danger' ? 'danger' : 'warning'"
                >
                  {{ item.label }}
                </ElTag>
                <span class="text-sm text-slate-500">{{
                  categoryLabel(item.category)
                }}</span>
              </div>
              <ElTag v-if="item.acknowledged" type="success">已确认</ElTag>
            </div>
            <p class="mt-3 text-sm leading-6 text-slate-700">
              {{ item.message }}
            </p>
            <p
              v-if="item.class_name || item.student_name"
              class="mt-1 text-xs text-slate-500"
            >
              {{ item.class_name || '今日托管'
              }}<span v-if="item.student_name"> · {{ item.student_name }}</span>
            </p>
            <div
              class="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500"
            >
              <span v-if="item.acknowledged"
                >{{ item.acknowledged_by || '工作人员' }} ·
                {{ item.acknowledged_at || '已确认' }}</span
              >
              <span v-else>等待教师或管理员跟进</span>
              <ElButton
                v-if="!item.acknowledged"
                size="small"
                @click="openAcknowledge(item)"
              >
                标记已知
              </ElButton>
            </div>
          </div>
        </div>
        <ElEmpty v-else description="当前视图没有异常明细" :image-size="90" />
      </ElCard>

      <ElCard class="sprout-table-card mt-4" shadow="never">
        <template #header>
          <div class="sprout-table-toolbar">
            <div>
              <h2 class="sprout-section-title">按班级查看</h2>
              <p class="sprout-section-caption">
                教师按负责班级查看，管理员可查看全机构数据
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

    <ElDialog
      v-model="acknowledgeVisible"
      title="标记异常为已知"
      width="min(520px, 92vw)"
    >
      <div v-if="selectedException" class="space-y-3">
        <div class="rounded-lg bg-slate-50 p-3 text-sm text-slate-700">
          <div class="font-medium">{{ selectedException.label }}</div>
          <div class="mt-1 leading-6">{{ selectedException.message }}</div>
        </div>
        <ElInput
          v-model="acknowledgeNote"
          type="textarea"
          :rows="4"
          maxlength="200"
          show-word-limit
          placeholder="例如：已电话联系家长，明天补齐照片"
        />
      </div>
      <template #footer>
        <ElButton @click="acknowledgeVisible = false">取消</ElButton>
        <ElButton
          type="primary"
          :loading="acknowledgeLoading"
          @click="submitAcknowledge"
        >
          确认已知
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
