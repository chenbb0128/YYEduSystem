<script lang="ts" setup>
import type { StudentRecord } from '#/api/master-data';
import type {
  ExtractedWrongQuestionRecord,
  WrongPaperRecord,
  WrongQuestionRecord,
  WrongQuestionStatus,
} from '#/api/wrongbook';

import { computed, onMounted, reactive, ref } from 'vue';

import {
  ElAlert,
  ElButton,
  ElCard,
  ElCheckbox,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElImage,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import { uploadHomeworkPhotoApi } from '#/api/homework';
import { getStudentsApi } from '#/api/master-data';
import { businessAssetURL } from '#/api/request';
import {
  createWrongPaperApi,
  createWrongQuestionsApi,
  extractWrongQuestionsApi,
  getWrongPaperApi,
  getWrongPapersApi,
  getWrongQuestionsApi,
  updateWrongQuestionApi,
} from '#/api/wrongbook';

defineOptions({ name: 'Wrongbook' });

interface CandidateQuestion extends ExtractedWrongQuestionRecord {
  selected: boolean;
}

const loading = ref(false);
const extracting = ref(false);
const saving = ref(false);
const uploadInput = ref<HTMLInputElement>();
const students = ref<StudentRecord[]>([]);
const questions = ref<WrongQuestionRecord[]>([]);
const papers = ref<WrongPaperRecord[]>([]);
const candidates = ref<CandidateQuestion[]>([]);
const extractionMocked = ref(false);
const sourceImageURL = ref('');
const sourceImagePreview = computed(() =>
  businessAssetURL(sourceImageURL.value),
);
const extractionText = ref('');
const selectedQuestionIDs = ref<number[]>([]);
const paperDialogVisible = ref(false);
const currentPaper = ref<null | WrongPaperRecord>(null);
const filters = reactive({
  keyword: '',
  status: 'active' as '' | WrongQuestionStatus,
  student_id: 0,
  subject: '',
});

const activeStudents = computed(() =>
  students.value.filter((item) => item.status === 'active'),
);

const selectedStudent = computed(() =>
  students.value.find((item) => item.id === filters.student_id),
);

const selectedQuestions = computed(() =>
  questions.value.filter((item) => selectedQuestionIDs.value.includes(item.id)),
);

function statusLabel(status: WrongQuestionStatus) {
  return { active: '待复习', archived: '已归档', mastered: '已掌握' }[status];
}

function statusType(status: WrongQuestionStatus) {
  if (status === 'active') return 'warning';
  if (status === 'mastered') return 'success';
  return 'info';
}

function sourceLabel(source: WrongPaperRecord['source']) {
  return { parent: '家长生成', system: '系统生成', teacher: '老师生成' }[
    source
  ];
}

function shortDate(value?: string) {
  return value ? value.slice(0, 10) : '';
}

async function loadReferences() {
  const result = await getStudentsApi();
  students.value = result.items;
  if (!filters.student_id) {
    filters.student_id = activeStudents.value[0]?.id || 0;
  }
}

async function loadWrongbook() {
  loading.value = true;
  try {
    const [questionResult, paperResult] = await Promise.all([
      getWrongQuestionsApi({
        keyword: filters.keyword || undefined,
        status: filters.status,
        student_id: filters.student_id || undefined,
        subject: filters.subject || undefined,
      }),
      getWrongPapersApi({ student_id: filters.student_id || undefined }),
    ]);
    questions.value = questionResult.items;
    papers.value = paperResult.items;
    selectedQuestionIDs.value = selectedQuestionIDs.value.filter((id) =>
      questions.value.some((item) => item.id === id),
    );
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '错题集加载失败');
  } finally {
    loading.value = false;
  }
}

function handleSelectionChange(rows: WrongQuestionRecord[]) {
  selectedQuestionIDs.value = rows.map((item) => item.id);
}

function resetFilters() {
  Object.assign(filters, { keyword: '', status: 'active' });
  void loadWrongbook();
}

