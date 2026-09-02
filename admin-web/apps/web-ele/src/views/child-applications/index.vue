<script lang="ts" setup>
import type {
  ChildApplicationRecord,
  ChildApplicationStatus,
  ReviewChildApplicationPayload,
} from '#/api/child-applications';
import type { SchoolClassRecord } from '#/api/master-data';
import type { TeacherAssignmentRecord } from '#/api/teacher-assignments';

import { computed, onMounted, reactive, ref } from 'vue';

import { useUserStore } from '@vben/stores';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElCheckbox,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import {
  getChildApplicationsApi,
  reviewChildApplicationApi,
} from '#/api/child-applications';
import { getSchoolClassesApi } from '#/api/master-data';
import { getTeacherAssignmentsApi } from '#/api/teacher-assignments';

defineOptions({ name: 'ChildApplications' });

type ApplicationFilterStatus = 'all' | ChildApplicationStatus;
type ReviewStatus = Exclude<ChildApplicationStatus, 'pending'>;

const userStore = useUserStore();
const loading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const applications = ref<ChildApplicationRecord[]>([]);
const schoolClasses = ref<SchoolClassRecord[]>([]);
const assignments = ref<TeacherAssignmentRecord[]>([]);
const selectedApplication = ref<ChildApplicationRecord | null>(null);
const dialogVisible = ref(false);

const filters = reactive({
  keyword: '',
  status: 'pending' as ApplicationFilterStatus,
});

const reviewForm = reactive<{
  create_school_class: boolean;
  review_note: string;
  school_class_id: number;
  status: ReviewStatus;
  student_id: number;
}>({
  create_school_class: false,
  review_note: '',
  school_class_id: 0,
  status: 'approved',
  student_id: 0,
});

const currentRole = computed(() => userStore.userInfo?.roles?.[0] || '');
const activeSchoolClasses = computed(() => {
  const active = schoolClasses.value.filter((item) => item.status === 'active');
  if (currentRole.value !== 'teacher') return active;

  const assignedClassIDs = new Set(
    assignments.value
      .filter((item) => item.status === 'active')
      .map((item) => item.school_class_id),
  );
  return active.filter((item) => assignedClassIDs.has(item.id));
});

const pendingCount = computed(
  () => applications.value.filter((item) => item.status === 'pending').length,
);
const needsInfoCount = computed(
  () =>
    applications.value.filter((item) => item.status === 'needs_info').length,
);
const approvedCount = computed(
  () => applications.value.filter((item) => item.status === 'approved').length,
);
const actionableCount = computed(
  () => pendingCount.value + needsInfoCount.value,
);

const filteredApplications = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase();
  return applications.value.filter((item) => {
    if (filters.status !== 'all' && item.status !== filters.status) {
      return false;
    }
    if (!keyword) return true;

    return [
      item.student_name,
      item.guardian_name,
      item.guardian_phone,
      item.school_name_input,
      item.grade_input,
      item.class_name_input,
      item.notes,
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(keyword);
  });
});

function isActionable(status: ChildApplicationStatus) {
  return status === 'pending' || status === 'needs_info';
}

function statusLabel(status: ChildApplicationStatus) {
  return {
    approved: '已通过',
    needs_info: '待补充',
    pending: '待审核',
    rejected: '已驳回',
  }[status];
}

function statusType(status: ChildApplicationStatus) {
  if (status === 'approved') return 'success';
  if (status === 'rejected') return 'danger';
  if (status === 'needs_info') return 'warning';
  return 'info';
}

function studentMatchLabel(student: { guardian_phone?: string; name: string }) {
  return student.guardian_phone
    ? `${student.name}（家长尾号${student.guardian_phone.slice(-4)}）`
    : student.name;
}

function reviewActionLabel(status: ReviewStatus) {
  if (status === 'approved') return '通过';
  if (status === 'rejected') return '驳回';
  return '标记为待补充';
}

function schoolClassLabel(id?: number) {
  if (!id) return '待确认班级';
  const schoolClass = schoolClasses.value.find((item) => item.id === id);
  return schoolClass ? `${schoolClass.grade}${schoolClass.name}` : `班级 ${id}`;
}

function applicationClassLabel(application: ChildApplicationRecord) {
  const grade = application.grade_input || application.grade;
  const className = application.class_name_input || application.class_name;
  return [grade, className].filter(Boolean).join('') || '待确认班级';
}

function applicationSchoolLabel(application: ChildApplicationRecord) {
  return application.school_name_input || '学校待确认';
}

