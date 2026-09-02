<script lang="ts" setup>
import type {
  DailySummaryRecord,
  DailySummaryStatus,
  DailySummaryVersionRecord,
} from '#/api/daily-summaries';
import type { StudentRecord } from '#/api/master-data';

import { computed, onMounted, ref } from 'vue';

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
  ElTag,
} from 'element-plus';

import {
  closeDailySummaryApi,
  correctDailySummaryApi,
  generateDailySummaryApi,
  getDailySummariesApi,
  getDailySummaryVersionsApi,
  publishDailySummaryApi,
  updateDailySummaryApi,
  withdrawDailySummaryApi,
} from '#/api/daily-summaries';
import { getStudentsApi } from '#/api/master-data';
import { businessToday } from '#/utils/business-date';

defineOptions({ name: 'DailySummaries' });

const today = businessToday();
const selectedDate = ref(today);
const loading = ref(false);
const submitting = ref(false);
const loadError = ref('');
const summary = ref<DailySummaryRecord | null>(null);
const students = ref<StudentRecord[]>([]);
const content = ref('');
const childUpdates = ref<Record<string, string>>({});
const versions = ref<DailySummaryVersionRecord[]>([]);
const correctionVisible = ref(false);
const correctionContent = ref('');
const correctionReason = ref('');
const correctionChildUpdates = ref<Record<string, string>>({});

const updateStudents = computed(() => {
  const ids = Object.keys(childUpdates.value);
  const selected = students.value.filter((item) =>
    ids.includes(String(item.id)),
  );
  return selected.length > 0 ? selected : students.value.slice(0, 0);
});

const correctionStudents = computed(() => {
  const ids = Object.keys(correctionChildUpdates.value);
  return students.value.filter((item) => ids.includes(String(item.id)));
});

function statusLabel(status: DailySummaryStatus) {
  return {
    closed: '当天已结束',
    draft: '草稿',
    published: '已发布',
    withdrawn: '已撤回',
  }[status];
}

function statusType(status: DailySummaryStatus) {
  if (status === 'published') return 'success';
  if (status === 'closed' || status === 'withdrawn') return 'info';
  return 'warning';
}

function studentName(id: string) {
  return (
    students.value.find((item) => String(item.id) === id)?.name || `学生 ${id}`
  );
}

function syncEditor(item: DailySummaryRecord | null) {
  summary.value = item;
  content.value = item?.content || '';
  childUpdates.value = item?.child_updates ?? {};
}

async function loadVersions(item: DailySummaryRecord | null) {
  if (!item) {
    versions.value = [];
    return;
  }
  try {
    const result = await getDailySummaryVersionsApi(item.id);
    versions.value = result.items;
  } catch {
    versions.value = [];
  }
}

async function loadData() {
  loading.value = true;
  loadError.value = '';
  try {
    const [summaryResult, studentResult] = await Promise.all([
      getDailySummariesApi({ date: selectedDate.value }),
      getStudentsApi({ status: 'active' }),
    ]);
    students.value = studentResult.items;
    syncEditor(summaryResult.items[0] || null);
    await loadVersions(summaryResult.items[0] || null);
  } catch {
    syncEditor(null);
    loadError.value = '每日总结加载失败，请稍后重试。';
  } finally {
    loading.value = false;
  }
}

async function generate() {
  if ((summary.value && summary.value.status !== 'draft') || submitting.value)
    return;
  submitting.value = true;
  try {
    const result = await generateDailySummaryApi(selectedDate.value);
    syncEditor(result);
    ElMessage.success('已根据当天数据生成总结草稿');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '每日总结生成失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function save(): Promise<boolean> {
  if (!summary.value || summary.value.status !== 'draft' || submitting.value) {
    return false;
  }
  if (!content.value.trim()) {
    ElMessage.warning('请填写总结内容');
    return false;
  }
  submitting.value = true;
  try {
    const result = await updateDailySummaryApi(summary.value.id, {
      content: content.value,
      child_updates: { ...childUpdates.value },
    });
    syncEditor(result);
    await loadVersions(result);
    ElMessage.success('每日总结已保存');
    return true;
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '每日总结保存失败',
    );
    return false;
  } finally {
    submitting.value = false;
  }
}

