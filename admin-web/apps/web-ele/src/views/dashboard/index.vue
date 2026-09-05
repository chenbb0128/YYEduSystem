<script lang="ts" setup>
import type { ChildApplicationRecord } from '#/api/child-applications';
import type { HomeworkStudentStatus, HomeworkTaskRecord } from '#/api/homework';
import type { MasterSummary } from '#/api/master-data';
import type {
  PickupOperationRecord,
  PickupOperationStudentRecord,
} from '#/api/pickup';

import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { useUserStore } from '@vben/stores';

import { ElAlert, ElButton, ElCard, ElTag } from 'element-plus';

import { getChildApplicationsApi } from '#/api/child-applications';
import {
  getHomeworkTasksApi,
  getHomeworkTaskStudentsApi,
} from '#/api/homework';
import { getLeaveRequestsApi } from '#/api/leave-requests';
import { getMasterSummaryApi } from '#/api/master-data';
import {
  getPickupOperationsApi,
  getPickupOperationStudentsApi,
} from '#/api/pickup';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'Dashboard' });

const router = useRouter();
const userStore = useUserStore();
const today = businessToday();
const loading = ref(false);
const loadError = ref(false);
const summary = ref<MasterSummary | null>(null);
const pickupOperations = ref<PickupOperationRecord[]>([]);
const pickupStudents = ref<PickupOperationStudentRecord[]>([]);
const homeworkTasks = ref<HomeworkTaskRecord[]>([]);
const homeworkStudents = ref<Array<{ status: HomeworkStudentStatus }>>([]);
const pendingLeaveCount = ref<null | number>(null);
const pendingApplicationCount = ref<null | number>(null);

function safeItems<T>(page?: null | { items?: null | T[] }) {
  return Array.isArray(page?.items) ? page.items : [];
}

const userName = computed(
  () => userStore.userInfo?.realName || userStore.userInfo?.username || '老师',
);
const displayDate = computed(() =>
  new Intl.DateTimeFormat('zh-CN', {
    day: 'numeric',
    month: 'long',
    weekday: 'long',
  }).format(new Date()),
);

const pickupSummary = computed(() => {
  const total = pickupStudents.value.length;
  const picked = pickupStudents.value.filter((student) =>
    ['parent_picked_up', 'picked_up'].includes(student.status),
  ).length;
  const arrived = pickupStudents.value.filter((student) =>
    ['arrived', 'self_arrived'].includes(student.status),
  ).length;
  const abnormal = pickupStudents.value.filter((student) =>
    ['abnormal', 'absent', 'not_arrived'].includes(student.status),
  ).length;
  return { abnormal, arrived, picked, total };
});

const homeworkSummary = computed(() => {
  const total = homeworkStudents.value.length;
  const completed = homeworkStudents.value.filter(
    (student) => student.status === 'completed',
  ).length;
  return {
    completed,
    percent: total ? Math.round((completed / total) * 100) : 0,
    total,
  };
});

const archiveItems = computed(() => [
  { label: '学生档案', value: summary.value?.students ?? '--' },
  { label: '学校', value: summary.value?.schools ?? '--' },
  { label: '学校班级', value: summary.value?.school_classes ?? '--' },
  { label: '托管班', value: summary.value?.care_classes ?? '--' },
]);

const quickActions = [
  {
    label: '查看今日接送',
    mark: '接',
    note: '点名、拍照、处理异常',
    path: '/pickup-operations',
    tone: 'tone-blue',
  },
  {
    label: '布置今日作业',
    mark: '作',
    note: '按班级批量布置',
    path: '/homework',
    tone: 'tone-green',
  },
  {
    label: '审核家长入班',
    mark: '审',
    note: '确认孩子入班申请',
    path: '/child-applications',
    tone: 'tone-orange',
  },
  {
    label: '处理请假审核',
    mark: '假',
    note: '及时同步家长结果',
    path: '/leave-requests',
    tone: 'tone-orange',
  },
  {
    label: '查看学生档案',
    mark: '档',
    note: '学校、班级、托管班',
    path: '/master-data',
    tone: 'tone-purple',
  },
  {
    label: '管理教师班级',
    mark: '班',
    note: '维护负责关系',
    path: '/teacher-assignments',
    tone: 'tone-cyan',
  },
  {
    label: '填写每日总结',
    mark: '总',
    note: '记录孩子今日表现',
    path: '/daily-summaries',
    tone: 'tone-yellow',
  },
  {
    label: '查看异常看板',
    mark: '异',
    note: '按日期追踪异常与待办',
    path: '/reports/anomalies',
    tone: 'tone-red',
  },
];

