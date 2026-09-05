<script lang="ts" setup>
import type {
  AcademicTermRecord,
  CareClassRecord,
  SchoolClassRecord,
  SchoolRecord,
  StudentImportIssue,
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
  importStudentsApi,
  updateStudentProfileApi,
} from '#/api/master-data';

defineOptions({ name: 'MasterData' });

const loading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const dialogVisible = ref(false);
const importDialogVisible = ref(false);
const editingStudentId = ref<null | number>(null);
const importing = ref(false);
const importText = ref('');
const importResult = ref<null | {
  created: StudentRecord[];
  invalid: StudentImportIssue[];
  skipped_duplicates: StudentImportIssue[];
}>(null);

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

function openImport() {
  importText.value = '';
  importResult.value = null;
  importDialogVisible.value = true;
}

function parseCSV(text: string) {
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = '';
  let quoted = false;
  const source = text.replace(/^\uFEFF/, '');
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index];
    const next = source[index + 1];
    if (char === '"') {
      if (quoted && next === '"') {
        cell += '"';
        index += 1;
      } else {
        quoted = !quoted;
      }
    } else if (char === ',' && !quoted) {
      row.push(cell.trim());
      cell = '';
    } else if ((char === '\n' || char === '\r') && !quoted) {
      if (char === '\r' && next === '\n') index += 1;
      row.push(cell.trim());
      if (row.some(Boolean)) rows.push(row);
      row = [];
      cell = '';
    } else {
      cell += char;
    }
  }
  if (cell || row.length > 0) {
    row.push(cell.trim());
    if (row.some(Boolean)) rows.push(row);
  }
  return rows;
}

const importHeaderAliases: Record<string, keyof StudentProfilePayload> = {
  学生姓名: 'name',
  姓名: 'name',
  学校: 'school_name',
  年级: 'grade',
  班级: 'class_name',
  学校班级: 'class_name',
  托管班: 'care_class_name',
  家长电话: 'guardian_phone',
  性别: 'gender',
  出生日期: 'birth_date',
  学号: 'student_no',
  学号编号: 'student_no',
  紧急联系人: 'emergency_contact',
  紧急电话: 'emergency_phone',
  紧急联系电话: 'emergency_phone',
  备注: 'notes',
};

function parseImportRows(text: string): StudentProfilePayload[] {
  const rows = parseCSV(text);
  if (rows.length < 2)
    throw new Error('请上传包含表头和至少一名学生的 CSV 文件');
  const firstRow = rows[0];
  if (!firstRow) throw new Error('导入文件缺少表头');
  const headers = firstRow.map((value) => importHeaderAliases[value] || '');
  const fieldLabels: Record<string, string> = {
    class_name: '班级',
    grade: '年级',
    name: '学生姓名',
    school_name: '学校',
  };
  for (const field of ['name', 'school_name', 'grade', 'class_name'] as const) {
    if (!headers.includes(field))
      throw new Error(`导入文件缺少“${fieldLabels[field]}”列`);
  }
  return rows.slice(1).map((values) => {
    const item = { ...emptyProfile() };
    const textItem = item as unknown as Record<string, string>;
    headers.forEach((field, index) => {
      if (field) textItem[field] = values[index] || '';
    });
    const gender = String(item.gender).trim();
    if (gender === '男') item.gender = 'male';
    else if (gender === '女') item.gender = 'female';
    else item.gender = 'unknown';
    return item;
  });
}

async function handleImportFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file) return;
  try {
    importText.value = await file.text();
    ElMessage.success(`已读取 ${file.name}，请确认后导入`);
  } catch {
    ElMessage.error('文件读取失败，请使用 UTF-8 编码的 CSV 文件');
  }
}

