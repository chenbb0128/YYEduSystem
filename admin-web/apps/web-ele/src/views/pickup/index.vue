<script lang="ts" setup>
import type {
  CareClassRecord,
  SchoolClassRecord,
  SchoolRecord,
  StudentRecord,
} from '#/api/master-data';
import type {
  PickupEventRecord,
  PickupHandoffRecord,
  PickupHandoffTeacherRecord,
  PickupMemberStatus,
  PickupOperationRecord,
  PickupOperationStudentRecord,
} from '#/api/pickup';
import type { TeacherAssignmentRecord } from '#/api/teacher-assignments';

import { computed, onMounted, reactive, ref, watch } from 'vue';

import { useUserStore } from '@vben/stores';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElCheckbox,
  ElCheckboxGroup,
  ElDatePicker,
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
  getCareClassesApi,
  getSchoolClassesApi,
  getSchoolsApi,
  getStudentsApi,
} from '#/api/master-data';
import {
  confirmPickupOperationApi,
  correctPickupEventApi,
  createPickupOperationApi,
  finishPickupOperationApi,
  getPickupEventsApi,
  getPickupHandoffsApi,
  getPickupHandoffTeachersApi,
  getPickupOperationsApi,
  getPickupOperationStudentsApi,
  handoverPickupOperationApi,
  markPickupStudentApi,
  startPickupOperationApi,
  uploadPickupPhotoApi,
} from '#/api/pickup';
import { businessAssetURL } from '#/api/request';
import { getTeacherAssignmentsApi } from '#/api/teacher-assignments';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'PickupOperations' });

const today = businessToday();
const selectedDate = ref(today);
const loading = ref(false);
const detailLoading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const dialogVisible = ref(false);
const currentOperation = ref<null | PickupOperationRecord>(null);
const operationStudents = ref<PickupOperationStudentRecord[]>([]);
const operations = ref<PickupOperationRecord[]>([]);
const schools = ref<SchoolRecord[]>([]);
const schoolClasses = ref<SchoolClassRecord[]>([]);
const careClasses = ref<CareClassRecord[]>([]);
const archiveStudents = ref<StudentRecord[]>([]);
const photoInput = ref<HTMLInputElement | null>(null);
const pendingPhotoStudent = ref<null | PickupOperationStudentRecord>(null);
const correctionDialogVisible = ref(false);
const correctionLoading = ref(false);
const correctionStudent = ref<null | PickupOperationStudentRecord>(null);
const correctionEventID = ref(0);
const correctionStatus =
  ref<Exclude<PickupMemberStatus, 'planned'>>('abnormal');
const correctionReason = ref('');
const assignments = ref<TeacherAssignmentRecord[]>([]);
const handoffDialogVisible = ref(false);
const handoffLoading = ref(false);
const handoffs = ref<PickupHandoffRecord[]>([]);
const handoffTeachers = ref<PickupHandoffTeacherRecord[]>([]);
const handoffForm = reactive({
  note: '',
  teacher_role: 'collaborator' as 'collaborator' | 'lead' | 'substitute',
  to_teacher_user_id: 0,
});
const userStore = useUserStore();

const operationForm = reactive({
  care_class_id: undefined as number | undefined,
  notes: '',
  operation_date: today,
  pickup_mode: 'school_pickup' as 'school_pickup' | 'self_arrival',
  school_class_id: 0,
  student_ids: [] as number[],
  teacher_user_id: undefined as number | undefined,
  teacher_name: '',
});

const currentRole = computed(() => userStore.userInfo?.roles?.[0] || '');
const overdueOperations = computed(() =>
  operations.value.filter((operation) => {
    if (
      operation.status !== 'started' ||
      operation.operation_date !== businessToday() ||
      !operation.expected_pickup_time
    ) {
      return false;
    }
    const expected = new Date(
      `${operation.operation_date}T${operation.expected_pickup_time}:00`,
    );
    return (
      !Number.isNaN(expected.getTime()) &&
      Date.now() > expected.getTime() + 30 * 60 * 1000
    );
  }),
);
const availableSchoolClasses = computed(() => {
  const activeClasses = schoolClasses.value.filter(
    (item) => item.status === 'active',
  );
  if (currentRole.value !== 'teacher') return activeClasses;
  const assignedClassIDs = new Set(
    assignments.value
      .filter((item) => item.status === 'active')
      .map((item) => item.school_class_id),
  );
  return activeClasses.filter((item) => assignedClassIDs.has(item.id));
});
const assignedTeachersForClass = computed(() =>
  assignments.value.filter(
    (item) =>
      item.status === 'active' &&
      item.school_class_id === operationForm.school_class_id,
  ),
);