async function publish() {
  if (!summary.value || summary.value.status !== 'draft' || submitting.value)
    return;
  try {
    await ElMessageBox.confirm(
      '发布后家长可以查看当天总结，确认发布？',
      '发布每日总结',
      { type: 'info' },
    );
  } catch {
    return;
  }
  const saved = await save();
  if (!saved || !summary.value) return;
  submitting.value = true;
  try {
    const result = await publishDailySummaryApi(summary.value.id);
    syncEditor(result);
    ElMessage.success('每日总结已发布');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '每日总结发布失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function closeDay() {
  if (!summary.value) return;
  try {
    await ElMessageBox.confirm(
      '结束当天托管后将不能继续编辑，确认结束？',
      '结束当天托管',
      { type: 'warning' },
    );
  } catch {
    return;
  }
  if (summary.value.status !== 'published' || submitting.value) return;
  submitting.value = true;
  try {
    const result = await closeDailySummaryApi(summary.value.id);
    syncEditor(result);
    await loadVersions(result);
    ElMessage.success('当天托管已结束');
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '当天托管结束失败',
    );
  } finally {
    submitting.value = false;
  }
}

async function withdraw() {
  if (
    !summary.value ||
    summary.value.status !== 'published' ||
    submitting.value
  ) {
    return;
  }
  try {
    const result = await ElMessageBox.prompt(
      '撤回后家长将暂时看不到这条总结，后续可通过更正重新发布。',
      '撤回每日总结',
      {
        inputPlaceholder: '请填写撤回原因',
        inputValidator: (value) => (value.trim() ? true : '请填写撤回原因'),
        type: 'warning',
      },
    );
    submitting.value = true;
    const updated = await withdrawDailySummaryApi(
      summary.value.id,
      result.value,
    );
    syncEditor(updated);
    await loadVersions(updated);
    ElMessage.success('每日总结已撤回');
  } catch (error) {
    if (error instanceof Error && error.message) ElMessage.error(error.message);
  } finally {
    submitting.value = false;
  }
}

function openCorrection() {
  if (
    !summary.value ||
    (summary.value.status !== 'published' &&
      summary.value.status !== 'closed' &&
      summary.value.status !== 'withdrawn')
  )
    return;
  correctionContent.value = content.value;
  correctionChildUpdates.value = { ...childUpdates.value };
  correctionReason.value = '';
  correctionVisible.value = true;
}

async function correct() {
  if (
    !summary.value ||
    !correctionContent.value.trim() ||
    !correctionReason.value.trim() ||
    submitting.value
  ) {
    ElMessage.warning('请填写更正内容和原因');
    return;
  }
  submitting.value = true;
  try {
    const updated = await correctDailySummaryApi(summary.value.id, {
      content: correctionContent.value,
      child_updates: { ...correctionChildUpdates.value },
      reason: correctionReason.value,
    });
    correctionVisible.value = false;
    syncEditor(updated);
    await loadVersions(updated);
    ElMessage.success('更正已发布，家长可查看最新版本');
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '更正发布失败');
  } finally {
    submitting.value = false;
  }
}

function versionActionLabel(value: string) {
  return (
    {
      closed: '结束当天',
      corrected: '更正发布',
      generated: '生成草稿',
      published: '发布',
      updated: '保存修改',
      withdrawn: '撤回',
    }[value] || value
  );
}