function openUploadPicker() {
  if (!filters.student_id) {
    ElMessage.warning('请先选择学生');
    return;
  }
  uploadInput.value?.click();
}

async function handleUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file) return;
  extracting.value = true;
  try {
    const asset = await uploadHomeworkPhotoApi(file);
    sourceImageURL.value = asset.url;
    await runExtract();
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : '图片上传或提题失败',
    );
  } finally {
    extracting.value = false;
  }
}

async function runExtract() {
  if (!sourceImageURL.value && !extractionText.value.trim()) {
    ElMessage.warning('请上传图片，或粘贴题目文字');
    return;
  }
  extracting.value = true;
  try {
    const result = await extractWrongQuestionsApi({
      image_url: sourceImageURL.value,
      source_text: extractionText.value.trim(),
      subject: filters.subject || '综合',
    });
    extractionMocked.value = result.mocked;
    candidates.value = result.items.map((item) => ({
      ...item,
      selected: true,
    }));
    ElMessage.success(
      result.mocked
        ? '已生成待校对题目卡片'
        : `已提取 ${result.total} 道候选题`,
    );
  } finally {
    extracting.value = false;
  }
}

async function saveCandidates() {
  if (!filters.student_id) {
    ElMessage.warning('请先选择学生');
    return;
  }
  const selected = candidates.value.filter(
    (item) => item.selected && item.question_text.trim(),
  );
  if (selected.length === 0) {
    ElMessage.warning('请至少选择一道错题');
    return;
  }
  saving.value = true;
  try {
    const result = await createWrongQuestionsApi({
      items: selected.map((item) => ({
        answer_text: item.answer_text?.trim(),
        explanation: item.explanation?.trim(),
        knowledge_point: item.knowledge_point?.trim(),
        question_text: item.question_text.trim(),
        source_image_url: sourceImageURL.value,
        student_id: filters.student_id,
        subject: item.subject || filters.subject || '综合',
        teacher_note: extractionMocked.value
          ? 'OCR 未配置，已由老师校对保存。'
          : '',
      })),
    });
    candidates.value = [];
    extractionText.value = '';
    extractionMocked.value = false;
    ElMessage.success(`已保存 ${result.total} 道错题`);
    await loadWrongbook();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存错题失败');
  } finally {
    saving.value = false;
  }
}

async function updateStatus(
  question: WrongQuestionRecord,
  status: WrongQuestionStatus,
) {
  saving.value = true;
  try {
    await updateWrongQuestionApi(question.id, {
      answer_text: question.answer_text,
      explanation: question.explanation,
      knowledge_point: question.knowledge_point,
      question_text: question.question_text,
      status,
      subject: question.subject,
      teacher_note: question.teacher_note,
    });
    ElMessage.success(status === 'mastered' ? '已标记掌握' : '状态已更新');
    await loadWrongbook();
  } finally {
    saving.value = false;
  }
}

async function generatePaper() {
  if (!filters.student_id) {
    ElMessage.warning('请先选择学生');
    return;
  }
  if (selectedQuestionIDs.value.length === 0) {
    ElMessage.warning('请先勾选错题');
    return;
  }
  const result = await ElMessageBox.prompt('可修改复习卷标题', '生成复习卷', {
    confirmButtonText: '生成',
    inputValue: `${selectedStudent.value?.name || '学生'}错题复习卷`,
    cancelButtonText: '取消',
  }).catch(() => undefined);
  if (!result) return;
  saving.value = true;
  try {
    const paper = await createWrongPaperApi({
      question_ids: selectedQuestionIDs.value,
      student_id: filters.student_id,
      title: result.value.trim(),
    });
    currentPaper.value = paper;
    paperDialogVisible.value = true;
    selectedQuestionIDs.value = [];
    ElMessage.success('复习卷已生成');
    await loadWrongbook();
  } finally {
    saving.value = false;
  }
}