const candidateStudents = computed(() =>
  archiveStudents.value.filter((student) => {
    if (
      student.status !== 'active' ||
      student.school_class_id !== operationForm.school_class_id
    )
      return false;
    return (
      !operationForm.care_class_id ||
      student.care_class_id === operationForm.care_class_id
    );
  }),
);

const pickedCount = computed(
  () =>
    operationStudents.value.filter((student) => student.status !== 'planned')
      .length,
);
const pendingCount = computed(
  () =>
    operationStudents.value.filter((student) => student.status === 'planned')
      .length,
);
function schoolName(id: number) {
  return schools.value.find((item) => item.id === id)?.name || `学校 ${id}`;
}
function schoolClassName(id: number) {
  const item = schoolClasses.value.find((schoolClass) => schoolClass.id === id);
  return item ? `${item.grade}${item.name}` : `班级 ${id}`;
}
function careClassName(id?: number) {
  return id
    ? careClasses.value.find((item) => item.id === id)?.name || `托管班 ${id}`
    : '未分配';
}
function photoURL(path?: string) {
  return businessAssetURL(path);
}
function statusLabel(status: PickupMemberStatus) {
  return (
    {
      abnormal: '其他异常',
      arrived: '已到托管班',
      absent: '未到',
      leave: '请假',
      left: '已离班',
      midway_left: '中途离班',
      not_arrived: '到班异常',
      parent_picked_up: '家长接走',
      picked_up: '校门口接到',
      planned: '待确认',
      self_arrived: '自行到班',
    } satisfies Record<PickupMemberStatus, string>
  )[status];
}
function operationStatusLabel(status: PickupOperationRecord['status']) {
  return {
    cancelled: '已取消',
    confirmed: '已确认',
    draft: '待确认',
    finished: '已完成',
    started: '接送中',
  }[status];
}
function operationStatusType(status: PickupOperationRecord['status']) {
  if (status === 'finished') return 'success';
  if (status === 'started') return 'warning';
  return 'info';
}
function memberStatusType(status: PickupMemberStatus) {
  if (
    status === 'picked_up' ||
    status === 'self_arrived' ||
    status === 'arrived' ||
    status === 'left'
  )
    return 'success';
  if (status === 'planned') return 'info';
  if (status === 'not_arrived' || status === 'abnormal') return 'danger';
  return 'warning';
}

const correctionStatuses: Array<Exclude<PickupMemberStatus, 'planned'>> = [
  'picked_up',
  'self_arrived',
  'parent_picked_up',
  'leave',
  'absent',
  'arrived',
  'not_arrived',
  'left',
  'midway_left',
  'abnormal',
];

async function loadReferences() {
  try {
    const [
      schoolResult,
      classResult,
      careClassResult,
      studentResult,
      assignmentResult,
    ] = await Promise.all([
      getSchoolsApi(),
      getSchoolClassesApi(),
      getCareClassesApi(),
      getStudentsApi(),
      getTeacherAssignmentsApi(),
    ]);
    schools.value = schoolResult.items;
    schoolClasses.value = classResult.items;
    careClasses.value = careClassResult.items;
    archiveStudents.value = studentResult.items;
    assignments.value = assignmentResult.items;
  } catch {
    loadError.value = '接送班级数据加载失败，请稍后重试。';
  }
}

