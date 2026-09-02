<script lang="ts" setup>
import type { SchoolClassRecord, SchoolRecord } from '#/api/master-data';
import type { SystemUserRecord } from '#/api/system/user';
import type {
  TeacherAssignmentRecord,
  TeacherAssignmentStatus,
} from '#/api/teacher-assignments';

import { computed, onMounted, reactive, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElMessage,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import { getSchoolClassesApi, getSchoolsApi } from '#/api/master-data';
import { getSystemUsersApi } from '#/api/system/user';
import {
  createTeacherAssignmentApi,
  getTeacherAssignmentsApi,
  updateTeacherAssignmentApi,
} from '#/api/teacher-assignments';

defineOptions({ name: 'TeacherAssignments' });

const loading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const dialogVisible = ref(false);
const assignments = ref<TeacherAssignmentRecord[]>([]);
const teachers = ref<SystemUserRecord[]>([]);
const schoolClasses = ref<SchoolClassRecord[]>([]);
const schools = ref<SchoolRecord[]>([]);
const form = reactive({ teacher_user_id: 0, school_class_id: 0 });

const availableTeachers = computed(() =>
  teachers.value.filter(
    (user) => user.role === 'teacher' && user.status === 'active',
  ),
);
const activeClasses = computed(() =>
  schoolClasses.value.filter((item) => item.status === 'active'),
);

function schoolName(id: number) {
  return schools.value.find((item) => item.id === id)?.name || `学校 ${id}`;
}

function classLabel(item: SchoolClassRecord) {
  return `${schoolName(item.school_id)} · ${item.grade}${item.name}`;
}

function statusLabel(status: TeacherAssignmentStatus) {
  return status === 'active' ? '已启用' : '已停用';
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const [assignmentResult, userResult, classResult, schoolResult] =
      await Promise.all([
        getTeacherAssignmentsApi(),
        getSystemUsersApi({ page: 1, pageSize: 100 }),
        getSchoolClassesApi(),
        getSchoolsApi(),
      ]);
    assignments.value = assignmentResult.items;
    teachers.value = userResult.items;
    schoolClasses.value = classResult.items;
    schools.value = schoolResult.items;
  } catch {
    assignments.value = [];
    loadError.value = '教师班级数据加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

function openCreateDialog() {
  Object.assign(form, {
    school_class_id: activeClasses.value[0]?.id || 0,
    teacher_user_id: availableTeachers.value[0]?.id || 0,
  });
  dialogVisible.value = true;
}

async function submitForm() {
  if (!form.teacher_user_id || !form.school_class_id) {
    ElMessage.warning('请选择教师和学校班级');
    return;
  }
  submitting.value = true;
  try {
    await createTeacherAssignmentApi({ ...form });
    ElMessage.success('教师负责班级已保存');
    dialogVisible.value = false;
    await loadData();
  } finally {
    submitting.value = false;
  }
}

async function toggleAssignment(row: TeacherAssignmentRecord) {
  submitting.value = true;
  try {
    const status = row.status === 'active' ? 'disabled' : 'active';
    await updateTeacherAssignmentApi(row.id, { status });
    ElMessage.success(status === 'active' ? '分配已启用' : '分配已停用');
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
        <p class="sprout-page-kicker">组织协作 · 教师负责关系</p>
        <h1 class="sprout-page-title">教师负责班级</h1>
        <p class="sprout-page-description">
          为教师分配负责的学校班级。教师登录后只能查看和操作自己负责的班级。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton :loading="loading" @click="loadData">刷新</ElButton>
        <ElButton
          v-access:code="'assignment:write'"
          type="primary"
          @click="openCreateDialog"
        >
          新增分配
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

    <ElCard class="sprout-table-card" shadow="never">
      <div class="sprout-table-toolbar mb-4">
        <div>
          <h2 class="sprout-section-title">负责关系清单</h2>
          <p class="sprout-section-caption">
            启用后，教师可在接送和作业页面看到对应班级。
          </p>
        </div>
        <span class="sprout-role-badge">{{ assignments.length }} 条分配</span>
      </div>
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="assignments" row-key="id" stripe>
          <ElTableColumn label="教师" min-width="180">
            <template #default="{ row }">
              <div class="font-medium">
                {{ row.teacher_name || row.username }}
              </div>
              <div class="text-xs text-gray-400">{{ row.username }}</div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="学校班级" min-width="240">
            <template #default="{ row }">
              {{ schoolName(row.school_id) }} · {{ row.grade
              }}{{ row.class_name }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="100">
            <template #default="{ row }">
              <ElTag :type="row.status === 'active' ? 'success' : 'info'">
                {{ statusLabel(row.status) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <ElButton
                v-access:code="'assignment:write'"
                :loading="submitting"
                link
                :type="row.status === 'active' ? 'danger' : 'success'"
                @click="toggleAssignment(row as TeacherAssignmentRecord)"
              >
                {{ row.status === 'active' ? '停用' : '启用' }}
              </ElButton>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="还没有教师班级分配" :image-size="90" />
          </template>
        </ElTable>
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      title="新增教师负责班级"
      width="min(520px, 92vw)"
    >
      <ElForm label-position="top" :model="form">
        <ElFormItem label="教师" required>
          <ElSelect v-model="form.teacher_user_id" class="w-full">
            <ElOption
              v-for="teacher in availableTeachers"
              :key="teacher.id"
              :label="`${teacher.realName || teacher.username}（${teacher.username}）`"
              :value="teacher.id"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="学校班级" required>
          <ElSelect v-model="form.school_class_id" class="w-full">
            <ElOption
              v-for="item in activeClasses"
              :key="item.id"
              :label="classLabel(item)"
              :value="item.id"
            />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton :loading="submitting" type="primary" @click="submitForm">
          保存
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