async function openPaper(row: WrongPaperRecord) {
  saving.value = true;
  try {
    currentPaper.value = await getWrongPaperApi(row.id);
    paperDialogVisible.value = true;
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  loading.value = true;
  try {
    await loadReferences();
    await loadWrongbook();
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="sprout-page wrongbook-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">AI 辅助整理 · 学生复习</p>
        <h1 class="sprout-page-title">学生错题集</h1>
        <p class="sprout-page-description">
          老师拍照提取题目，勾选错题后保存到学生错题集；家长和老师都可以基于错题生成复习卷。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton
          :loading="extracting"
          type="primary"
          @click="openUploadPicker"
        >
          拍照/上传提题
        </ElButton>
        <ElButton :loading="saving" type="warning" @click="generatePaper">
          生成复习卷
        </ElButton>
      </div>
    </div>

    <input
      ref="uploadInput"
      accept="image/jpeg,image/png,image/webp"
      class="hidden"
      type="file"
      @change="handleUpload"
    />

    <div class="wrongbook-layout">
      <ElCard class="sprout-table-card wrongbook-side-card" shadow="never">
        <template #header>错题筛选</template>
        <ElForm label-position="top">
          <ElFormItem label="学生">
            <ElSelect
              v-model="filters.student_id"
              class="w-full"
              filterable
              placeholder="选择学生"
              @change="loadWrongbook"
            >
              <ElOption
                v-for="student in activeStudents"
                :key="student.id"
                :label="student.name"
                :value="student.id"
              />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="学科">
            <ElInput
              v-model="filters.subject"
              clearable
              placeholder="数学 / 语文 / 英语"
              @keyup.enter="loadWrongbook"
            />
          </ElFormItem>
          <ElFormItem label="状态">
            <ElSelect
              v-model="filters.status"
              class="w-full"
              @change="loadWrongbook"
            >
              <ElOption label="待复习" value="active" />
              <ElOption label="已掌握" value="mastered" />
              <ElOption label="已归档" value="archived" />
              <ElOption label="全部" value="" />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="关键词">
            <ElInput
              v-model="filters.keyword"
              clearable
              placeholder="题干、知识点、备注"
              @keyup.enter="loadWrongbook"
            />
          </ElFormItem>
          <div class="flex gap-2">
            <ElButton :loading="loading" type="primary" @click="loadWrongbook">
              查询
            </ElButton>
            <ElButton @click="resetFilters"> 重置 </ElButton>
          </div>
        </ElForm>

        <div class="wrongbook-paper-list">
          <div class="wrongbook-side-title">复习卷记录</div>
          <div
            v-for="paper in papers"
            :key="paper.id"
            class="wrongbook-paper-row"
            @click="openPaper(paper)"
          >
            <div>
              <div class="wrongbook-paper-title">{{ paper.title }}</div>
              <div class="wrongbook-paper-meta">
                {{ paper.question_count }} 道题 ·
                {{ sourceLabel(paper.source) }}
              </div>
            </div>
            <span>{{ shortDate(paper.created_at) }}</span>
          </div>
          <ElEmpty
            v-if="papers.length === 0"
            description="暂无复习卷"
            :image-size="60"
          />
        </div>
      </ElCard>

      <div class="wrongbook-main">
        <ElCard class="sprout-table-card wrongbook-extract-card" shadow="never">
          <template #header>
            <div class="flex items-center justify-between gap-3">
              <span>拍照提题候选区</span>
              <div class="flex gap-2">
                <ElButton :loading="extracting" @click="runExtract">
                  重新提取
                </ElButton>
                <ElButton
                  :disabled="candidates.length === 0"
                  :loading="saving"
                  type="success"
                  @click="saveCandidates"
                >
                  保存选中错题
                </ElButton>
              </div>
            </div>
          </template>
          <ElAlert
            v-if="extractionMocked"
            class="mb-4"
            :closable="false"
            show-icon
            title="当前未配置真实 OCR，系统生成的是待校对卡片；请老师改准确后再保存。"
            type="warning"
          />
          <div class="wrongbook-extract-grid">
            <div class="wrongbook-photo-box">
              <ElImage
                v-if="sourceImagePreview"
                fit="cover"
                :src="sourceImagePreview"
              />
              <div v-else class="wrongbook-photo-empty">
                上传学生作业照片后会显示在这里
              </div>
            </div>
            <ElInput
              v-model="extractionText"
              type="textarea"
              :rows="7"
              placeholder="如果已有 OCR 文本或想快速测试，可粘贴题目文字；系统会按 1. / 2. / 3. 自动拆题。"
            />
          </div>
          <div v-if="candidates.length" class="wrongbook-candidates">
            <div
              v-for="(item, index) in candidates"
              :key="item.temp_id"
              class="wrongbook-candidate-card"
            >
              <div class="wrongbook-candidate-head">
                <ElCheckbox v-model="item.selected">
                  候选题 {{ index + 1 }}
                </ElCheckbox>
                <ElTag type="info">
                  置信度 {{ Math.round(item.confidence * 100) }}%
                </ElTag>
              </div>
              <ElInput
                v-model="item.question_text"
                type="textarea"
                :rows="3"
                placeholder="题目内容"
              />
              <div class="grid gap-3 md:grid-cols-3">
                <ElInput
                  v-model="item.answer_text"
                  placeholder="参考答案（可选）"
                />
                <ElInput
                  v-model="item.knowledge_point"
                  placeholder="知识点（可选）"
                />
                <ElInput
                  v-model="item.explanation"
                  placeholder="改正提示（可选）"
                />
              </div>
            </div>
          </div>
          <ElEmpty
            v-else
            description="还没有候选题，点击上传图片或粘贴文本后提取"
            :image-size="80"
          />
        </ElCard>

        <ElCard class="sprout-table-card" shadow="never">
          <template #header>
            <div class="flex items-center justify-between">
              <span>错题清单</span>
              <ElTag type="warning">
                已选 {{ selectedQuestions.length }} 道
              </ElTag>
            </div>
          </template>
          <ElTable
            v-loading="loading"
            :data="questions"
            row-key="id"
            @selection-change="handleSelectionChange"
          >
            <ElTableColumn type="selection" width="48" />
            <ElTableColumn label="学生" width="96" prop="student_name" />
            <ElTableColumn label="学科" width="90" prop="subject" />
            <ElTableColumn label="题目" min-width="260">
              <template #default="{ row }">
                <div class="wrongbook-question-text">
                  {{ (row as WrongQuestionRecord).question_text }}
                </div>
                <div
                  v-if="(row as WrongQuestionRecord).knowledge_point"
                  class="wrongbook-question-meta"
                >
                  知识点：{{ (row as WrongQuestionRecord).knowledge_point }}
                </div>
              </template>
            </ElTableColumn>
            <ElTableColumn label="答案/备注" min-width="180">
              <template #default="{ row }">
                <div v-if="(row as WrongQuestionRecord).answer_text">
                  答案：{{ (row as WrongQuestionRecord).answer_text }}
                </div>
                <div
                  v-if="(row as WrongQuestionRecord).teacher_note"
                  class="wrongbook-question-meta"
                >
                  备注：{{ (row as WrongQuestionRecord).teacher_note }}
                </div>
              </template>
            </ElTableColumn>
            <ElTableColumn label="状态" width="100">
              <template #default="{ row }">
                <ElTag :type="statusType((row as WrongQuestionRecord).status)">
                  {{ statusLabel((row as WrongQuestionRecord).status) }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="创建时间" width="120">
              <template #default="{ row }">
                {{ shortDate((row as WrongQuestionRecord).created_at) }}
              </template>
            </ElTableColumn>
            <ElTableColumn fixed="right" label="操作" width="190">
              <template #default="{ row }">
                <ElButton
                  link
                  type="success"
                  @click="updateStatus(row as WrongQuestionRecord, 'mastered')"
                >
                  已掌握
                </ElButton>
                <ElButton
                  link
                  type="primary"
                  @click="updateStatus(row as WrongQuestionRecord, 'active')"
                >
                  待复习
                </ElButton>
                <ElButton
                  link
                  type="danger"
                  @click="updateStatus(row as WrongQuestionRecord, 'archived')"
                >
                  归档
                </ElButton>
              </template>
            </ElTableColumn>
            <template #empty>
              <ElEmpty description="暂时没有错题" :image-size="90" />
            </template>
          </ElTable>
        </ElCard>
      </div>
    </div>

    <ElDialog
      v-model="paperDialogVisible"
      :title="currentPaper?.title || '错题复习卷'"
      width="min(760px, 94vw)"
    >
      <div v-if="currentPaper" class="wrongbook-paper-preview">
        <div class="wrongbook-paper-summary">
          {{ currentPaper.student_name }} · 共
          {{ currentPaper.question_count }} 道题 ·
          {{ sourceLabel(currentPaper.source) }}
        </div>
        <div
          v-for="(question, index) in currentPaper.questions || []"
          :key="question.id"
          class="wrongbook-paper-question"
        >
          <div class="wrongbook-paper-index">{{ index + 1 }}</div>
          <div>
            <div class="wrongbook-question-text">
              {{ question.question_text }}
            </div>
            <div v-if="question.answer_text" class="wrongbook-question-meta">
              参考答案：{{ question.answer_text }}
            </div>
            <div v-if="question.explanation" class="wrongbook-question-meta">
              提示：{{ question.explanation }}
            </div>
          </div>
        </div>
      </div>
    </ElDialog>
  </div>
</template>

<style scoped>
.wrongbook-layout {
  display: grid;
  grid-template-columns: minmax(260px, 0.45fr) minmax(0, 1fr);
  gap: 20px;
}

.wrongbook-side-card {
  align-self: start;
}

.wrongbook-main {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

.wrongbook-extract-card {
  overflow: hidden;
}

.wrongbook-extract-grid {
  display: grid;
  grid-template-columns: minmax(220px, 0.45fr) minmax(0, 1fr);
  gap: 16px;
}

.wrongbook-photo-box {
  min-height: 178px;
  overflow: hidden;
  background: #fff9e7;
  border: 1px dashed #e6d8bd;
  border-radius: 20px;
}

.wrongbook-photo-box :deep(.el-image) {
  width: 100%;
  height: 100%;
}

.wrongbook-photo-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 178px;
  padding: 16px;
  color: #8a7a5c;
  text-align: center;
}

.wrongbook-candidates {
  display: grid;
  gap: 14px;
  margin-top: 18px;
}

.wrongbook-candidate-card {
  display: grid;
  gap: 12px;
  padding: 16px;
  background: #fffdf7;
  border: 1px solid #eef0e8;
  border-radius: 20px;
}

.wrongbook-candidate-head,
.wrongbook-paper-row {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.wrongbook-paper-list {
  padding-top: 20px;
  margin-top: 22px;
  border-top: 1px solid #eef0e8;
}

.wrongbook-side-title {
  margin-bottom: 12px;
  font-weight: 700;
  color: #1f2937;
}

.wrongbook-paper-row {
  padding: 12px 0;
  cursor: pointer;
  border-bottom: 1px solid #f1eee7;
}

.wrongbook-paper-title {
  font-weight: 600;
  color: #1f2937;
}

.wrongbook-paper-meta,
.wrongbook-question-meta,
.wrongbook-paper-summary {
  font-size: 12px;
  color: #8b8b8b;
}

.wrongbook-question-text {
  line-height: 1.6;
  color: #1f2937;
  white-space: pre-wrap;
}

.wrongbook-paper-preview {
  display: grid;
  gap: 14px;
}

.wrongbook-paper-question {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  gap: 12px;
  padding: 14px;
  background: #fff9e7;
  border-radius: 16px;
}

.wrongbook-paper-index {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  font-weight: 700;
  color: #fff;
  background: #52c46a;
  border-radius: 12px;
}

@media (max-width: 960px) {
  .wrongbook-layout,
  .wrongbook-extract-grid {
    grid-template-columns: 1fr;
  }
}
</style>
