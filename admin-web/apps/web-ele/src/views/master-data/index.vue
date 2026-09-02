<script lang="ts" setup>
import type {
  AcademicTermRecord,
  CareClassRecord,
  SchoolClassRecord,
  SchoolRecord,
  StudentProfilePayload,
  StudentRecord,
} from '#/api/master-data';

import { computed, onMounted, reactive, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
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
  createStudentProfileApi,
  getAcademicTermsApi,
  getCareClassesApi,
  getSchoolClassesApi,
  getSchoolsApi,
  getStudentsApi,
  updateStudentProfileApi,
} from '#/api/master-data';

defineOptions({ name: 'MasterData' });

const loading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const dialogVisible = ref(false);
const editingStudentId = ref<null | number>(null);

const schools = ref<SchoolRecord[]>([]);
const terms = ref<AcademicTermRecord[]>([]);
const schoolClasses = ref<SchoolClassRecord[]>([]);
const careClasses = ref<CareClassRecord[]>([]);
const students = ref<StudentRecord[]>([]);

const emptyProfile = (): StudentProfilePayload => ({
  school_name: '',
  grade: '',
  class_name: '',
  care_class_name: '',
  name: '',
  gender: 'unknown',
  birth_date: '',
  student_no: '',
  guardian_phone: '',
  emergency_contact: '',
  emergency_phone: '',
  notes: '',
});
const profileForm = reactive<StudentProfilePayload>(emptyProfile());
const studentQuery = reactive({
  keyword: '',
  school_id: undefined as number | undefined,
  school_class_id: undefined as number | undefined,
  care_class_id: undefined as number | undefined,
  status: undefined as StudentRecord['status'] | undefined,
});

const currentTerm = computed(
  () => terms.value.find((item) => item.is_current) || terms.value[0],
);
const filteredSchoolClasses = computed(() =>
  schoolClasses.value.filter(
    (item) =>
      !studentQuery.school_id || item.school_id === studentQuery.school_id,
  ),
);
const gradeOptions = computed(() =>
  [
    ...new Set(schoolClasses.value.map((item) => item.grade).filter(Boolean)),
  ].toSorted(),
);
const classNameOptions = computed(() =>
  [
    ...new Set(schoolClasses.value.map((item) => item.name).filter(Boolean)),
  ].toSorted(),
);

function schoolName(id: number) {
  return schools.value.find((item) => item.id === id)?.name || `学校 ${id}`;
}

function termName(id: number) {
  return terms.value.find((item) => item.id === id)?.name || `学期 ${id}`;
}

function schoolClassLabel(id: number) {
  const item = schoolClasses.value.find((schoolClass) => schoolClass.id === id);
  return item ? `${item.grade} · ${item.name}` : `班级 ${id}`;
}

function careClassName(id?: number) {
  return id
    ? careClasses.value.find((item) => item.id === id)?.name || `托管班 ${id}`
    : '未分组';
}

function studentClass(row: StudentRecord) {
  return schoolClassLabel(row.school_class_id);
}

function profileTermLabel() {
  return profileForm.term_id
    ? termName(profileForm.term_id)
    : currentTerm.value?.name || '保存时自动生成当前学期';
}