function applicationClassSummary(application: ChildApplicationRecord) {
  return [
    applicationSchoolLabel(application),
    applicationClassLabel(application),
  ].join(' · ');
}

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value || '--';
  return new Intl.DateTimeFormat('zh-CN', {
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    month: '2-digit',
  }).format(date);
}

function resetFilters() {
  filters.keyword = '';
  filters.status = 'pending';
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  const [applicationResult, classResult, assignmentResult] =
    await Promise.allSettled([
      getChildApplicationsApi(),
      getSchoolClassesApi(),
      getTeacherAssignmentsApi(),
    ]);

  if (applicationResult.status === 'fulfilled') {
    applications.value = applicationResult.value.items;
  } else {
    applications.value = [];
    loadError.value = '家长入班申请加载失败，请稍后重试。';
  }
  if (classResult.status === 'fulfilled') {
    schoolClasses.value = classResult.value.items;
  }
  if (assignmentResult.status === 'fulfilled') {
    assignments.value = assignmentResult.value.items;
  }
  loading.value = false;
}

function openReview(application: ChildApplicationRecord) {
  selectedApplication.value = application;
  reviewForm.status = 'approved';
  reviewForm.school_class_id = application.school_class_id || 0;
  reviewForm.student_id = application.student_id || 0;
  reviewForm.create_school_class = false;
  reviewForm.review_note = application.review_note || '';
  dialogVisible.value = true;
}

function closeReview() {
  dialogVisible.value = false;
  selectedApplication.value = null;
}

