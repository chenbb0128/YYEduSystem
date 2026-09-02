<script lang="ts" setup>
import type {
  HomeworkStudentStatus,
  HomeworkTaskRecord,
  HomeworkTaskStudentRecord,
} from '#/api/homework';
import type {
  SchoolClassRecord,
  SchoolRecord,
  StudentRecord,
} from '#/api/master-data';
import type { TeacherAssignmentRecord } from '#/api/teacher-assignments';

import { computed, onMounted, reactive, ref, watch } from 'vue';

import { useUserStore } from '@vben/stores';

import {
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
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import {
  createHomeworkTaskApi,
  getHomeworkTasksApi,
  getHomeworkTaskStudentsApi,
  reviewHomeworkStudentApi,
  uploadHomeworkPhotoApi,
} from '#/api/homework';
import {
  getSchoolClassesApi,
  getSchoolsApi,
  getStudentsApi,
} from '#/api/master-data';
import { getTeacherAssignmentsApi } from '#/api/teacher-assignments';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'Homework' });

const today = businessToday();
const selectedDate = ref(today);
const loading = ref(false);
const detailLoading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const dialogVisible = ref(false);
const attachmentInput = ref<HTMLInputElement>();
const attachmentUploading = ref(false);
const attachmentNames = ref<string[]>([]);
const tasks = ref<HomeworkTaskRecord[]>([]);
const currentTask = ref<HomeworkTaskRecord | null>(null);
const taskStudents = ref<HomeworkTaskStudentRecord[]>([]);
const schools = ref<SchoolRecord[]>([]);
const schoolClasses = ref<SchoolClassRecord[]>([]);
const students = ref<StudentRecord[]>([]);
const assignments = ref<TeacherAssignmentRecord[]>([]);
const reviewNotes = reactive<Record<number, string>>({});
const userStore = useUserStore();

const currentRole = computed(() => userStore.userInfo?.roles?.[0] || '');
const availableSchoolClasses = computed(() => {
  const activeClasses = schoolClasses.value.filter(
    (item) => item.status === 'active',
  );
  if (currentRole.value !== 'teacher') return activeClasses;
  const assigned = new Set(
    assignments.value
      .filter((item) => item.status === 'active')
      .map((item) => item.school_class_id),
  );
  return activeClasses.filter((item) => assigned.has(item.id));
});

const homeworkForm = reactive({
  homework_date: today,
  school_class_id: 0,
  subject: '综合作业',
  content: '',
  attachment_urls: [] as string[],
  student_ids: [] as number[],
});

const candidateStudents = computed(() =>
  students.value.filter(
    (item) =>
      item.status === 'active' &&
      item.school_class_id === homeworkForm.school_class_id,
  ),
);

function schoolName(id: number) {
  return schools.value.find((item) => item.id === id)?.name || `学校 ${id}`;
}

function className(id: number) {
  const item = schoolClasses.value.find((schoolClass) => schoolClass.id === id);
  return item
    ? `${schoolName(item.school_id)} · ${item.grade}${item.name}`
    : `班级 ${id}`;
}

function statusLabel(status: HomeworkStudentStatus) {
  return {
    completed: '已完成',
    incomplete: '需订正',
    not_submitted: '未提交',
    pending: '待批改',
  }[status];
}

function statusType(status: HomeworkStudentStatus) {
  if (status === 'completed') return 'success';
  if (status === 'incomplete' || status === 'not_submitted') return 'danger';
  return 'warning';
}

async function loadReferences() {
  try {
    const [schoolResult, classResult, studentResult, assignmentResult] =
      await Promise.all([
        getSchoolsApi(),
        getSchoolClassesApi(),
        getStudentsApi(),
        getTeacherAssignmentsApi(),
      ]);
    schools.value = schoolResult.items;
    schoolClasses.value = classResult.items;
    students.value = studentResult.items;
    assignments.value = assignmentResult.items;
  } catch {
    loadError.value = '作业相关班级数据加载失败，请稍后重试。';
  }
}

