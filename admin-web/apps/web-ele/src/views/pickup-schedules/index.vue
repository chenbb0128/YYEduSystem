<script lang="ts" setup>
import type {
  CareClassRecord,
  SchoolClassRecord,
  SchoolRecord,
} from '#/api/master-data';
import type {
  PickupScheduleMode,
  PickupSchedulePayload,
  PickupScheduleRecord,
} from '#/api/pickup-schedules';
import type { SystemUserRecord } from '#/api/system/user';

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
  ElSwitch,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import {
  getCareClassesApi,
  getSchoolClassesApi,
  getSchoolsApi,
} from '#/api/master-data';
import {
  createPickupScheduleApi,
  generatePickupSchedulesApi,
  getPickupSchedulesApi,
  updatePickupScheduleApi,
} from '#/api/pickup-schedules';
import { getSystemUsersApi } from '#/api/system/user';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'PickupSchedules' });

const today = businessToday();
const loading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const dialogVisible = ref(false);
const editingID = ref(0);
const selectedDate = ref(today);
const schedules = ref<PickupScheduleRecord[]>([]);
const schools = ref<SchoolRecord[]>([]);
const schoolClasses = ref<SchoolClassRecord[]>([]);
const careClasses = ref<CareClassRecord[]>([]);
const teachers = ref<SystemUserRecord[]>([]);

const form = reactive<PickupSchedulePayload>({
  school_id: 0,
  school_class_id: 0,
  weekday: 1,
  pickup_mode: 'school_pickup',
  expected_pickup_time: '16:30',
  effective_from: today,
  effective_to: '',
  enabled: true,
  notes: '',
});

const activeSchools = computed(() =>
  schools.value.filter((item) => item.status === 'active'),
);
const activeClasses = computed(() =>
  schoolClasses.value.filter(
    (item) => item.status === 'active' && item.school_id === form.school_id,
  ),
);
const activeTeachers = computed(() =>
  teachers.value.filter(
    (item) => item.role === 'teacher' && item.status === 'active',
  ),
);
const activeCareClasses = computed(() =>
  careClasses.value.filter((item) => item.status === 'active'),
);

function schoolName(id: number) {
  return schools.value.find((item) => item.id === id)?.name || `学校 ${id}`;
}

function classLabel(item: SchoolClassRecord) {
  return `${item.grade}${item.name}`;
}

function modeLabel(mode: PickupScheduleMode) {
  return mode === 'school_pickup' ? '学校接送' : '自行到班';
}

function loadFormDefaults() {
  const firstSchool = activeSchools.value[0];
  const firstClass = schoolClasses.value.find(
    (item) => item.status === 'active' && item.school_id === firstSchool?.id,
  );
  Object.assign(form, {
    school_id: firstSchool?.id || 0,
    school_class_id: firstClass?.id || 0,
    care_class_id: undefined,
    weekday: 1,
    pickup_mode: 'school_pickup',
    teacher_user_id: activeTeachers.value[0]?.id,
    teacher_name: '',
    expected_pickup_time: '16:30',
    effective_from: today,
    effective_to: '',
    enabled: true,
    notes: '',
  });
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const [scheduleResult, schoolResult, classResult, careResult, userResult] =
      await Promise.all([
        getPickupSchedulesApi(),
        getSchoolsApi(),
        getSchoolClassesApi(),
        getCareClassesApi(),
        getSystemUsersApi({ page: 1, pageSize: 100 }),
      ]);
    schedules.value = scheduleResult.items;
    schools.value = schoolResult.items;
    schoolClasses.value = classResult.items;
    careClasses.value = careResult.items;
    teachers.value = userResult.items;
  } catch {
    schedules.value = [];
    loadError.value = '接送排班加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingID.value = 0;
  loadFormDefaults();
  dialogVisible.value = true;
}

function openEdit(row: unknown) {
  const schedule = row as PickupScheduleRecord;
  editingID.value = schedule.id;
  Object.assign(form, {
    school_id: schedule.school_id,
    school_class_id: schedule.school_class_id,
    care_class_id: schedule.care_class_id,
    weekday: schedule.weekday,
    pickup_mode: schedule.pickup_mode,
    teacher_user_id: schedule.teacher_user_id,
    teacher_name: schedule.teacher_name,
    expected_pickup_time: schedule.expected_pickup_time,
    effective_from: schedule.effective_from,
    effective_to: schedule.effective_to || '',
    enabled: schedule.enabled,
    notes: schedule.notes,
  });
  dialogVisible.value = true;
}

function syncClass() {
  if (!activeClasses.value.some((item) => item.id === form.school_class_id)) {
    form.school_class_id = activeClasses.value[0]?.id || 0;
  }
}

async function save() {
  if (!form.school_id || !form.school_class_id || !form.effective_from) {
    ElMessage.warning('请先选择学校班级和生效日期');
    return;
  }
  if (!form.weekday || form.weekday < 1 || form.weekday > 7) {
    ElMessage.warning('请选择星期');
    return;
  }
  submitting.value = true;
  try {
    const payload: PickupSchedulePayload = { ...form };
    if (!payload.care_class_id) delete payload.care_class_id;
    if (!payload.teacher_user_id) delete payload.teacher_user_id;
    if (!payload.effective_to) delete payload.effective_to;
    await (editingID.value
      ? updatePickupScheduleApi(editingID.value, payload)
      : createPickupScheduleApi(payload));
    ElMessage.success(editingID.value ? '排班已更新' : '排班已创建');
    dialogVisible.value = false;
    await loadData();
  } finally {
    submitting.value = false;
  }
}