const progressItems = computed(() => {
  const total = pickupSummary.value.total;
  const toPercent = (value: number) =>
    total ? Math.min(100, Math.round((value / total) * 100)) : 0;
  return [
    {
      label: '已接到',
      note: '校门口接到 / 家长接走',
      value: pickupSummary.value.picked,
      percent: toPercent(pickupSummary.value.picked),
      tone: 'tone-blue',
    },
    {
      label: '已到班',
      note: '安全抵达托管班',
      value: pickupSummary.value.arrived,
      percent: toPercent(pickupSummary.value.arrived),
      tone: 'tone-green',
    },
    {
      label: '异常待跟进',
      note: '未到 / 未找到 / 其他异常',
      value: pickupSummary.value.abnormal,
      percent: toPercent(pickupSummary.value.abnormal),
      tone: 'tone-red',
    },
  ];
});

const attentionItems = computed(() => [
  {
    label: '家长入班申请',
    value: pendingApplicationCount.value ?? '--',
    note: '等待教师或管理员确认',
    path: '/child-applications',
    tone: 'tone-sun',
  },
  {
    label: '请假申请',
    value: pendingLeaveCount.value ?? '--',
    note: '需要及时处理的申请',
    path: '/leave-requests',
    tone: 'tone-sky',
  },
  {
    label: '接送异常',
    value: pickupSummary.value.abnormal,
    note: '请打开今日接送查看详情',
    path: '/pickup-operations',
    tone: 'tone-danger',
  },
]);

function goTo(path: string) {
  void router.push(path);
}

async function loadDashboard() {
  loading.value = true;
  loadError.value = false;
  const [
    summaryResult,
    pickupResult,
    homeworkResult,
    leaveResult,
    applicationResult,
  ] = await Promise.allSettled([
    getMasterSummaryApi(),
    getPickupOperationsApi({ date: today }),
    getHomeworkTasksApi({ date: today }),
    getLeaveRequestsApi(),
    getChildApplicationsApi(),
  ]);

  if (summaryResult.status === 'fulfilled') summary.value = summaryResult.value;
  if (pickupResult.status === 'fulfilled') {
    pickupOperations.value = safeItems(pickupResult.value);
    const detailResults = await Promise.allSettled(
      pickupOperations.value.map((operation) =>
        getPickupOperationStudentsApi(operation.id),
      ),
    );
    pickupStudents.value = detailResults.flatMap((result) =>
      result.status === 'fulfilled' ? safeItems(result.value) : [],
    );
  }
  if (homeworkResult.status === 'fulfilled') {
    homeworkTasks.value = safeItems(homeworkResult.value);
    const detailResults = await Promise.allSettled(
      homeworkTasks.value.map((task) => getHomeworkTaskStudentsApi(task.id)),
    );
    homeworkStudents.value = detailResults.flatMap((result) =>
      result.status === 'fulfilled' ? safeItems(result.value) : [],
    );
  }
  if (leaveResult.status === 'fulfilled') {
    pendingLeaveCount.value = safeItems(leaveResult.value).filter(
      (request) => request.status === 'pending',
    ).length;
  }
  if (applicationResult.status === 'fulfilled') {
    pendingApplicationCount.value = safeItems(applicationResult.value).filter(
      (application: ChildApplicationRecord) =>
        application.status === 'pending' || application.status === 'needs_info',
    ).length;
  }
  loadError.value = [summaryResult, pickupResult, homeworkResult].some(
    (result) => result.status === 'rejected',
  );
  loading.value = false;
}

onMounted(loadDashboard);
</script>