async function submitReview() {
  const application = selectedApplication.value;
  if (!application) return;

  if (
    reviewForm.status === 'approved' &&
    !reviewForm.school_class_id &&
    !reviewForm.create_school_class
  ) {
    ElMessage.warning('请先选择孩子所在的学校班级，或勾选自动创建班级。');
    return;
  }
  if (
    reviewForm.status === 'approved' &&
    (application.student_matches?.length || 0) > 1 &&
    !reviewForm.student_id
  ) {
    ElMessage.warning('该班有同名学生，请先选择要绑定的学生档案。');
    return;
  }
  if (reviewForm.status === 'needs_info' && !reviewForm.review_note.trim()) {
    ElMessage.warning('请填写需要家长补充的信息。');
    return;
  }

  const actionLabel = reviewActionLabel(reviewForm.status);
  try {
    await ElMessageBox.confirm(
      `确认${actionLabel}「${application.student_name}」的入班申请吗？`,
      '确认审核结果',
      { type: reviewForm.status === 'rejected' ? 'warning' : 'info' },
    );
  } catch {
    return;
  }

  submitting.value = true;
  try {
    const payload: ReviewChildApplicationPayload = {
      review_note: reviewForm.review_note.trim() || undefined,
      status: reviewForm.status,
    };
    if (reviewForm.status === 'approved') {
      if (reviewForm.school_class_id) {
        payload.school_class_id = reviewForm.school_class_id;
      }
      if (reviewForm.student_id) {
        payload.student_id = reviewForm.student_id;
      }
      payload.create_school_class = reviewForm.create_school_class;
    }

    await reviewChildApplicationApi(application.id, payload);
    ElMessage.success(`已${actionLabel}家长入班申请`);
    closeReview();
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
        <p class="sprout-page-kicker">家校协同 · 入班审核</p>
        <h1 class="sprout-page-title">家长入班申请</h1>
        <p class="sprout-page-description">
          核对孩子、班级和监护人信息后再通过申请，审核结果会同步到家长端。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton :loading="loading" @click="loadData">刷新</ElButton>
        <ElButton type="primary" @click="filters.status = 'pending'">
          只看待审核
        </ElButton>
      </div>
    </div>

    <div class="child-application-overview">
      <article class="child-application-lead">
        <div class="child-application-lead-copy">
          <span class="child-application-lead-kicker">今日入班队列</span>
          <h2>先核对信息，再让孩子入班</h2>
          <p>
            把家长提交的学校、班级和监护人信息确认清楚，后续接送和作业才会准确同步。
          </p>
        </div>
        <div class="child-application-lead-footer">
          <span
            >当前待处理 <strong>{{ actionableCount }}</strong> 条</span
          >
          <ElButton type="primary" @click="filters.status = 'pending'">
            查看待处理
          </ElButton>
        </div>
      </article>

      <div class="child-application-stat-grid">
        <button
          class="child-application-stat-card tone-pending"
          type="button"
          @click="filters.status = 'pending'"
        >
          <span class="child-application-stat-icon">审</span>
          <span>
            <small>待审核</small>
            <strong>{{ pendingCount }}</strong>
          </span>
        </button>
        <button
          class="child-application-stat-card tone-info"
          type="button"
          @click="filters.status = 'needs_info'"
        >
          <span class="child-application-stat-icon">补</span>
          <span>
            <small>待补充</small>
            <strong>{{ needsInfoCount }}</strong>
          </span>
        </button>
        <button
          class="child-application-stat-card tone-approved"
          type="button"
          @click="filters.status = 'approved'"
        >
          <span class="child-application-stat-icon">✓</span>
          <span>
            <small>已通过</small>
            <strong>{{ approvedCount }}</strong>
          </span>
        </button>
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
      <div class="sprout-table-toolbar mb-4 child-application-toolbar">
        <div>
          <h2 class="sprout-section-title">申请列表</h2>
          <p class="sprout-section-caption">
            共 {{ applications.length }} 条申请，当前显示
            {{ filteredApplications.length }} 条。
          </p>
        </div>
        <div class="child-application-filter">
          <ElInput
            v-model="filters.keyword"
            clearable
            placeholder="搜索孩子、家长或手机号"
          />
          <ElSelect v-model="filters.status" placeholder="申请状态">
            <ElOption label="待审核" value="pending" />
            <ElOption label="待补充" value="needs_info" />
            <ElOption label="已通过" value="approved" />
            <ElOption label="已驳回" value="rejected" />
            <ElOption label="全部状态" value="all" />
          </ElSelect>
          <ElButton @click="resetFilters">重置</ElButton>
        </div>
      </div>

      <div class="sprout-table-wrap">
        <ElTable
          v-loading="loading"
          :data="filteredApplications"
          row-key="id"
          stripe
        >
          <ElTableColumn label="孩子" min-width="155">
            <template #default="{ row }">
              <div class="child-application-person">
                <strong>{{
                  (row as ChildApplicationRecord).student_name
                }}</strong>
                <small>申请 #{{ (row as ChildApplicationRecord).id }}</small>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="所在班级" min-width="210">
            <template #default="{ row }">
              <div class="child-application-class">
                <strong>{{
                  applicationClassLabel(row as ChildApplicationRecord)
                }}</strong>
                <small>{{
                  applicationSchoolLabel(row as ChildApplicationRecord)
                }}</small>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="监护人" min-width="170">
            <template #default="{ row }">
              <div class="child-application-person">
                <strong>{{
                  (row as ChildApplicationRecord).guardian_name || '家长'
                }}</strong>
                <small>{{
                  (row as ChildApplicationRecord).guardian_phone ||
                  '未填写手机号'
                }}</small>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="提交时间" width="135">
            <template #default="{ row }">
              {{ formatDate((row as ChildApplicationRecord).created_at) }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="105">
            <template #default="{ row }">
              <ElTag
                class="sprout-status"
                :type="statusType((row as ChildApplicationRecord).status)"
              >
                {{ statusLabel((row as ChildApplicationRecord).status) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="家长备注" min-width="190">
            <template #default="{ row }">
              <span
                class="child-application-note"
                :title="(row as ChildApplicationRecord).notes || '无备注'"
              >
                {{ (row as ChildApplicationRecord).notes || '无备注' }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <ElButton
                v-if="isActionable((row as ChildApplicationRecord).status)"
                link
                type="primary"
                @click="openReview(row as ChildApplicationRecord)"
              >
                {{
                  (row as ChildApplicationRecord).status === 'pending'
                    ? '审核'
                    : '继续处理'
                }}
              </ElButton>
              <span v-else class="text-gray-400">已处理</span>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty
              :description="
                applications.length
                  ? '没有符合当前筛选条件的申请'
                  : '暂时没有家长入班申请'
              "
              :image-size="90"
            />
          </template>
        </ElTable>
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      title="处理家长入班申请"
      width="min(620px, 94vw)"
      @closed="selectedApplication = null"
    >
      <template v-if="selectedApplication">
        <div class="child-application-dialog-intro">
          <div class="child-application-avatar">
            {{ selectedApplication.student_name.slice(0, 1) }}
          </div>
          <div>
            <strong>{{ selectedApplication.student_name }}</strong>
            <span>{{ applicationClassSummary(selectedApplication) }}</span>
          </div>
          <ElTag
            class="sprout-status"
            :type="statusType(selectedApplication.status)"
          >
            {{ statusLabel(selectedApplication.status) }}
          </ElTag>
        </div>

        <div class="child-application-detail-grid">
          <div>
            <small>监护人</small>
            <strong>{{ selectedApplication.guardian_name || '家长' }}</strong>
          </div>
          <div>
            <small>联系电话</small>
            <strong>{{
              selectedApplication.guardian_phone || '未填写'
            }}</strong>
          </div>
          <div>
            <small>关系</small>
            <strong>{{ selectedApplication.relationship || '家长' }}</strong>
          </div>
          <div>
            <small>提交时间</small>
            <strong>{{ formatDate(selectedApplication.created_at) }}</strong>
          </div>
        </div>

        <div class="child-application-parent-note">
          <span>家长说明</span>
          <p>{{ selectedApplication.notes || '家长未填写补充说明' }}</p>
        </div>

        <ElForm label-position="top">
          <ElFormItem label="审核结果" required>
            <ElSelect v-model="reviewForm.status" class="w-full">
              <ElOption label="通过申请" value="approved" />
              <ElOption label="需要家长补充信息" value="needs_info" />
              <ElOption label="驳回申请" value="rejected" />
            </ElSelect>
          </ElFormItem>

          <ElFormItem
            v-if="reviewForm.status === 'approved'"
            label="归属学校班级"
            required
          >
            <ElSelect
              v-model="reviewForm.school_class_id"
              class="w-full"
              clearable
              placeholder="请选择孩子所在班级"
            >
              <ElOption
                v-for="schoolClass in activeSchoolClasses"
                :key="schoolClass.id"
                :label="`${schoolClass.grade}${schoolClass.name}`"
                :value="schoolClass.id"
              />
            </ElSelect>
            <p class="child-application-form-hint">
              当前申请班级：{{
                schoolClassLabel(selectedApplication.school_class_id)
              }}
            </p>
          </ElFormItem>

          <ElFormItem
            v-if="
              reviewForm.status === 'approved' &&
              !selectedApplication.school_class_id
            "
          >
            <ElCheckbox v-model="reviewForm.create_school_class">
              没有匹配班级时，按申请信息自动创建班级
            </ElCheckbox>
          </ElFormItem>

          <ElFormItem
            v-if="
              reviewForm.status === 'approved' &&
              (selectedApplication.student_matches?.length || 0) > 1
            "
            label="匹配学生档案"
            required
          >
            <ElSelect
              v-model="reviewForm.student_id"
              class="w-full"
              placeholder="请选择同名学生档案"
            >
              <ElOption
                v-for="student in selectedApplication.student_matches"
                :key="student.id"
                :label="studentMatchLabel(student)"
                :value="student.id"
              />
            </ElSelect>
            <p class="child-application-form-hint">
              检测到同班同名学生，选择后才会绑定到正确档案。
            </p>
          </ElFormItem>

          <ElAlert
            v-if="
              reviewForm.status === 'approved' &&
              !activeSchoolClasses.length &&
              !selectedApplication.school_class_id
            "
            :closable="false"
            show-icon
            title="当前没有可选择的负责班级，请勾选自动创建班级，或让管理员先维护班级。"
            type="warning"
          />

          <ElFormItem label="审核备注">
            <ElInput
              v-model="reviewForm.review_note"
              :autosize="{ minRows: 3, maxRows: 6 }"
              placeholder="通过可填写安排说明；待补充时请写清需要家长补充的内容"
              type="textarea"
            />
          </ElFormItem>
        </ElForm>
      </template>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton :loading="submitting" type="primary" @click="submitReview">
          确认处理
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
.child-application-overview {
  display: grid;
  grid-template-columns: minmax(280px, 1.1fr) minmax(0, 1.45fr);
  gap: 14px;
  margin-bottom: 18px;
}

.child-application-lead {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-height: 190px;
  padding: 23px 24px;
  overflow: hidden;
  background:
    radial-gradient(circle at 100% 0%, rgb(255 211 113 / 34%), transparent 31%),
    linear-gradient(135deg, #edf7ff 0%, #f7fbff 62%, #fffaf0 100%);
  border: 1px solid #d5e6f5;
  border-radius: 18px;
  box-shadow: var(--sprout-shadow);
}

.child-application-lead::after {
  position: absolute;
  right: -32px;
  bottom: -44px;
  width: 126px;
  height: 126px;
  content: '';
  border: 15px solid rgb(37 99 235 / 9%);
  border-radius: 50%;
}

.child-application-lead-copy,
.child-application-lead-footer {
  position: relative;
  z-index: 1;
}

.child-application-lead-kicker {
  font-size: 12px;
  font-weight: 700;
  color: var(--sprout-primary);
}

.child-application-lead h2 {
  margin: 10px 0 7px;
  font-size: 20px;
  font-weight: 760;
  color: var(--sprout-ink);
}

.child-application-lead p {
  max-width: 440px;
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--sprout-muted);
}

.child-application-lead-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 18px;
  font-size: 13px;
  color: #53627a;
}

.child-application-lead-footer strong {
  margin: 0 3px;
  font-size: 22px;
  color: var(--sprout-primary);
}

.child-application-stat-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.child-application-stat-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  align-items: center;
  min-width: 0;
  padding: 18px 16px;
  text-align: left;
  cursor: pointer;
  background: #fff;
  border: 1px solid var(--sprout-line);
  border-radius: 16px;
  box-shadow: 0 8px 22px rgb(36 65 110 / 5%);
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.child-application-stat-card:hover,
.child-application-stat-card:focus-visible {
  outline: none;
  border-color: #b7cdf5;
  box-shadow: 0 10px 24px rgb(37 99 235 / 11%);
  transform: translateY(-1px);
}

.child-application-stat-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  font-size: 17px;
  font-weight: 750;
  color: var(--sprout-primary);
  background: var(--sprout-primary-soft);
  border-radius: 12px;
}

.child-application-stat-card.tone-pending .child-application-stat-icon {
  color: #c57c00;
  background: var(--sprout-sun-soft);
}

.child-application-stat-card.tone-info .child-application-stat-icon {
  color: #0b83ba;
  background: var(--sprout-sky-soft);
}

.child-application-stat-card.tone-approved .child-application-stat-icon {
  color: #078b68;
  background: #e9faf4;
}

.child-application-stat-card small,
.child-application-stat-card strong {
  display: block;
}

.child-application-stat-card small {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  color: var(--sprout-muted);
  white-space: nowrap;
}

.child-application-stat-card strong {
  margin-top: 7px;
  font-size: 25px;
  line-height: 1;
  color: var(--sprout-ink);
}

.child-application-toolbar {
  align-items: flex-end;
}

.child-application-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  justify-content: flex-end;
}

.child-application-filter .el-input {
  width: 240px;
}

.child-application-filter .el-select {
  width: 130px;
}

.child-application-person,
.child-application-class {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.child-application-person strong,
.child-application-class strong {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 13px;
  font-weight: 700;
  color: #2c3d59;
  white-space: nowrap;
}

.child-application-person small,
.child-application-class small {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  color: #8997ac;
  white-space: nowrap;
}

.child-application-note {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #62718a;
  white-space: nowrap;
}

.child-application-dialog-intro {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 14px;
  background: #f7faff;
  border: 1px solid #e0eaf7;
  border-radius: 14px;
}

.child-application-avatar {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 44px;
  height: 44px;
  font-size: 18px;
  font-weight: 750;
  color: #fff;
  background: linear-gradient(145deg, #60a5fa, #2563eb);
  border-radius: 14px;
}

.child-application-dialog-intro > div:nth-child(2) {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.child-application-dialog-intro strong {
  font-size: 15px;
  color: var(--sprout-ink);
}

.child-application-dialog-intro span:not(.el-tag) {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  color: var(--sprout-muted);
  white-space: nowrap;
}

.child-application-dialog-intro .el-tag {
  margin-left: auto;
}

.child-application-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 13px 20px;
  padding: 18px 2px;
  border-bottom: 1px solid #edf1f7;
}

.child-application-detail-grid > div {
  display: grid;
  gap: 4px;
}

.child-application-detail-grid small,
.child-application-parent-note > span {
  font-size: 12px;
  color: #8997ac;
}

.child-application-detail-grid strong {
  font-size: 13px;
  color: #2c3d59;
}

.child-application-parent-note {
  padding: 16px 2px 4px;
}

.child-application-parent-note p {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.7;
  color: #53627a;
  white-space: pre-wrap;
}

.child-application-form-hint {
  margin: 7px 0 0;
  font-size: 12px;
  color: #8997ac;
}

@media (max-width: 980px) {
  .child-application-overview {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .child-application-stat-grid {
    grid-template-columns: 1fr;
  }

  .child-application-toolbar {
    align-items: flex-start;
  }

  .child-application-filter {
    justify-content: flex-start;
    width: 100%;
  }

  .child-application-filter .el-input,
  .child-application-filter .el-select {
    width: 100%;
  }

  .child-application-lead {
    padding: 20px;
  }
}

@media (max-width: 460px) {
  .child-application-lead-footer {
    flex-direction: column;
    gap: 10px;
    align-items: flex-start;
  }

  .child-application-detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