async function loadTasks() {
  loading.value = true;
  loadError.value = '';
  try {
    const result = await getHomeworkTasksApi({ date: selectedDate.value });
    tasks.value = result.items;
    if (currentTask.value) {
      const refreshed = tasks.value.find(
        (item) => item.id === currentTask.value?.id,
      );
      if (refreshed) await openTask(refreshed);
      else {
        currentTask.value = null;
        taskStudents.value = [];
      }
    }
  } catch {
    tasks.value = [];
    currentTask.value = null;
    taskStudents.value = [];
    loadError.value = '作业记录加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

async function openTask(task: HomeworkTaskRecord) {
  currentTask.value = task;
  detailLoading.value = true;
  try {
    const result = await getHomeworkTaskStudentsApi(task.id);
    taskStudents.value = result.items;
    for (const student of result.items) {
      reviewNotes[student.student_id] = student.correction_note;
    }
  } catch (error) {
    taskStudents.value = [];
    ElMessage.error(
      error instanceof Error ? error.message : '学生作业加载失败',
    );
  } finally {
    detailLoading.value = false;
  }
}

function openCreateDialog() {
  Object.assign(homeworkForm, {
    homework_date: selectedDate.value,
    school_class_id: availableSchoolClasses.value[0]?.id || 0,
    subject: '综合作业',
    content: '',
    attachment_urls: [],
    student_ids: [],
  });
  attachmentNames.value = [];
  syncStudents();
  dialogVisible.value = true;
}

function openAttachmentPicker() {
  attachmentInput.value?.click();
}

async function handleAttachments(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = [...(input.files || [])];
  input.value = '';
  const available = Math.max(0, 9 - homeworkForm.attachment_urls.length);
  if (files.length > available) {
    ElMessage.warning('最多上传 9 张作业图片');
  }
  const selected = files.slice(0, available);
  if (selected.length === 0) return;
  attachmentUploading.value = true;
  try {
    for (const file of selected) {
      const asset = await uploadHomeworkPhotoApi(file, {
        task_id: currentTask.value?.id,
      });
      homeworkForm.attachment_urls.push(asset.url);
      attachmentNames.value.push(file.name);
    }
    ElMessage.success(`已上传 ${selected.length} 张作业图片`);
  } finally {
    attachmentUploading.value = false;
  }
}

function removeAttachment(index: number) {
  homeworkForm.attachment_urls.splice(index, 1);
  attachmentNames.value.splice(index, 1);
}

function syncStudents() {
  homeworkForm.student_ids = candidateStudents.value.map((item) => item.id);
}

watch(() => homeworkForm.school_class_id, syncStudents);

async function createTask() {
  if (!homeworkForm.content.trim()) {
    ElMessage.warning('请填写作业内容');
    return;
  }
  submitting.value = true;
  try {
    const created = await createHomeworkTaskApi({
      ...homeworkForm,
      student_ids:
        homeworkForm.student_ids.length > 0
          ? homeworkForm.student_ids
          : undefined,
    });
    dialogVisible.value = false;
    ElMessage.success('作业已布置');
    await loadTasks();
    await openTask(created);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '作业布置失败');
  } finally {
    submitting.value = false;
  }
}

async function reviewStudent(
  student: HomeworkTaskStudentRecord,
  status: Exclude<HomeworkStudentStatus, 'pending'>,
) {
  if (!currentTask.value) return;
  submitting.value = true;
  try {
    await reviewHomeworkStudentApi(currentTask.value.id, student.student_id, {
      status,
      correction_note: reviewNotes[student.student_id] || '',
    });
    await openTask(currentTask.value);
    ElMessage.success(`${student.student_name}：${statusLabel(status)}`);
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  await loadReferences();
  await loadTasks();
});
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">学习陪伴 · 班级作业</p>
        <h1 class="sprout-page-title">作业管理</h1>
        <p class="sprout-page-description">
          可按学校班级一次布置，个别学生也可以单独添加；批改结果会同步到家长端。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElDatePicker
          v-model="selectedDate"
          type="date"
          value-format="YYYY-MM-DD"
          @change="loadTasks"
        />
        <ElButton
          v-access:code="'homework:write'"
          type="primary"
          @click="openCreateDialog"
        >
          布置作业
        </ElButton>
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

    <div
      class="grid gap-5 xl:grid-cols-[minmax(360px,0.9fr)_minmax(600px,1.6fr)]"
    >
      <ElCard class="sprout-table-card" shadow="never">
        <template #header>{{ selectedDate }} 作业记录</template>
        <div class="sprout-table-wrap">
          <ElTable
            v-loading="loading"
            :data="tasks"
            row-key="id"
            highlight-current-row
            @row-click="openTask"
          >
            <ElTableColumn label="学校班级" min-width="190">
              <template #default="{ row }">
                {{ className((row as HomeworkTaskRecord).school_class_id) }}
              </template>
            </ElTableColumn>
            <ElTableColumn label="科目" width="100" prop="subject" />
            <ElTableColumn
              label="作业内容"
              min-width="220"
              prop="content"
              show-overflow-tooltip
            />
            <ElTableColumn label="布置人" width="90" prop="creator_name" />
            <template #empty>
              <ElEmpty description="当天还没有作业" :image-size="80" />
            </template>
          </ElTable>
        </div>
      </ElCard>

      <ElCard class="sprout-table-card" shadow="never">
        <template #header>
          <div class="flex items-center justify-between">
            <span class="font-medium">学生完成情况</span>
            <div v-if="currentTask" class="flex flex-wrap items-center gap-2">
              <ElTag>
                {{ currentTask.subject }} · {{ currentTask.content }}
              </ElTag>
              <ElTag v-if="currentTask.attachment_urls.length" type="info">
                {{ currentTask.attachment_urls.length }} 张图片
              </ElTag>
            </div>
          </div>
        </template>
        <div v-if="currentTask" class="sprout-table-wrap">
          <ElTable v-loading="detailLoading" :data="taskStudents" stripe>
            <ElTableColumn label="学生" width="100" prop="student_name" />
            <ElTableColumn label="状态" width="100">
              <template #default="{ row }">
                <ElTag
                  :type="statusType((row as HomeworkTaskStudentRecord).status)"
                >
                  {{ statusLabel((row as HomeworkTaskStudentRecord).status) }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="批改意见" min-width="190">
              <template #default="{ row }">
                <ElInput
                  v-model="
                    reviewNotes[(row as HomeworkTaskStudentRecord).student_id]
                  "
                  placeholder="可填写意见"
                />
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="220" fixed="right">
              <template #default="{ row }">
                <ElButton
                  v-access:code="'homework:write'"
                  link
                  type="success"
                  @click="
                    reviewStudent(row as HomeworkTaskStudentRecord, 'completed')
                  "
                >
                  完成
                </ElButton>
                <ElButton
                  v-access:code="'homework:write'"
                  link
                  type="warning"
                  @click="
                    reviewStudent(
                      row as HomeworkTaskStudentRecord,
                      'incomplete',
                    )
                  "
                >
                  订正
                </ElButton>
                <ElButton
                  v-access:code="'homework:write'"
                  link
                  type="danger"
                  @click="
                    reviewStudent(
                      row as HomeworkTaskStudentRecord,
                      'not_submitted',
                    )
                  "
                >
                  未交
                </ElButton>
              </template>
            </ElTableColumn>
            <template #empty>
              <ElEmpty description="没有学生名单" :image-size="70" />
            </template>
          </ElTable>
        </div>
        <ElEmpty v-else description="请先选择左侧作业" :image-size="100" />
      </ElCard>
    </div>

    <ElDialog v-model="dialogVisible" title="布置作业" width="min(620px, 94vw)">
      <ElForm label-position="top" :model="homeworkForm">
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="作业日期" required>
            <ElDatePicker
              v-model="homeworkForm.homework_date"
              type="date"
              value-format="YYYY-MM-DD"
              class="w-full"
            />
          </ElFormItem>
          <ElFormItem label="科目">
            <ElInput v-model="homeworkForm.subject" placeholder="例如：语文" />
          </ElFormItem>
        </div>
        <ElFormItem label="学校班级" required>
          <ElSelect v-model="homeworkForm.school_class_id" class="w-full">
            <ElOption
              v-for="item in availableSchoolClasses"
              :key="item.id"
              :label="className(item.id)"
              :value="item.id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="作业内容" required>
          <ElInput
            v-model="homeworkForm.content"
            type="textarea"
            :rows="4"
            placeholder="填写今天要完成的作业"
          />
        </ElFormItem>
        <ElFormItem label="作业图片（可选，最多 9 张）">
          <input
            ref="attachmentInput"
            accept="image/jpeg,image/png,image/webp"
            class="hidden"
            multiple
            type="file"
            @change="handleAttachments"
          />
          <div class="flex flex-wrap items-center gap-2">
            <ElButton
              :loading="attachmentUploading"
              :disabled="homeworkForm.attachment_urls.length >= 9"
              @click="openAttachmentPicker"
            >
              上传图片
            </ElButton>
            <span class="text-xs text-gray-500">
              图片会同步展示给家长，单张不超过 5MB
            </span>
          </div>
          <div v-if="attachmentNames.length" class="mt-2 space-y-1">
            <div
              v-for="(name, index) in attachmentNames"
              :key="`${name}-${index}`"
              class="flex items-center justify-between rounded bg-gray-50 px-2 py-1 text-xs"
            >
              <span class="truncate">{{ name }}</span>
              <ElButton link type="danger" @click="removeAttachment(index)">
                移除
              </ElButton>
            </div>
          </div>
        </ElFormItem>
        <ElFormItem label="布置学生">
          <div
            class="max-h-48 w-full overflow-auto rounded border border-gray-200 p-3"
          >
            <ElCheckboxGroup
              v-model="homeworkForm.student_ids"
              class="grid grid-cols-2 gap-2"
            >
              <ElCheckbox
                v-for="student in candidateStudents"
                :key="student.id"
                :label="student.id"
              >
                {{ student.name }}
              </ElCheckbox>
            </ElCheckboxGroup>
            <ElEmpty
              v-if="candidateStudents.length === 0"
              description="该班级暂无在托学生"
              :image-size="50"
            />
          </div>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton
          :disabled="homeworkForm.student_ids.length === 0"
          :loading="submitting"
          type="primary"
          @click="createTask"
        >
          确认布置
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