onMounted(loadData);
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">家校沟通 · 当天回顾</p>
        <h1 class="sprout-page-title">教师每日总结</h1>
        <p class="sprout-page-description">
          根据接送、作业和餐食记录生成草稿，教师确认后发布给家长，并在当天工作结束时完成收口。
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
          v-access:code="'summary:write'"
          :disabled="summary != null && summary.status !== 'draft'"
          :loading="submitting"
          type="primary"
          @click="generate"
        >
          {{ summary ? '重新生成草稿' : '生成今日草稿' }}
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

    <ElCard v-if="summary" class="sprout-table-card" shadow="never">
      <template #header>
        <div class="sprout-table-toolbar">
          <div>
            <h2 class="sprout-section-title">
              {{ summary.summary_date }} 工作总结
            </h2>
            <p class="sprout-section-caption">
              生成人：{{ summary.created_by_name || '未注明' }} · 当前版本 v{{
                summary.version
              }}
            </p>
          </div>
          <ElTag :type="statusType(summary.status)">{{
            statusLabel(summary.status)
          }}</ElTag>
        </div>
      </template>
      <ElForm label-position="top">
        <ElFormItem label="总体总结" required>
          <ElInput
            v-model="content"
            :disabled="summary.status !== 'draft'"
            :rows="8"
            type="textarea"
            placeholder="记录当天托管情况、需要家长关注的事项"
          />
        </ElFormItem>
        <ElFormItem
          v-if="updateStudents.length"
          label="孩子动态（只对对应家长展示）"
        >
          <div class="w-full space-y-3">
            <div
              v-for="student in updateStudents"
              :key="student.id"
              class="grid gap-2 md:grid-cols-[130px_1fr] md:items-center"
            >
              <span class="text-sm font-semibold text-gray-600">{{
                studentName(String(student.id))
              }}</span>
              <ElInput
                v-model="childUpdates[String(student.id)]"
                :disabled="summary.status !== 'draft'"
                placeholder="如：作业已完成、今日状态良好"
              />
            </div>
          </div>
        </ElFormItem>
      </ElForm>
      <div class="mt-2 flex flex-wrap justify-end gap-2">
        <ElButton
          v-if="summary.status === 'draft'"
          v-access:code="'summary:write'"
          :loading="submitting"
          @click="save"
          >保存修改</ElButton
        >
        <ElButton
          v-if="summary.status === 'draft'"
          v-access:code="'summary:write'"
          :loading="submitting"
          type="primary"
          @click="publish"
          >发布给家长</ElButton
        >
        <ElButton
          v-if="summary.status === 'published'"
          v-access:code="'summary:write'"
          :loading="submitting"
          type="warning"
          @click="closeDay"
          >结束当天托管</ElButton
        >
        <ElButton
          v-if="summary.status === 'published'"
          v-access:code="'summary:write'"
          :loading="submitting"
          type="warning"
          @click="withdraw"
          >撤回发布</ElButton
        >
        <ElButton
          v-if="
            summary.status === 'published' ||
            summary.status === 'closed' ||
            summary.status === 'withdrawn'
          "
          v-access:code="'summary:write'"
          :loading="submitting"
          @click="openCorrection"
          >发起更正</ElButton
        >
      </div>
      <div
        v-if="summary.withdrawal_reason || summary.correction_reason"
        class="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-800"
      >
        <span v-if="summary.withdrawal_reason">
          撤回原因：{{ summary.withdrawal_reason }}
        </span>
        <span v-else>最近更正原因：{{ summary.correction_reason }}</span>
      </div>
    </ElCard>
    <ElCard v-else class="sprout-table-card" shadow="never">
      <ElEmpty
        description="当天还没有每日总结，点击右上角生成草稿"
        :image-size="110"
      />
    </ElCard>
    <ElCard
      v-if="summary && versions.length"
      class="sprout-table-card mt-4"
      shadow="never"
    >
      <template #header>
        <div class="sprout-table-toolbar">
          <div>
            <h2 class="sprout-section-title">版本记录</h2>
            <p class="sprout-section-caption">历史内容只读留痕</p>
          </div>
        </div>
      </template>
      <div class="space-y-3">
        <div
          v-for="item in versions"
          :key="item.id"
          class="flex flex-wrap items-center gap-3 rounded-lg bg-slate-50 p-3 text-sm"
        >
          <ElTag size="small">v{{ item.version }}</ElTag>
          <span class="font-semibold text-slate-700">{{
            versionActionLabel(item.action)
          }}</span>
          <span class="text-slate-500"
            >{{ item.created_by_name || '系统' }} · {{ item.created_at }}</span
          >
          <span v-if="item.reason" class="text-amber-700"
            >原因：{{ item.reason }}</span
          >
        </div>
      </div>
    </ElCard>
    <ElDialog v-model="correctionVisible" title="更正并重新发布" width="640px">
      <ElForm label-position="top">
        <ElFormItem label="更正后的总体总结" required>
          <ElInput
            v-model="correctionContent"
            :rows="8"
            type="textarea"
            maxlength="4000"
            show-word-limit
          />
        </ElFormItem>
        <ElFormItem
          v-if="correctionStudents.length"
          label="孩子动态（只对对应家长展示）"
        >
          <div class="w-full space-y-3">
            <div
              v-for="student in correctionStudents"
              :key="student.id"
              class="grid gap-2 md:grid-cols-[130px_1fr] md:items-center"
            >
              <span class="text-sm font-semibold text-gray-600">{{
                studentName(String(student.id))
              }}</span>
              <ElInput
                v-model="correctionChildUpdates[String(student.id)]"
                placeholder="如：作业已完成、今日状态良好"
              />
            </div>
          </div>
        </ElFormItem>
        <ElFormItem label="更正原因" required>
          <ElInput
            v-model="correctionReason"
            type="textarea"
            :rows="3"
            maxlength="500"
            show-word-limit
            placeholder="例如：补充说明孩子离班时间"
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="correctionVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="correct"
          >确认更正并发布</ElButton
        >
      </template>
    </ElDialog>
  </div>
</template>
