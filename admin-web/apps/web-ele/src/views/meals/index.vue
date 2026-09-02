<script lang="ts" setup>
import type { StudentRecord } from '#/api/master-data';
import type {
  DietNoteChangeRequestRecord,
  DietNoteRecord,
  MealPlanRecord,
} from '#/api/meals';

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
  ElMessageBox,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import { getStudentsApi } from '#/api/master-data';
import {
  copyMealPlanApi,
  getDietNoteChangeRequestsApi,
  getDietNotesApi,
  getMealPlansApi,
  reviewDietNoteChangeRequestApi,
  updateDietNoteApi,
  uploadMealPhotoApi,
  upsertMealPlanApi,
} from '#/api/meals';
import { businessAssetURL } from '#/api/request';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'Meals' });

const today = businessToday();
const selectedDate = ref(today);
const sourceDate = ref(today);
const loading = ref(false);
const submitting = ref(false);
const photoUploading = ref(false);
const loadError = ref('');
const dialogVisible = ref(false);
const photoInput = ref<HTMLInputElement>();
const plans = ref<MealPlanRecord[]>([]);
const students = ref<StudentRecord[]>([]);
const dietNotes = ref<DietNoteRecord[]>([]);
const dietNoteRequests = ref<DietNoteChangeRequestRecord[]>([]);
const dietDrafts = reactive<Record<number, string>>({});
const mealForm = reactive({
  adjustment_note: '',
  meal_date: today,
  menu_text: '',
  photo_url: '',
});

const currentPlan = computed(() =>
  plans.value.find((item) => item.meal_date === selectedDate.value),
);
const historyPlans = computed(() =>
  plans.value.toSorted((left, right) =>
    right.meal_date.localeCompare(left.meal_date),
  ),
);
const activeStudents = computed(() =>
  students.value.filter((item) => item.status === 'active'),
);

function offsetDate(date: string, offset: number) {
  const value = new Date(`${date}T00:00:00`);
  value.setDate(value.getDate() + offset);
  const month = `${value.getMonth() + 1}`.padStart(2, '0');
  const day = `${value.getDate()}`.padStart(2, '0');
  return `${value.getFullYear()}-${month}-${day}`;
}

function studentName(id: number) {
  return students.value.find((item) => item.id === id)?.name || `学生 ${id}`;
}