async function generateToday() {
  submitting.value = true;
  try {
    const result = await generatePickupSchedulesApi(selectedDate.value);
    const created = result.created_operation_ids.length;
    const skipped = result.skipped_schedule_ids.length;
    ElMessage.success(
      `已处理 ${created} 条任务${skipped ? `，跳过 ${skipped} 条` : ''}`,
    );
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
        <p class="sprout-page-kicker">接送管理 · 周期规则</p>
        <h1 class="sprout-page-title">接送排班</h1>
        <p class="sprout-page-description">
          设置每周接送规则。系统会自动生成当天待确认任务，教师核对名单后再通知家长。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElDatePicker
          v-model="selectedDate"
          type="date"
          value-format="YYYY-MM-DD"
          placeholder="生成日期"
        />
        <ElButton :loading="submitting" @click="generateToday">
          生成当天任务
        </ElButton>
        <ElButton :loading="loading" @click="loadData">刷新</ElButton>
        <ElButton type="primary" @click="openCreate">新增排班</ElButton>
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
          <h2 class="sprout-section-title">周期排班规则</h2>
          <p class="sprout-section-caption">
            关闭规则不会删除历史任务；修改规则只影响后续自动生成的任务。
          </p>
        </div>
        <span class="sprout-role-badge">{{ schedules.length }} 条规则</span>
      </div>
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="schedules" row-key="id" stripe>
          <ElTableColumn label="星期" width="90">
            <template #default="{ row }">{{ row.weekday_label }}</template>
          </ElTableColumn>
          <ElTableColumn label="学校班级" min-width="220">
            <template #default="{ row }">
              {{ row.school_name || schoolName(row.school_id) }} · {{ row.grade
              }}{{ row.class_name }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="方式 / 出发时间" min-width="170">
            <template #default="{ row }">
              {{ modeLabel(row.pickup_mode) }}
              <span v-if="row.expected_pickup_time" class="text-gray-400">
                · {{ row.expected_pickup_time }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="教师" min-width="120">
            <template #default="{ row }">{{
              row.teacher_name || '待安排'
            }}</template>
          </ElTableColumn>
          <ElTableColumn label="有效期" min-width="180">
            <template #default="{ row }">
              {{ row.effective_from }} ~ {{ row.effective_to || '长期' }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="90">
            <template #default="{ row }">
              <ElTag :type="row.enabled ? 'success' : 'info'">
                {{ row.enabled ? '启用' : '停用' }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <ElButton link type="primary" @click="openEdit(row)"
                >编辑</ElButton
              >
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="还没有周期排班" :image-size="90" />
          </template>
        </ElTable>
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      destroy-on-close
      :title="editingID ? '编辑接送排班' : '新增接送排班'"
      width="min(680px, 94vw)"
    >
      <ElForm label-position="top" :model="form">
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="学校" required>
            <ElSelect
              v-model="form.school_id"
              class="w-full"
              @change="syncClass"
            >
              <ElOption
                v-for="item in activeSchools"
                :key="item.id"
                :label="item.name"
                :value="item.id"
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
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="星期" required>
            <ElSelect v-model="form.weekday" class="w-full">
              <ElOption label="周一" :value="1" />
              <ElOption label="周二" :value="2" />
              <ElOption label="周三" :value="3" />
              <ElOption label="周四" :value="4" />
              <ElOption label="周五" :value="5" />
              <ElOption label="周六" :value="6" />
              <ElOption label="周日" :value="7" />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="接送方式" required>
            <ElSelect v-model="form.pickup_mode" class="w-full">
              <ElOption label="学校接送" value="school_pickup" />
              <ElOption label="自行到班" value="self_arrival" />
            </ElSelect>
          </ElFormItem>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="默认教师">
            <ElSelect v-model="form.teacher_user_id" class="w-full" clearable>
              <ElOption
                v-for="teacher in activeTeachers"
                :key="teacher.id"
                :label="`${teacher.realName || teacher.username}（${teacher.username}）`"
                :value="teacher.id"
              />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="预计出发时间">
            <ElInput
              v-model="form.expected_pickup_time"
              placeholder="例如 16:30"
            />
          </ElFormItem>
        </div>
        <ElFormItem label="托管班（可选）">
          <ElSelect v-model="form.care_class_id" class="w-full" clearable>
            <ElOption
              v-for="item in activeCareClasses"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </ElSelect>
        </ElFormItem>
        <div class="grid grid-cols-2 gap-3">
          <ElFormItem label="生效日期" required>
            <ElDatePicker
              v-model="form.effective_from"
              type="date"
              value-format="YYYY-MM-DD"
              class="w-full"
            />
          </ElFormItem>
          <ElFormItem label="结束日期">
            <ElDatePicker
              v-model="form.effective_to"
              type="date"
              value-format="YYYY-MM-DD"
              clearable
              class="w-full"
            />
          </ElFormItem>
        </div>
        <ElFormItem label="规则状态">
          <ElSwitch
            v-model="form.enabled"
            active-text="启用"
            inactive-text="停用"
          />
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput
            v-model="form.notes"
            type="textarea"
            :rows="2"
            maxlength="500"
            show-word-limit
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton :loading="submitting" type="primary" @click="save"
          >保存排班</ElButton
        >
      </template>
    </ElDialog>
  </div>
</template>