async function loadOperations() {
  loading.value = true;
  loadError.value = '';
  try {
    const result = await getPickupOperationsApi({ date: selectedDate.value });
    operations.value = result.items;
    if (currentOperation.value) {
      const refreshed = operations.value.find(
        (item) => item.id === currentOperation.value?.id,
      );
      if (refreshed) await openOperation(refreshed);
      else {
        currentOperation.value = null;
        operationStudents.value = [];
      }
    }
  } catch {
    operations.value = [];
    currentOperation.value = null;
    operationStudents.value = [];
    loadError.value = '今日接送任务加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

async function openOperation(operation: PickupOperationRecord) {
  currentOperation.value = operation;
  detailLoading.value = true;
  try {
    const [studentsResult, handoffResult] = await Promise.all([
      getPickupOperationStudentsApi(operation.id),
      getPickupHandoffsApi(operation.id),
    ]);
    operationStudents.value = studentsResult.items;
    handoffs.value = handoffResult.items;
  } catch (error) {
    operationStudents.value = [];
    ElMessage.error(
      error instanceof Error ? error.message : '接送名单加载失败',
    );
  } finally {
    detailLoading.value = false;
  }
}

async function openHandoffDialog() {
  if (!currentOperation.value || currentOperation.value.status !== 'started') {
    return;
  }
  handoffLoading.value = true;
  try {
    const result = await getPickupHandoffTeachersApi(currentOperation.value.id);
    handoffTeachers.value = result.items.filter(
      (teacher) =>
        teacher.teacher_user_id !==
        currentOperation.value?.executing_teacher_user_id,
    );
    handoffForm.to_teacher_user_id =
      handoffTeachers.value[0]?.teacher_user_id || 0;
    handoffForm.note = '';
    handoffForm.teacher_role = 'collaborator';
    if (handoffTeachers.value.length === 0) {
      ElMessage.warning('当前班级暂时没有可交接的其他教师');
      return;
    }
    handoffDialogVisible.value = true;
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '可交接教师加载失败',
    );
  } finally {
    handoffLoading.value = false;
  }
}

async function submitHandoff() {
  if (
    !currentOperation.value ||
    !handoffForm.to_teacher_user_id ||
    submitting.value
  ) {
    ElMessage.warning('请选择接手老师');
    return;
  }
  const teacher = handoffTeachers.value.find(
    (item) => item.teacher_user_id === handoffForm.to_teacher_user_id,
  );
  submitting.value = true;
  try {
    await handoverPickupOperationApi(currentOperation.value.id, {
      note: handoffForm.note.trim(),
      teacher_role: handoffForm.teacher_role,
      to_teacher_name: teacher?.teacher_name,
      to_teacher_user_id: handoffForm.to_teacher_user_id,
    });
    handoffDialogVisible.value = false;
    await loadOperations();
    if (currentOperation.value) {
      const refreshed = operations.value.find(
        (item) => item.id === currentOperation.value?.id,
      );
      if (refreshed) await openOperation(refreshed);
    }
    ElMessage.success('接送任务已完成交接，家长将收到老师变更通知');
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '接送交接失败');
  } finally {
    submitting.value = false;
  }
}

function openCreateDialog() {
  Object.assign(operationForm, {
    care_class_id: undefined,
    notes: '',
    operation_date: selectedDate.value,
    pickup_mode: 'school_pickup',
    school_class_id: availableSchoolClasses.value[0]?.id || 0,
    student_ids: [],
    teacher_user_id: undefined,
    teacher_name: '',
  });
  syncCandidateStudents();
  syncTeacher();
  dialogVisible.value = true;
}

function syncCandidateStudents() {
  operationForm.student_ids = candidateStudents.value.map(
    (student) => student.id,
  );
}

function syncTeacher() {
  if (currentRole.value === 'teacher') {
    operationForm.teacher_user_id = undefined;
    operationForm.teacher_name =
      userStore.userInfo?.realName || userStore.userInfo?.username || '老师';
    return;
  }
  const currentTeacher = assignedTeachersForClass.value.find(
    (item) => item.teacher_user_id === operationForm.teacher_user_id,
  );
  const teacher = currentTeacher || assignedTeachersForClass.value[0];
  operationForm.teacher_user_id = teacher?.teacher_user_id;
  operationForm.teacher_name = teacher?.teacher_name || '';
}

watch(
  () => [operationForm.school_class_id, operationForm.care_class_id],
  () => {
    syncCandidateStudents();
    syncTeacher();
  },
);