function openPlanDialog(plan?: MealPlanRecord) {
  Object.assign(mealForm, {
    adjustment_note: plan?.adjustment_note || '',
    meal_date: plan?.meal_date || selectedDate.value,
    menu_text: plan?.menu_text || '',
    photo_url: plan?.photo_url || '',
  });
  dialogVisible.value = true;
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const [planResult, studentResult, noteResult, requestResult] =
      await Promise.all([
        getMealPlansApi({
          from: offsetDate(selectedDate.value, -6),
          to: selectedDate.value,
        }),
        getStudentsApi({ status: 'active' }),
        getDietNotesApi(),
        getDietNoteChangeRequestsApi({ status: 'pending' }),
      ]);
    plans.value = planResult.items;
    students.value = studentResult.items;
    dietNotes.value = noteResult.items;
    dietNoteRequests.value = requestResult.items;
    for (const student of students.value) {
      dietDrafts[student.id] =
        noteResult.items.find((item) => item.student_id === student.id)?.note ||
        '';
    }
  } catch {
    plans.value = [];
    dietNoteRequests.value = [];
    loadError.value = '餐食数据加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

function studentNameForRequest(request: DietNoteChangeRequestRecord) {
  return studentName(request.student_id);
}

async function reviewDietNoteRequest(
  request: DietNoteChangeRequestRecord,
  status: 'approved' | 'rejected',
) {
  let promptResult;
  try {
    promptResult = await ElMessageBox.prompt(
      status === 'approved'
        ? '可补充一条照护提醒（可选）'
        : '可填写未通过原因，方便家长修改（可选）',
      status === 'approved' ? '确认饮食备注' : '驳回饮食备注申请',
      {
        inputPlaceholder: '备注内容（可选）',
        inputValue: '',
        confirmButtonText: '确认',
        cancelButtonText: '取消',
      },
    );
  } catch {
    return;
  }
  const reviewNote = promptResult.value.trim();
  submitting.value = true;
  try {
    await reviewDietNoteChangeRequestApi(request.id, {
      status,
      review_note: reviewNote,
    });
    ElMessage.success(
      status === 'approved' ? '饮食备注已确认生效' : '申请已驳回',
    );
    await loadData();
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '饮食备注审核失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function savePlan() {
  if (!mealForm.menu_text.trim()) {
    ElMessage.warning('请填写菜单内容');
    return;
  }
  submitting.value = true;
  try {
    await upsertMealPlanApi({ ...mealForm });
    ElMessage.success('餐食安排已保存');
    dialogVisible.value = false;
    await loadData();
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '餐食安排保存失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function copyPlan() {
  if (sourceDate.value === selectedDate.value) {
    ElMessage.warning('请选择不同的历史日期');
    return;
  }
  try {
    await ElMessageBox.confirm(
      `将 ${sourceDate.value} 的菜单复制到 ${selectedDate.value}？`,
      '复制历史菜单',
      { type: 'info' },
    );
  } catch {
    return;
  }
  submitting.value = true;
  try {
    await copyMealPlanApi({
      source_date: sourceDate.value,
      target_date: selectedDate.value,
    });
    ElMessage.success('历史菜单已复制');
    await loadData();
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '历史菜单复制失败',
    );
  } finally {
    submitting.value = false;
  }
}

function choosePhoto() {
  photoInput.value?.click();
}

async function handlePhoto(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file) return;
  photoUploading.value = true;
  try {
    const asset = await uploadMealPhotoApi(file, {
      meal_plan_id: currentPlan.value?.id,
    });
    mealForm.photo_url = asset.url;
    ElMessage.success('餐食照片已上传');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '餐食照片上传失败',
    );
  } finally {
    photoUploading.value = false;
  }
}