async function loadAll() {
  loading.value = true;
  loadError.value = '';
  try {
    const [schoolResult, termResult, schoolClassResult, careClassResult] =
      await Promise.all([
        getSchoolsApi(),
        getAcademicTermsApi(),
        getSchoolClassesApi(),
        getCareClassesApi(),
      ]);
    schools.value = schoolResult.items;
    terms.value = termResult.items;
    schoolClasses.value = schoolClassResult.items;
    careClasses.value = careClassResult.items;
    await loadStudents();
  } catch {
    loadError.value = '档案数据加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

async function loadStudents() {
  try {
    const result = await getStudentsApi({
      keyword: studentQuery.keyword.trim() || undefined,
      school_id: studentQuery.school_id,
      school_class_id: studentQuery.school_class_id,
      care_class_id: studentQuery.care_class_id,
      status: studentQuery.status,
    });
    students.value = result.items;
    loadError.value = '';
  } catch {
    students.value = [];
    loadError.value = '学生档案加载失败，请稍后重试。';
  }
}

function resetFilters() {
  Object.assign(studentQuery, {
    keyword: '',
    school_id: undefined,
    school_class_id: undefined,
    care_class_id: undefined,
    status: undefined,
  });
  void loadStudents();
}

function handleSchoolFilterChange() {
  if (
    studentQuery.school_class_id &&
    !filteredSchoolClasses.value.some(
      (item) => item.id === studentQuery.school_class_id,
    )
  ) {
    studentQuery.school_class_id = undefined;
  }
  void loadStudents();
}

function resetDialog() {
  dialogVisible.value = false;
  editingStudentId.value = null;
}

function openCreate() {
  editingStudentId.value = null;
  Object.assign(profileForm, emptyProfile());
  dialogVisible.value = true;
}

function openEditStudent(row: StudentRecord) {
  const schoolClass = schoolClasses.value.find(
    (item) => item.id === row.school_class_id,
  );
  editingStudentId.value = row.id;
  Object.assign(profileForm, {
    ...emptyProfile(),
    term_id: row.term_id,
    school_name: schoolName(row.school_id),
    grade: schoolClass?.grade || '',
    class_name: schoolClass?.name || '',
    care_class_name: careClassName(row.care_class_id),
    name: row.name,
    gender: row.gender,
    birth_date: row.birth_date || '',
    student_no: row.student_no,
    guardian_phone: row.guardian_phone,
    emergency_contact: row.emergency_contact,
    emergency_phone: row.emergency_phone,
    status: row.status,
    notes: row.notes,
  });
  if (!row.care_class_id) profileForm.care_class_name = '';
  dialogVisible.value = true;
}

async function submitDialog() {
  if (
    !profileForm.name.trim() ||
    !profileForm.school_name.trim() ||
    !profileForm.grade.trim() ||
    !profileForm.class_name.trim()
  ) {
    ElMessage.warning('请填写学生姓名、学校、年级和班级');
    return;
  }
  const payload: StudentProfilePayload = {
    ...profileForm,
    school_name: profileForm.school_name.trim(),
    grade: profileForm.grade.trim(),
    class_name: profileForm.class_name.trim(),
    care_class_name: profileForm.care_class_name?.trim() || '',
    name: profileForm.name.trim(),
  };
  submitting.value = true;
  try {
    if (editingStudentId.value) {
      await updateStudentProfileApi(editingStudentId.value, payload);
      ElMessage.success('学生档案已更新');
    } else {
      await createStudentProfileApi(payload);
      ElMessage.success('学生档案已新增，分类已自动归档');
    }
    resetDialog();
    await loadAll();
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '学生档案保存失败',
    );
  } finally {
    submitting.value = false;
  }
}

onMounted(loadAll);
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">档案中心 · 学生成长档案</p>
        <h1 class="sprout-page-title">学生档案</h1>
        <p class="sprout-page-description">
          只维护一份学生档案；学校、年级、班级和托管班在保存时自动复用或创建。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton type="primary" @click="openCreate">新增学生</ElButton>
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
      <ElAlert
        class="mb-4"
        :closable="false"
        show-icon
        title="日常不需要先新增学校、学期或班级，直接填写学生档案即可。分类数据仅用于筛选、接送、作业和历史追溯。"
        type="info"
      />
      <div class="sprout-filter-panel mb-4">
        <ElInput
          v-model="studentQuery.keyword"
          clearable
          class="w-56"
          placeholder="搜索学生姓名/编号"
          @keyup.enter="loadStudents"
        />
        <ElSelect
          v-model="studentQuery.school_id"
          clearable
          class="w-48"
          placeholder="按学校筛选"
          @change="handleSchoolFilterChange"
        >
          <ElOption
            v-for="item in schools"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </ElSelect>
        <ElSelect
          v-model="studentQuery.school_class_id"
          clearable
          class="w-56"
          placeholder="按学校班级筛选"
          @change="loadStudents"
        >
          <ElOption
            v-for="item in filteredSchoolClasses"
            :key="item.id"
            :label="`${schoolName(item.school_id)} · ${item.grade}${item.name}`"
            :value="item.id"
          />
        </ElSelect>
        <ElSelect
          v-model="studentQuery.care_class_id"
          clearable
          class="w-44"
          placeholder="按托管班筛选"
          @change="loadStudents"
        >
          <ElOption
            v-for="item in careClasses"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </ElSelect>
        <ElSelect
          v-model="studentQuery.status"
          clearable
          class="w-32"
          placeholder="状态"
          @change="loadStudents"
        >
          <ElOption label="在托" value="active" />
          <ElOption label="停托" value="inactive" />
        </ElSelect>
        <ElButton @click="loadStudents">查询</ElButton>
        <ElButton text @click="resetFilters">重置</ElButton>
      </div>

      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="students" stripe>
          <ElTableColumn label="学生" min-width="120" prop="name" />
          <ElTableColumn label="学校" min-width="150">
            <template #default="{ row }">
              {{ schoolName((row as StudentRecord).school_id) }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="年级·班级" min-width="140">
            <template #default="{ row }">
              {{ studentClass(row as StudentRecord) }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="托管班" min-width="120">
            <template #default="{ row }">
              {{ careClassName((row as StudentRecord).care_class_id) }}
            </template>
          </ElTableColumn>
          <ElTableColumn
            label="家长电话"
            min-width="140"
            prop="guardian_phone"
          />
          <ElTableColumn label="状态" width="90">
            <template #default="{ row }">
              <ElTag
                :type="
                  (row as StudentRecord).status === 'active'
                    ? 'success'
                    : 'info'
                "
              >
                {{
                  (row as StudentRecord).status === 'active' ? '在托' : '停托'
                }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <ElButton
                link
                type="primary"
                @click="openEditStudent(row as StudentRecord)"
              >
                编辑
              </ElButton>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty
              description="还没有学生档案，请直接新增"
              :image-size="80"
            />
          </template>
        </ElTable>
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      destroy-on-close
      :title="editingStudentId ? '编辑学生档案' : '新增学生档案'"
      width="min(620px, 92vw)"
      @closed="resetDialog"
    >
      <ElForm label-position="top" :model="profileForm">
        <ElAlert
          class="mb-4"
          :closable="false"
          show-icon
          :title="`当前学期：${profileTermLabel()}；新学校、班级和托管班名称保存时会自动归类。`"
          type="success"
        />
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="学生姓名" required>
            <ElInput v-model="profileForm.name" placeholder="例如：小明" />
          </ElFormItem>
          <ElFormItem label="学号/编号">
            <ElInput v-model="profileForm.student_no" />
          </ElFormItem>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="学校" required>
            <ElSelect
              v-model="profileForm.school_name"
              allow-create
              class="w-full"
              default-first-option
              filterable
              placeholder="可直接输入新学校"
            >
              <ElOption
                v-for="item in schools"
                :key="item.id"
                :label="item.name"
                :value="item.name"
              />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="当前学期">
            <ElInput :model-value="profileTermLabel()" disabled />
          </ElFormItem>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="年级" required>
            <ElSelect
              v-model="profileForm.grade"
              allow-create
              class="w-full"
              default-first-option
              filterable
              placeholder="例如：三年级"
            >
              <ElOption
                v-for="grade in gradeOptions"
                :key="grade"
                :label="grade"
                :value="grade"
              />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="学校班级" required>
            <ElSelect
              v-model="profileForm.class_name"
              allow-create
              class="w-full"
              default-first-option
              filterable
              placeholder="例如：1班"
            >
              <ElOption
                v-for="className in classNameOptions"
                :key="className"
                :label="className"
                :value="className"
              />
            </ElSelect>
          </ElFormItem>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="托管班（可选）">
            <ElSelect
              v-model="profileForm.care_class_name"
              allow-create
              clearable
              class="w-full"
              default-first-option
              filterable
              placeholder="可留空，保存时自动归类"
            >
              <ElOption
                v-for="item in careClasses"
                :key="item.id"
                :label="item.name"
                :value="item.name"
              />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="性别">
            <ElSelect v-model="profileForm.gender" class="w-full">
              <ElOption label="未设置" value="unknown" />
              <ElOption label="男" value="male" />
              <ElOption label="女" value="female" />
            </ElSelect>
          </ElFormItem>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="出生日期">
            <ElDatePicker
              v-model="profileForm.birth_date"
              type="date"
              value-format="YYYY-MM-DD"
              class="w-full"
            />
          </ElFormItem>
          <ElFormItem label="家长电话">
            <ElInput v-model="profileForm.guardian_phone" />
          </ElFormItem>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="紧急联系人">
            <ElInput v-model="profileForm.emergency_contact" />
          </ElFormItem>
          <ElFormItem label="紧急联系电话">
            <ElInput v-model="profileForm.emergency_phone" />
          </ElFormItem>
        </div>
        <ElFormItem v-if="editingStudentId" label="托管状态">
          <ElSelect v-model="profileForm.status" class="w-full">
            <ElOption label="在托" value="active" />
            <ElOption label="停托" value="inactive" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="profileForm.notes" type="textarea" :rows="3" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="resetDialog">取消</ElButton>
        <ElButton :loading="submitting" type="primary" @click="submitDialog"
          >保存</ElButton
        >
      </template>
    </ElDialog>
  </div>
</template>