async function createOperation() {
  if (
    !operationForm.school_class_id ||
    operationForm.student_ids.length === 0
  ) {
    ElMessage.warning('请选择学校班级和至少一名学生');
    return;
  }
  submitting.value = true;
  try {
    const created = await createPickupOperationApi({
      ...operationForm,
      student_ids:
        operationForm.student_ids.length > 0
          ? operationForm.student_ids
          : undefined,
    });
    dialogVisible.value = false;
    ElMessage.success('接送任务已创建，名单已生成');
    await loadOperations();
    await openOperation(created);
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '接送任务创建失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function startOperation() {
  if (!currentOperation.value || submitting.value) return;
  submitting.value = true;
  try {
    currentOperation.value = await startPickupOperationApi(
      currentOperation.value.id,
    );
    await loadOperations();
    ElMessage.success('接送任务已开始');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '接送任务启动失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function confirmOperation() {
  if (!currentOperation.value || submitting.value) return;
  submitting.value = true;
  try {
    const result = await ElMessageBox.prompt(
      '确认后家长会收到今日执行老师通知，可填写预计出发时间。',
      '确认今日接送任务',
      { inputPlaceholder: '例如：16:20，可留空' },
    );
    currentOperation.value = await confirmPickupOperationApi(
      currentOperation.value.id,
      {
        executing_teacher_user_id: currentOperation.value.teacher_user_id,
        expected_pickup_time: result.value.trim(),
        teacher_role: 'lead',
      },
    );
    await loadOperations();
    ElMessage.success('任务已确认，家长将收到今日接送老师通知');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '接送任务确认失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function finishOperation() {
  if (!currentOperation.value || submitting.value) return;
  submitting.value = true;
  try {
    currentOperation.value = await finishPickupOperationApi(
      currentOperation.value.id,
    );
    await loadOperations();
    ElMessage.success('接送任务已完成');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '接送任务完成失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function markStudent(
  student: PickupOperationStudentRecord,
  status: Exclude<PickupMemberStatus, 'planned'>,
  photo?: string,
  note = '',
) {
  if (!currentOperation.value || submitting.value) return;
  submitting.value = true;
  try {
    await markPickupStudentApi(currentOperation.value.id, student.student_id, {
      status,
      photo_url: photo,
      operator_name: currentOperation.value.teacher_name || '老师',
      note,
    });
    await openOperation(currentOperation.value);
    ElMessage.success(`${student.student_name}：${statusLabel(status)}`);
  } finally {
    submitting.value = false;
  }
}

async function markStudentWithNote(
  student: PickupOperationStudentRecord,
  status: Exclude<PickupMemberStatus, 'planned'>,
) {
  try {
    const prompt =
      status === 'left' || status === 'midway_left'
        ? '请填写离班说明，便于后续核对。'
        : '请填写接走人或异常说明，便于后续核对。';
    const result = await ElMessageBox.prompt(prompt, statusLabel(status), {
      inputPlaceholder:
        status === 'left' || status === 'midway_left'
          ? '例如：17:30 家长接走 / 参加兴趣班中途离开'
          : '例如：爸爸在校门口接走 / 未在校门口找到',
    });
    await markStudent(student, status, undefined, result.value);
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error;
  }
}

function choosePhoto(student: PickupOperationStudentRecord) {
  pendingPhotoStudent.value = student;
  photoInput.value?.click();
}

async function openCorrection(student: PickupOperationStudentRecord) {
  if (!currentOperation.value || correctionLoading.value) return;
  correctionLoading.value = true;
  try {
    const result = await getPickupEventsApi(currentOperation.value.id);
    const event = result.items.find(
      (item: PickupEventRecord) =>
        item.student_id === student.student_id &&
        item.event_type !== 'correction',
    );
    if (!event) {
      ElMessage.warning('找不到这名学生的原始接送记录');
      return;
    }
    correctionStudent.value = student;
    correctionEventID.value = event.id;
    correctionStatus.value =
      student.status === 'planned' ? 'abnormal' : student.status;
    correctionReason.value = '';
    correctionDialogVisible.value = true;
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '接送记录读取失败',
    );
  } finally {
    correctionLoading.value = false;
  }
}

async function submitCorrection() {
  if (
    !currentOperation.value ||
    !correctionEventID.value ||
    !correctionReason.value.trim()
  ) {
    ElMessage.warning('请填写更正原因');
    return;
  }
  submitting.value = true;
  try {
    await correctPickupEventApi(
      currentOperation.value.id,
      correctionEventID.value,
      {
        status: correctionStatus.value,
        reason: correctionReason.value.trim(),
      },
    );
    correctionDialogVisible.value = false;
    await openOperation(currentOperation.value);
    ElMessage.success('接送记录已更正，并已生成更正通知');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '接送记录更正失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function uploadAndMark(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  const student = pendingPhotoStudent.value;
  input.value = '';
  pendingPhotoStudent.value = null;
  if (!file || !student) return;
  submitting.value = true;
  try {
    const asset = await uploadPickupPhotoApi(file, {
      operation_id: currentOperation.value?.id,
    });
    if (!currentOperation.value) return;
    await markPickupStudentApi(currentOperation.value.id, student.student_id, {
      status: 'picked_up',
      photo_url: asset.url,
      operator_name: currentOperation.value.teacher_name || '老师',
      note: student.note || '',
    });
    await openOperation(currentOperation.value);
    ElMessage.success(`${student.student_name}：照片已补传并完成登记`);
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '接送照片上传失败',
    );
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  await loadReferences();
  await loadOperations();
});
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">安全守护 · 今日接送</p>
        <h1 class="sprout-page-title">今日接送</h1>
        <p class="sprout-page-description">
          每个学校班级生成一条任务，出发后逐个确认；校门口接到必须保留照片。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElDatePicker
          v-model="selectedDate"
          value-format="YYYY-MM-DD"
          type="date"
          @change="loadOperations"
        />
        <ElButton type="primary" @click="openCreateDialog">
          生成接送任务
        </ElButton>
        <input
          ref="photoInput"
          accept="image/jpeg,image/png,image/webp"
          class="hidden"
          type="file"
          @change="uploadAndMark"
        />
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

    <ElAlert
      v-if="overdueOperations.length"
      class="mb-4"
      :closable="false"
      show-icon
      title="有接送任务超过预计出发时间，请优先核对现场进度。"
      type="warning"
    >
      <template #default>
        <span>
          {{
            overdueOperations
              .map(
                (item) =>
                  `${schoolClassName(item.school_class_id)}（${item.expected_pickup_time}）`,
              )
              .join('、')
          }}
        </span>
      </template>
    </ElAlert>

    <div class="sprout-metric-grid mb-5">
      <article class="sprout-metric-card tone-sky">
        <div class="text-sm text-gray-500">当天任务</div>
        <div class="sprout-metric-value">
          {{ operations.length }}
        </div>
        <div class="sprout-metric-note">按学校班级生成</div>
      </article>
      <article class="sprout-metric-card">
        <div class="text-sm text-gray-500">当前任务已确认</div>
        <div class="sprout-metric-value">
          {{ pickedCount }}
        </div>
        <div class="sprout-metric-note">已登记接送状态</div>
      </article>
      <article class="sprout-metric-card tone-sun">
        <div class="text-sm text-gray-500">当前任务待确认</div>
        <div class="sprout-metric-value">
          {{ pendingCount }}
        </div>
        <div class="sprout-metric-note">逐人点名后自动更新</div>
      </article>
    </div>

    <div
      class="grid gap-5 xl:grid-cols-[minmax(360px,0.9fr)_minmax(600px,1.6fr)]"
    >
      <ElCard class="sprout-table-card" shadow="never">
        <template #header>
          <span class="font-medium">{{ selectedDate }} 接送任务</span>
        </template>
        <div class="sprout-table-wrap">
          <ElTable
            v-loading="loading"
            :data="operations"
            highlight-current-row
            row-key="id"
            @row-click="openOperation"
          >
            <ElTableColumn label="学校班级" min-width="180">
              <template #default="{ row }">
                {{ schoolName((row as PickupOperationRecord).school_id) }} ·
                {{
                  schoolClassName(
                    (row as PickupOperationRecord).school_class_id,
                  )
                }}
              </template>
            </ElTableColumn>
            <ElTableColumn label="托管班" min-width="110">
              <template #default="{ row }">
                {{
                  careClassName((row as PickupOperationRecord).care_class_id)
                }}
              </template>
            </ElTableColumn>
            <ElTableColumn label="老师" min-width="80" prop="teacher_name" />
            <ElTableColumn label="状态" width="90">
              <template #default="{ row }">
                <ElTag
                  :type="
                    operationStatusType((row as PickupOperationRecord).status)
                  "
                >
                  {{
                    operationStatusLabel((row as PickupOperationRecord).status)
                  }}
                </ElTag>
              </template>
            </ElTableColumn>
            <template #empty>
              <ElEmpty description="当天还没有接送任务" :image-size="80" />
            </template>
          </ElTable>
        </div>
      </ElCard>

      <ElCard class="sprout-table-card" shadow="never">
        <template #header>
          <div class="flex items-center justify-between">
            <span class="font-medium">接送名单</span>
            <div v-if="currentOperation" class="flex items-center gap-2">
              <ElTag>
                {{ schoolName(currentOperation.school_id) }} ·
                {{ schoolClassName(currentOperation.school_class_id) }} </ElTag
              ><ElButton
                v-if="currentOperation.status === 'draft'"
                :loading="submitting"
                size="small"
                type="primary"
                @click="confirmOperation"
              >
                确认今日任务 </ElButton
              ><ElButton
                v-if="currentOperation.status === 'confirmed'"
                :loading="submitting"
                size="small"
                type="primary"
                @click="startOperation"
              >
                确认出发 </ElButton
              ><ElButton
                v-if="currentOperation.status === 'started'"
                :loading="handoffLoading"
                size="small"
                @click="openHandoffDialog"
              >
                途中交接 </ElButton
              ><ElButton
                v-if="currentOperation.status === 'started'"
                :loading="submitting"
                size="small"
                type="success"
                @click="finishOperation"
              >
                完成任务
              </ElButton>
            </div>
          </div>
        </template>
        <div v-if="currentOperation" class="sprout-table-wrap">
          <ElTable v-loading="detailLoading" :data="operationStudents" stripe>
            <ElTableColumn label="学生" min-width="100" prop="student_name" />
            <ElTableColumn label="当前状态" width="110">
              <template #default="{ row }">
                <ElTag
                  :type="
                    memberStatusType(
                      (row as PickupOperationStudentRecord).status,
                    )
                  "
                >
                  {{
                    statusLabel((row as PickupOperationStudentRecord).status)
                  }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="确认时间" min-width="170" prop="checked_at" />
            <ElTableColumn label="操作" min-width="290" fixed="right">
              <template #default="{ row }">
                <ElButton
                  v-if="
                    currentOperation?.status === 'started' &&
                    (row as PickupOperationStudentRecord).status !== 'planned'
                  "
                  link
                  type="warning"
                  :loading="correctionLoading"
                  @click="openCorrection(row as PickupOperationStudentRecord)"
                >
                  更正
                </ElButton>
                <template
                  v-if="
                    currentOperation?.status === 'started' &&
                    (row as PickupOperationStudentRecord).status === 'planned'
                  "
                >
                  <ElButton
                    link
                    type="primary"
                    @click="choosePhoto(row as PickupOperationStudentRecord)"
                  >
                    拍照接到 </ElButton
                  ><ElButton
                    link
                    type="success"
                    @click="
                      markStudent(
                        row as PickupOperationStudentRecord,
                        'self_arrived',
                      )
                    "
                  >
                    自行到班 </ElButton
                  ><ElButton
                    link
                    type="warning"
                    @click="
                      markStudent(
                        row as PickupOperationStudentRecord,
                        'parent_picked_up',
                      )
                    "
                  >
                    家长接走 </ElButton
                  ><ElButton
                    link
                    type="danger"
                    @click="
                      markStudent(row as PickupOperationStudentRecord, 'leave')
                    "
                  >
                    请假
                  </ElButton>
                  <ElButton
                    link
                    type="danger"
                    @click="
                      markStudentWithNote(
                        row as PickupOperationStudentRecord,
                        'absent',
                      )
                    "
                  >
                    未找到
                  </ElButton>
                </template>
                <template
                  v-else-if="
                    currentOperation?.status === 'started' &&
                    (row as PickupOperationStudentRecord).status === 'picked_up'
                  "
                >
                  <ElButton
                    v-if="!(row as PickupOperationStudentRecord).photo_url"
                    link
                    type="primary"
                    @click="choosePhoto(row as PickupOperationStudentRecord)"
                  >
                    补传照片
                  </ElButton>
                  <ElButton
                    link
                    type="success"
                    @click="
                      markStudent(
                        row as PickupOperationStudentRecord,
                        'arrived',
                      )
                    "
                  >
                    到班
                  </ElButton>
                  <ElButton
                    link
                    type="danger"
                    @click="
                      markStudentWithNote(
                        row as PickupOperationStudentRecord,
                        'not_arrived',
                      )
                    "
                  >
                    未到班
                  </ElButton>
                  <ElButton
                    link
                    type="warning"
                    @click="
                      markStudentWithNote(
                        row as PickupOperationStudentRecord,
                        'abnormal',
                      )
                    "
                  >
                    异常
                  </ElButton>
                </template>
                <template
                  v-else-if="
                    currentOperation?.status === 'started' &&
                    ((row as PickupOperationStudentRecord).status ===
                      'arrived' ||
                      (row as PickupOperationStudentRecord).status ===
                        'self_arrived')
                  "
                >
                  <ElButton
                    link
                    type="success"
                    @click="
                      markStudentWithNote(
                        row as PickupOperationStudentRecord,
                        'left',
                      )
                    "
                  >
                    已离班
                  </ElButton>
                  <ElButton
                    link
                    type="warning"
                    @click="
                      markStudentWithNote(
                        row as PickupOperationStudentRecord,
                        'midway_left',
                      )
                    "
                  >
                    中途离班
                  </ElButton>
                  <ElButton
                    link
                    type="danger"
                    @click="
                      markStudentWithNote(
                        row as PickupOperationStudentRecord,
                        'abnormal',
                      )
                    "
                  >
                    异常
                  </ElButton>
                </template>
                <a
                  v-if="(row as PickupOperationStudentRecord).photo_url"
                  class="sprout-photo-link ml-2"
                  :href="
                    photoURL((row as PickupOperationStudentRecord).photo_url)
                  "
                  target="_blank"
                >
                  <img
                    alt="接送照片缩略图"
                    class="sprout-photo-thumb"
                    :src="
                      photoURL((row as PickupOperationStudentRecord).photo_url)
                    "
                  />
                  <span>查看照片</span>
                </a>
              </template>
            </ElTableColumn>
            <template #empty>
              <ElEmpty description="没有名单成员" :image-size="70" />
            </template>
          </ElTable>
          <div v-if="handoffs.length" class="mt-4 rounded-lg bg-emerald-50 p-3">
            <div
              class="mb-2 flex items-center justify-between text-sm font-medium text-emerald-900"
            >
              <span>交接记录</span>
              <span class="text-xs font-normal text-emerald-700"
                >{{ handoffs.length }} 次</span
              >
            </div>
            <div
              v-for="handoff in handoffs"
              :key="handoff.id"
              class="border-b border-emerald-100 py-2 text-xs text-emerald-800 last:border-0"
            >
              <div>
                {{ handoff.from_teacher_name || '原执行老师' }} →
                {{ handoff.to_teacher_name || '接手老师' }} ·
                {{ handoff.handoff_at }}
              </div>
              <div v-if="handoff.note" class="mt-1 text-emerald-700">
                {{ handoff.note }}
              </div>
            </div>
          </div>
        </div>
        <ElEmpty v-else description="请先选择左侧接送任务" :image-size="100" />
      </ElCard>
    </div>

    <ElDialog
      v-model="handoffDialogVisible"
      destroy-on-close
      title="途中交接接送任务"
      width="min(520px, 94vw)"
    >
      <ElForm label-position="top" :model="handoffForm">
        <ElFormItem label="接手老师" required>
          <ElSelect v-model="handoffForm.to_teacher_user_id" class="w-full">
            <ElOption
              v-for="teacher in handoffTeachers"
              :key="teacher.teacher_user_id"
              :label="`${teacher.teacher_name || teacher.username}（${teacher.username}）`"
              :value="teacher.teacher_user_id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="接手角色">
          <ElSelect v-model="handoffForm.teacher_role" class="w-full">
            <ElOption label="协作老师" value="collaborator" />
            <ElOption label="代班老师" value="substitute" />
            <ElOption label="主负责人" value="lead" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="交接说明">
          <ElInput
            v-model="handoffForm.note"
            :rows="3"
            type="textarea"
            maxlength="500"
            show-word-limit
            placeholder="例如：王老师临时有事，在校门口完成交接"
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="handoffDialogVisible = false">取消</ElButton>
        <ElButton :loading="submitting" type="primary" @click="submitHandoff">
          确认交接
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="correctionDialogVisible"
      destroy-on-close
      title="更正接送记录"
      width="min(520px, 94vw)"
    >
      <ElForm label-position="top">
        <ElFormItem label="学生">
          <ElInput
            :model-value="correctionStudent?.student_name || ''"
            disabled
          />
        </ElFormItem>
        <ElFormItem label="更正为" required>
          <ElSelect v-model="correctionStatus" class="w-full">
            <ElOption
              v-for="item in correctionStatuses"
              :key="item"
              :label="statusLabel(item)"
              :value="item"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="更正原因" required>
          <ElInput
            v-model="correctionReason"
            :rows="3"
            type="textarea"
            maxlength="500"
            show-word-limit
            placeholder="例如：现场照片误关联，需要改为家长接走"
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="correctionDialogVisible = false">取消</ElButton>
        <ElButton
          :loading="submitting"
          type="primary"
          @click="submitCorrection"
        >
          保存更正
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="dialogVisible"
      destroy-on-close
      title="生成接送任务"
      width="min(620px, 94vw)"
    >
      <ElForm label-position="top" :model="operationForm">
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="接送日期" required>
            <ElDatePicker
              v-model="operationForm.operation_date"
              type="date"
              value-format="YYYY-MM-DD"
              class="w-full"
            /> </ElFormItem
          ><ElFormItem label="负责老师">
            <ElSelect
              v-if="currentRole !== 'teacher'"
              v-model="operationForm.teacher_user_id"
              class="w-full"
              clearable
              placeholder="可不指定，之后再安排"
              @change="syncTeacher"
            >
              <ElOption
                v-for="teacher in assignedTeachersForClass"
                :key="teacher.teacher_user_id"
                :label="`${teacher.teacher_name || teacher.username}（${teacher.username}）`"
                :value="teacher.teacher_user_id"
              />
            </ElSelect>
            <ElInput v-else v-model="operationForm.teacher_name" disabled />
          </ElFormItem>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="学校班级" required>
            <ElSelect v-model="operationForm.school_class_id" class="w-full">
              <ElOption
                v-for="item in availableSchoolClasses"
                :key="item.id"
                :label="`${schoolName(item.school_id)} · ${item.grade}${item.name}`"
                :value="item.id"
              />
            </ElSelect> </ElFormItem
          ><ElFormItem label="托管班">
            <ElSelect
              v-model="operationForm.care_class_id"
              clearable
              class="w-full"
            >
              <ElOption
                v-for="item in careClasses"
                :key="item.id"
                :label="item.name"
                :value="item.id"
              />
            </ElSelect>
          </ElFormItem>
        </div>
        <ElFormItem label="今日确认名单">
          <div
            class="max-h-48 w-full overflow-auto rounded border border-gray-200 p-3"
          >
            <ElCheckboxGroup
              v-model="operationForm.student_ids"
              class="grid grid-cols-2 gap-2"
            >
              <ElCheckbox
                v-for="student in candidateStudents"
                :key="student.id"
                :label="student.id"
              >
                {{ student.name }}
              </ElCheckbox> </ElCheckboxGroup
            ><ElEmpty
              v-if="candidateStudents.length === 0"
              description="该班级暂无在托学生"
              :image-size="50"
            />
          </div>
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="operationForm.notes" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton
        ><ElButton
          :disabled="operationForm.student_ids.length === 0"
          :loading="submitting"
          type="primary"
          @click="createOperation"
        >
          确认生成
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