function downloadImportTemplate() {
  const content =
    '学生姓名,学校,年级,班级,托管班,家长电话,性别,出生日期,学号,紧急联系人,紧急电话,备注\n小明,实验小学,三年级,1班,晚托一班,13800000000,男,2017-01-01,,妈妈,13900000000,示例行';
  const blob = new Blob([`\uFEFF${content}`], {
    type: 'text/csv;charset=utf-8',
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = '豆芽成长助手-学生导入模板.csv';
  link.click();
  URL.revokeObjectURL(url);
}

async function submitImport() {
  if (importing.value) return;
  let items: StudentProfilePayload[];
  try {
    items = parseImportRows(importText.value);
  } catch (error) {
    ElMessage.warning(
      error instanceof Error ? error.message : '导入文件格式不正确',
    );
    return;
  }
  if (items.length > 500) {
    ElMessage.warning('一次最多导入 500 名学生');
    return;
  }
  importing.value = true;
  try {
    importResult.value = await importStudentsApi({ items });
    await loadAll();
    ElMessage.success(
      `导入完成：新增 ${importResult.value.created.length} 名学生`,
    );
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '学生导入失败');
  } finally {
    importing.value = false;
  }
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
          只维护一份学生档案；学校年级用于来源追溯，托管班级用于接送、作业和日常管理。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton plain @click="openImport">批量导入</ElButton>
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
        title="家长申请审核通过后会自动生成学生档案；日常新增时也可直接填写学生，学校、年级班级和托管班级会自动归档。"
        type="info"
      />
      <div class="sprout-filter-panel master-data-filter-panel mb-4">
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
          placeholder="按学校年级/班级筛选"
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
          placeholder="按托管班级筛选"
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
        <div class="master-data-filter-actions">
          <ElButton type="primary" @click="loadStudents">查询</ElButton>
          <ElButton plain @click="resetFilters">重置</ElButton>
        </div>
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
          <ElTableColumn label="托管班级" min-width="120">
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
              description="还没有学生档案，先新增一名学生或通过家长申请自动生成"
              :image-size="80"
            >
              <ElButton
                class="master-data-empty-action"
                type="primary"
                @click="openCreate"
              >
                新增学生
              </ElButton>
            </ElEmpty>
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

    <ElDialog
      v-model="importDialogVisible"
      destroy-on-close
      title="批量导入学生"
      width="min(760px, 94vw)"
    >
      <ElAlert
        class="mb-4"
        :closable="false"
        show-icon
        title="请使用 UTF-8 CSV 文件；学校、年级、班级和学生姓名为必填项，重复学生会跳过并返回行号。"
        type="info"
      />
      <div class="import-actions mb-3">
        <input
          accept=".csv,text/csv"
          class="import-file-input"
          type="file"
          @change="handleImportFile"
        />
        <ElButton plain @click="downloadImportTemplate">下载导入模板</ElButton>
      </div>
      <ElInput
        v-model="importText"
        :autosize="{ minRows: 8, maxRows: 16 }"
        placeholder="也可以直接粘贴 CSV 内容，例如：学生姓名,学校,年级,班级..."
        type="textarea"
      />
      <div v-if="importResult" class="import-result mt-4">
        <ElAlert
          :closable="false"
          :title="`新增 ${importResult.created.length} 人 · 跳过重复 ${importResult.skipped_duplicates.length} 行 · 无效 ${importResult.invalid.length} 行`"
          type="success"
        />
        <div
          v-if="importResult.skipped_duplicates.length"
          class="import-issue-list"
        >
          <div class="import-issue-title">重复行</div>
          <div
            v-for="item in importResult.skipped_duplicates"
            :key="`duplicate-${item.row}`"
            class="import-issue-item"
          >
            第 {{ item.row }} 行：{{ item.name || '未命名学生' }}（已跳过）
          </div>
        </div>
        <div v-if="importResult.invalid.length" class="import-issue-list">
          <div class="import-issue-title">无效行</div>
          <div
            v-for="item in importResult.invalid"
            :key="`invalid-${item.row}-${item.field}`"
            class="import-issue-item"
          >
            第 {{ item.row }} 行：{{ item.name || '未命名学生' }} ·
            {{ item.field || '关联数据' }} · {{ item.reason }}
          </div>
        </div>
      </div>
      <template #footer>
        <ElButton @click="importDialogVisible = false">关闭</ElButton>
        <ElButton :loading="importing" type="primary" @click="submitImport"
          >开始导入</ElButton
        >
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
.master-data-filter-panel {
  align-items: stretch;
}

.master-data-filter-actions {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-left: auto;
}

.master-data-filter-actions :deep(.el-button) {
  min-width: 72px;
}

.master-data-empty-action {
  margin-top: 8px;
}

.import-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.import-file-input {
  max-width: 280px;
}

.import-issue-list {
  padding: 10px 12px;
  margin-top: 12px;
  background: var(--el-fill-color-light);
  border-radius: 10px;
}

.import-issue-title {
  margin-bottom: 6px;
  font-weight: 600;
}

.import-issue-item {
  font-size: 13px;
  line-height: 1.8;
  color: var(--el-text-color-secondary);
}

@media (max-width: 768px) {
  .master-data-filter-actions {
    width: 100%;
    margin-left: 0;
  }

  .master-data-filter-actions :deep(.el-button) {
    flex: 1;
  }
}
</style>