<template>
  <div class="sprout-page">
    <section class="sprout-dashboard-hero">
      <span class="sprout-dashboard-date">{{ displayDate }}</span>
      <div class="sprout-hero-content">
        <p class="sprout-hero-eyebrow">豆芽成长助手 · 今日工作台</p>
        <h1 class="sprout-hero-title">早上好，{{ userName }}！</h1>
        <p class="sprout-hero-description">
          把每一次接送、每一份作业和每一个孩子的变化，稳稳地记录下来。今天也一起陪孩子长高一点点。
        </p>
        <div class="sprout-inline-actions mt-5">
          <ElButton type="primary" @click="goTo('/pickup-operations')">
            查看今日接送
          </ElButton>
          <ElButton plain @click="goTo('/homework')">查看作业进度</ElButton>
        </div>
      </div>
    </section>

    <ElAlert
      v-if="loadError"
      class="mt-4"
      :closable="false"
      show-icon
      title="部分今日数据暂时没有加载成功，可以稍后点击顶部刷新重试。"
      type="warning"
    />

    <div v-loading="loading" class="sprout-metric-grid mt-5">
      <article class="sprout-metric-card">
        <div class="sprout-metric-label">今日接送人数</div>
        <div class="sprout-metric-value">{{ pickupSummary.total }}</div>
        <div class="sprout-metric-note">
          {{ pickupOperations.length }} 个班级任务
        </div>
      </article>
      <article class="sprout-metric-card tone-sky">
        <div class="sprout-metric-label">已接到人数</div>
        <div class="sprout-metric-value">{{ pickupSummary.picked }}</div>
        <div class="sprout-metric-note">校门口接到 / 家长接走</div>
      </article>
      <article class="sprout-metric-card">
        <div class="sprout-metric-label">已到班人数</div>
        <div class="sprout-metric-value">{{ pickupSummary.arrived }}</div>
        <div class="sprout-metric-note">已安全抵达托管班</div>
      </article>
      <article class="sprout-metric-card tone-danger">
        <div class="sprout-metric-label">异常人数</div>
        <div class="sprout-metric-value">{{ pickupSummary.abnormal }}</div>
        <div class="sprout-metric-note">未到 / 未找到 / 其他异常</div>
      </article>
    </div>

    <div v-loading="loading" class="sprout-dashboard-stat-grid">
      <article class="sprout-dashboard-stat-card tone-orange">
        <span class="sprout-dashboard-stat-icon">审</span>
        <div>
          <div class="sprout-dashboard-stat-label">待审核家长申请</div>
          <div class="sprout-dashboard-stat-value">
            {{ pendingApplicationCount ?? '--' }}
          </div>
        </div>
        <ElButton link type="primary" @click="goTo('/child-applications')">
          去审核
        </ElButton>
      </article>
      <article class="sprout-dashboard-stat-card tone-sky">
        <span class="sprout-dashboard-stat-icon">假</span>
        <div>
          <div class="sprout-dashboard-stat-label">待处理请假</div>
          <div class="sprout-dashboard-stat-value">
            {{ pendingLeaveCount ?? '--' }}
          </div>
        </div>
        <ElButton link type="primary" @click="goTo('/leave-requests')">
          查看
        </ElButton>
      </article>
      <article class="sprout-dashboard-stat-card tone-green">
        <span class="sprout-dashboard-stat-icon">作</span>
        <div>
          <div class="sprout-dashboard-stat-label">今日作业完成率</div>
          <div class="sprout-dashboard-stat-value">
            {{ homeworkSummary.percent }}<small>%</small>
          </div>
        </div>
        <ElButton link type="primary" @click="goTo('/homework')">
          去批改
        </ElButton>
      </article>
    </div>

    <ElCard class="sprout-card sprout-quick-actions-card" shadow="never">
      <div class="sprout-section-head mb-4">
        <div>
          <h2 class="sprout-section-title">常用操作</h2>
          <p class="sprout-section-caption">从这里快速进入今天要处理的工作</p>
        </div>
        <span class="sprout-role-badge tone-blue">工作台</span>
      </div>
      <div class="sprout-action-grid">
        <button
          v-for="action in quickActions"
          :key="action.path"
          class="sprout-action-tile"
          type="button"
          @click="goTo(action.path)"
        >
          <span class="sprout-action-icon" :class="action.tone">
            {{ action.mark }}
          </span>
          <span class="sprout-action-copy">
            <strong>{{ action.label }}</strong>
            <small>{{ action.note }}</small>
          </span>
          <span class="sprout-action-arrow">›</span>
        </button>
      </div>
    </ElCard>

    <div class="sprout-dashboard-grid">
      <ElCard class="sprout-card" shadow="never">
        <div class="sprout-section-head mb-4">
          <div>
            <h2 class="sprout-section-title">今日作业完成情况</h2>
            <p class="sprout-section-caption">
              {{ homeworkTasks.length }} 份班级作业 ·
              {{ homeworkSummary.completed }}/{{ homeworkSummary.total }}
              人已完成
            </p>
          </div>
          <ElTag class="sprout-status" type="success">
            {{ homeworkSummary.percent }}% 完成
          </ElTag>
        </div>
        <div class="sprout-progress">
          <span :style="{ width: `${homeworkSummary.percent}%` }"></span>
        </div>
        <ul v-if="homeworkTasks.length" class="sprout-list mt-5">
          <li
            v-for="task in homeworkTasks.slice(0, 4)"
            :key="task.id"
            class="sprout-list-item"
          >
            <div class="sprout-list-item-main">
              <div class="sprout-list-item-title">
                {{ task.subject }} · {{ task.content }}
              </div>
              <div class="sprout-list-item-meta">
                {{ task.creator_name }} 布置
              </div>
            </div>
            <ElButton link type="primary" @click="goTo('/homework')"
              >查看</ElButton
            >
          </li>
        </ul>
        <div v-else class="sprout-empty-state mt-5">今天还没有布置作业</div>
      </ElCard>

      <ElCard class="sprout-card" shadow="never">
        <div class="sprout-section-head mb-4">
          <div>
            <h2 class="sprout-section-title">待办工作</h2>
            <p class="sprout-section-caption">只保留需要你实际处理的事项</p>
          </div>
          <span class="sprout-role-badge">优先处理</span>
        </div>
        <ul class="sprout-list">
          <li
            v-for="item in attentionItems"
            :key="item.label"
            class="sprout-list-item"
          >
            <div class="sprout-list-item-main">
              <div class="sprout-list-item-title">{{ item.label }}</div>
              <div class="sprout-list-item-meta">{{ item.note }}</div>
            </div>
            <div class="sprout-inline-actions">
              <span class="sprout-role-badge" :class="[item.tone]">{{
                item.value
              }}</span>
              <ElButton link type="primary" @click="goTo(item.path)"
                >处理</ElButton
              >
            </div>
          </li>
        </ul>
      </ElCard>
    </div>

    <div class="sprout-dashboard-grid sprout-dashboard-grid--bottom">
      <ElCard class="sprout-card" shadow="never">
        <div class="sprout-section-head mb-4">
          <div>
            <h2 class="sprout-section-title">今日接送进度</h2>
            <p class="sprout-section-caption">按当前点名状态实时汇总</p>
          </div>
          <ElButton link type="primary" @click="goTo('/pickup-operations')">
            查看明细
          </ElButton>
        </div>
        <div class="sprout-progress-list">
          <div
            v-for="item in progressItems"
            :key="item.label"
            class="sprout-progress-item"
          >
            <div class="sprout-progress-item-head">
              <span>
                <span class="sprout-progress-dot" :class="item.tone"></span>
                {{ item.label }}
              </span>
              <strong>{{ item.value }} 人</strong>
            </div>
            <div class="sprout-progress-track">
              <span
                :class="item.tone"
                :style="{ width: `${item.percent}%` }"
              ></span>
            </div>
            <div class="sprout-progress-item-note">
              {{ item.note }}
            </div>
          </div>
        </div>
      </ElCard>
    </div>

    <ElCard class="sprout-card mt-5" shadow="never">
      <div class="sprout-section-head mb-4">
        <div>
          <h2 class="sprout-section-title">基础档案概览</h2>
          <p class="sprout-section-caption">档案中心的最新数据概况</p>
        </div>
        <ElButton link type="primary" @click="goTo('/master-data')">
          进入档案中心
        </ElButton>
      </div>
      <div class="sprout-metric-grid mb-0">
        <article
          v-for="item in archiveItems"
          :key="item.label"
          class="sprout-metric-card"
        >
          <div class="sprout-metric-label">{{ item.label }}</div>
          <div class="sprout-metric-value">{{ item.value }}</div>
        </article>
      </div>
    </ElCard>
  </div>
</template>