async function saveDietNote(student: StudentRecord) {
  submitting.value = true;
  try {
    await updateDietNoteApi(student.id, { note: dietDrafts[student.id] || '' });
    ElMessage.success(`${student.name} 的饮食备注已保存`);
    const result = await getDietNotesApi();
    dietNotes.value = result.items;
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '饮食备注保存失败',
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
        <p class="sprout-page-kicker">安心照护 · 今日餐食</p>
        <h1 class="sprout-page-title">餐食管理</h1>
        <p class="sprout-page-description">
          提前公布菜单，实际用餐后补充照片；学生饮食禁忌会在教师端和家长端保持可追溯。
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
        <ElButton
          v-access:code="'meal:write'"
          type="primary"
          @click="openPlanDialog(currentPlan)"
        >
          {{ currentPlan ? '编辑菜单' : '录入菜单' }}
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

    <div class="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_minmax(420px,0.8fr)]">
      <ElCard class="sprout-table-card" shadow="never">
        <template #header>
          <div class="sprout-table-toolbar">
            <div>
              <h2 class="sprout-section-title">{{ selectedDate }} 餐食安排</h2>
              <p class="sprout-section-caption">
                菜单可先发布，临时变化写在调整说明中。
              </p>
            </div>
            <ElTag :type="currentPlan ? 'success' : 'warning'">
              {{ currentPlan ? '已登记' : '待登记' }}
            </ElTag>
          </div>
        </template>
        <div v-if="currentPlan" class="space-y-4">
          <div
            class="whitespace-pre-line rounded-xl bg-green-50 p-4 text-sm leading-7"
          >
            {{ currentPlan.menu_text }}
          </div>
          <div
            v-if="currentPlan.adjustment_note"
            class="rounded-xl bg-amber-50 p-4 text-sm leading-7 text-amber-800"
          >
            <span class="font-semibold">调整说明：</span
            >{{ currentPlan.adjustment_note }}
          </div>
          <div v-if="currentPlan.photo_url" class="flex items-center gap-3">
            <img
              class="h-20 w-20 rounded-xl object-cover"
              :src="businessAssetURL(currentPlan.photo_url)"
              alt="实际餐食照片"
            />
            <span class="text-xs text-gray-500">已上传实际餐食照片</span>
          </div>
          <div
            class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500"
          >
            <span>记录人：{{ currentPlan.created_by_name || '未注明' }}</span>
            <span>最后更新：{{ currentPlan.updated_at }}</span>
          </div>
        </div>
        <ElEmpty v-else description="当天还没有餐食安排" :image-size="96" />
      </ElCard>

      <ElCard class="sprout-filter-card" shadow="never">
        <template #header>复制历史菜单</template>
        <p class="mb-4 text-sm leading-6 text-gray-500">
          复制只带入菜单和调整说明，不会带入历史餐食照片，避免把旧照片误当成当天实拍。
        </p>
        <div class="flex flex-wrap items-center gap-3">
          <ElDatePicker
            v-model="sourceDate"
            type="date"
            value-format="YYYY-MM-DD"
          />
          <ElButton
            v-access:code="'meal:write'"
            :loading="submitting"
            @click="copyPlan"
            >复制到 {{ selectedDate }}</ElButton
          >
        </div>
      </ElCard>
    </div>

    <ElCard class="sprout-table-card mt-5" shadow="never">
      <template #header>
        <div class="sprout-table-toolbar">
          <div>
            <h2 class="sprout-section-title">近 7 天餐食历史</h2>
            <p class="sprout-section-caption">
              菜单、临时调整和实际餐食照片按日期留存，方便复盘和家长查询。
            </p>
          </div>
          <span class="sprout-role-badge">{{ historyPlans.length }} 天</span>
        </div>
      </template>
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="historyPlans" row-key="id" stripe>
          <ElTableColumn label="日期" width="130" prop="meal_date" />
          <ElTableColumn label="菜单" min-width="260" prop="menu_text" />
          <ElTableColumn label="调整说明" min-width="220">
            <template #default="{ row }">
              {{ (row as MealPlanRecord).adjustment_note || '—' }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="实拍" width="100">
            <template #default="{ row }">
              <ElTag
                :type="(row as MealPlanRecord).photo_url ? 'success' : 'info'"
              >
                {{ (row as MealPlanRecord).photo_url ? '已上传' : '未上传' }}
              </ElTag>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="近 7 天还没有餐食历史" :image-size="80" />
          </template>
        </ElTable>
      </div>
    </ElCard>

    <ElCard class="sprout-table-card mt-5" shadow="never">
      <template #header>
        <div class="sprout-table-toolbar">
          <div>
            <h2 class="sprout-section-title">待确认的饮食变更</h2>
            <p class="sprout-section-caption">
              家长提交的过敏和特殊饮食变更，确认后才会更新学生正式照护备注。
            </p>
          </div>
          <ElTag :type="dietNoteRequests.length ? 'warning' : 'success'">
            {{ dietNoteRequests.length }} 条待处理
          </ElTag>
        </div>
      </template>
      <div class="sprout-table-wrap">
        <ElTable
          v-loading="loading"
          :data="dietNoteRequests"
          row-key="id"
          stripe
        >
          <ElTableColumn label="学生" width="140">
            <template #default="{ row }">
              {{ studentNameForRequest(row as DietNoteChangeRequestRecord) }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="当前备注" min-width="220">
            <template #default="{ row }">
              {{ (row as DietNoteChangeRequestRecord).current_note || '暂无' }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="家长申请修改为" min-width="240">
            <template #default="{ row }">
              {{
                (row as DietNoteChangeRequestRecord).requested_note ||
                '清除备注'
              }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="提交时间" width="180" prop="created_at" />
          <ElTableColumn label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <div class="flex items-center gap-2">
                <ElButton
                  v-access:code="'meal:write'"
                  link
                  type="primary"
                  @click="
                    reviewDietNoteRequest(
                      row as DietNoteChangeRequestRecord,
                      'approved',
                    )
                  "
                  >确认生效</ElButton
                >
                <ElButton
                  v-access:code="'meal:write'"
                  link
                  type="danger"
                  @click="
                    reviewDietNoteRequest(
                      row as DietNoteChangeRequestRecord,
                      'rejected',
                    )
                  "
                  >驳回</ElButton
                >
              </div>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="暂无待确认的饮食变更" :image-size="80" />
          </template>
        </ElTable>
      </div>
    </ElCard>

    <ElCard class="sprout-table-card mt-5" shadow="never">
      <template #header>
        <div class="sprout-table-toolbar">
          <div>
            <h2 class="sprout-section-title">学生饮食备注</h2>
            <p class="sprout-section-caption">
              只记录必要的过敏和特殊饮食信息，不替代医疗诊断。
            </p>
          </div>
          <span class="sprout-role-badge">{{ dietNotes.length }} 条已记录</span>
        </div>
      </template>
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="activeStudents" row-key="id" stripe>
          <ElTableColumn label="学生" width="150">
            <template #default="{ row }">{{ studentName(row.id) }}</template>
          </ElTableColumn>
          <ElTableColumn label="饮食备注" min-width="320">
            <template #default="{ row }">
              <ElInput
                v-model="dietDrafts[row.id]"
                placeholder="例如：花生过敏、忌牛奶"
              />
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <ElButton
                v-access:code="'meal:write'"
                :loading="submitting"
                link
                type="primary"
                @click="saveDietNote(row as StudentRecord)"
                >保存</ElButton
              >
            </template>
          </ElTableColumn>
          <template #empty
            ><ElEmpty description="暂无学生档案" :image-size="80"
          /></template>
        </ElTable>
      </div>
    </ElCard>

    <ElDialog v-model="dialogVisible" title="餐食安排" width="min(620px, 94vw)">
      <ElForm label-position="top" :model="mealForm">
        <ElFormItem label="日期" required>
          <ElDatePicker
            v-model="mealForm.meal_date"
            class="w-full"
            type="date"
            value-format="YYYY-MM-DD"
          />
        </ElFormItem>
        <ElFormItem label="菜单内容" required>
          <ElInput
            v-model="mealForm.menu_text"
            :rows="5"
            type="textarea"
            placeholder="例如：香菇鸡肉饭、番茄蛋花汤、时令水果"
          />
        </ElFormItem>
        <ElFormItem label="临时调整说明">
          <ElInput
            v-model="mealForm.adjustment_note"
            :rows="3"
            type="textarea"
            placeholder="如无变化可留空"
          />
        </ElFormItem>
        <ElFormItem label="实际餐食照片">
          <input
            ref="photoInput"
            accept="image/jpeg,image/png,image/webp"
            class="hidden"
            type="file"
            @change="handlePhoto"
          />
          <div class="flex flex-wrap items-center gap-3">
            <ElButton :loading="photoUploading" @click="choosePhoto"
              >上传照片</ElButton
            >
            <img
              v-if="mealForm.photo_url"
              class="h-16 w-16 rounded-xl object-cover"
              :src="businessAssetURL(mealForm.photo_url)"
              alt="餐食照片预览"
            />
            <span class="text-xs text-gray-500"
              >建议在实际用餐前后拍摄，单张不超过 5MB。</span
            >
          </div>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton
          v-access:code="'meal:write'"
          :loading="submitting"
          type="primary"
          @click="savePlan"
          >保存安排</ElButton
        >
      </template>
    </ElDialog>
  </div>
</template>
