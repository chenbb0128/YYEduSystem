<script lang="ts" setup>
import type { FormInstance, FormRules } from 'element-plus';

import type {
  SystemUserPayload,
  SystemUserRecord,
  SystemUserRole,
  SystemUserStatus,
} from '#/api/system/user';

import { computed, onMounted, reactive, ref } from 'vue';

import dayjs from 'dayjs';
import {
  ElAlert,
  ElButton,
  ElCard,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';

import {
  createSystemUserApi,
  deleteSystemUserApi,
  getSystemUsersApi,
  updateSystemUserApi,
} from '#/api/system/user';
import { $t } from '#/locales';

import { buildUserListParams, createUserForm, toUserForm } from './model';

defineOptions({ name: 'SystemUsers' });

type DialogMode = 'create' | 'edit';

const DEFAULT_PAGE_SIZE = 10;

const users = ref<SystemUserRecord[]>([]);
const loading = ref(false);
const loadError = ref('');
const query = reactive({
  keyword: '',
  status: '' as '' | SystemUserStatus,
});
const pagination = reactive({
  page: 1,
  pageSize: DEFAULT_PAGE_SIZE,
  total: 0,
});

const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>('create');
const editingId = ref<null | number>(null);
const submitting = ref(false);
const formRef = ref<FormInstance>();
const form = reactive<SystemUserPayload>(createUserForm());

const formRules = computed<FormRules>(() => ({
  realName: [
    {
      message: $t('page.users.realNameRequired'),
      required: true,
      trigger: 'blur',
    },
  ],
  username: [
    {
      message: $t('page.users.usernameRequired'),
      required: true,
      trigger: 'blur',
    },
    {
      max: 32,
      message: $t('page.users.usernameLength'),
      min: 3,
      trigger: 'blur',
    },
  ],
  password: [
    {
      message: '请输入至少 6 位密码',
      min: 6,
      required: dialogMode.value === 'create',
      trigger: 'blur',
    },
  ],
}));

const dialogTitle = computed(() =>
  dialogMode.value === 'create'
    ? $t('page.users.create')
    : $t('page.users.edit'),
);

const roleOptions: Array<{ label: string; value: SystemUserRole }> = [
  { label: 'page.users.roles.admin', value: 'admin' },
  { label: 'page.users.roles.editor', value: 'editor' },
  { label: 'page.users.roles.teacher', value: 'teacher' },
  { label: 'page.users.roles.viewer', value: 'viewer' },
];

async function loadUsers() {
  loading.value = true;
  loadError.value = '';
  try {
    const result = await getSystemUsersApi(
      buildUserListParams(query, pagination.page, pagination.pageSize),
    );
    users.value = result.items;
    pagination.total = result.total;
  } catch {
    loadError.value = $t('page.users.loadError');
    users.value = [];
    pagination.total = 0;
  } finally {
    loading.value = false;
  }
}

function handleSearch() {
  pagination.page = 1;
  void loadUsers();
}

function handleReset() {
  query.keyword = '';
  query.status = '';
  pagination.page = 1;
  void loadUsers();
}

function handlePageChange(page: number) {
  pagination.page = page;
  void loadUsers();
}

function handleSizeChange(pageSize: number) {
  pagination.page = 1;
  pagination.pageSize = pageSize;
  void loadUsers();
}

function openCreateDialog() {
  dialogMode.value = 'create';
  editingId.value = null;
  Object.assign(form, createUserForm());
  dialogVisible.value = true;
}

function openEditDialog(user: SystemUserRecord) {
  dialogMode.value = 'edit';
  editingId.value = user.id;
  Object.assign(form, toUserForm(user));
  dialogVisible.value = true;
}

async function submitForm() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;

  submitting.value = true;
  try {
    if (dialogMode.value === 'create') {
      await createSystemUserApi({ ...form });
      ElMessage.success($t('page.users.createSuccess'));
    } else if (editingId.value !== null) {
      await updateSystemUserApi(editingId.value, { ...form });
      ElMessage.success($t('page.users.updateSuccess'));
    }
    dialogVisible.value = false;
    await loadUsers();
  } finally {
    submitting.value = false;
  }
}

async function removeUser(user: SystemUserRecord) {
  try {
    await ElMessageBox.confirm(
      $t('page.users.confirmDelete', [user.username]),
      $t('page.users.deleteTitle'),
      {
        cancelButtonText: $t('page.users.cancel'),
        confirmButtonText: $t('page.users.confirm'),
        type: 'warning',
      },
    );
  } catch {
    return;
  }

  await deleteSystemUserApi(user.id);
  ElMessage.success($t('page.users.deleteSuccess'));
  if (users.value.length === 1 && pagination.page > 1) {
    pagination.page -= 1;
  }
  await loadUsers();
}

function roleLabel(role: SystemUserRole) {
  return $t(`page.users.roles.${role}`);
}

function roleType(role: SystemUserRole) {
  if (role === 'admin') return 'warning';
  if (role === 'teacher') return 'success';
  if (role === 'editor') return 'primary';
  return 'info';
}

function statusLabel(status: SystemUserStatus) {
  return $t(`page.users.statuses.${status}`);
}

onMounted(loadUsers);
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">系统设置 · 账号与权限</p>
        <h1 class="sprout-page-title">{{ $t('page.users.title') }}</h1>
        <p class="sprout-page-description">
          {{ $t('page.users.description') }}
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton
          v-access:code="'system:user:create'"
          type="primary"
          @click="openCreateDialog"
        >
          {{ $t('page.users.create') }}
        </ElButton>
      </div>
    </div>

    <ElAlert
      class="mb-4"
      :closable="false"
      show-icon
      title="小程序老师端将使用手机号验证码登录；请把老师手机号登记在“登录手机号/账号”中。"
      type="success"
    />

    <ElCard class="sprout-filter-card" shadow="never">
      <div class="sprout-filter-panel">
        <ElInput
          v-model="query.keyword"
          class="md:max-w-72"
          clearable
          :placeholder="$t('page.users.keywordPlaceholder')"
          @keyup.enter="handleSearch"
        />
        <ElSelect
          v-model="query.status"
          class="md:w-44"
          :placeholder="$t('page.users.status')"
        >
          <ElOption :label="$t('page.users.allStatus')" value="" />
          <ElOption :label="$t('page.users.statuses.active')" value="active" />
          <ElOption
            :label="$t('page.users.statuses.disabled')"
            value="disabled"
          />
        </ElSelect>
        <div class="flex gap-2">
          <ElButton type="primary" @click="handleSearch">
            {{ $t('page.users.search') }}
          </ElButton>
          <ElButton @click="handleReset">
            {{ $t('page.users.reset') }}
          </ElButton>
        </div>
      </div>
    </ElCard>

    <ElAlert
      v-if="loadError"
      :closable="false"
      show-icon
      :title="loadError"
      type="error"
    />

    <ElCard class="sprout-table-card" shadow="never">
      <div class="sprout-table-toolbar mb-4">
        <div>
          <h2 class="sprout-section-title">账号列表</h2>
          <p class="sprout-section-caption">
            管理员、教师和其他账号的登录状态与操作权限。
          </p>
        </div>
        <span class="sprout-role-badge">共 {{ pagination.total }} 个账号</span>
      </div>
      <div class="sprout-table-wrap">
        <ElTable v-loading="loading" :data="users" row-key="id">
          <ElTableColumn
            :label="$t('page.users.username')"
            min-width="150"
            prop="username"
          />
          <ElTableColumn
            :label="$t('page.users.realName')"
            min-width="160"
            prop="realName"
          />
          <ElTableColumn :label="$t('page.users.role')" min-width="120">
            <template #default="{ row }">
              <ElTag
                class="sprout-status"
                effect="light"
                :type="roleType(row.role)"
              >
                {{ roleLabel(row.role) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="$t('page.users.status')" min-width="120">
            <template #default="{ row }">
              <ElTag :type="row.status === 'active' ? 'success' : 'info'">
                {{ statusLabel(row.status) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="$t('page.users.createdAt')" min-width="180">
            <template #default="{ row }">
              {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm') }}
            </template>
          </ElTableColumn>
          <ElTableColumn
            align="right"
            fixed="right"
            :label="$t('page.users.actions')"
            min-width="150"
          >
            <template #default="{ row }">
              <ElButton
                v-access:code="'system:user:update'"
                link
                type="primary"
                @click="openEditDialog(row as SystemUserRecord)"
              >
                {{ $t('page.users.edit') }}
              </ElButton>
              <ElButton
                v-access:code="'system:user:delete'"
                :disabled="row.id === 1"
                link
                type="danger"
                @click="removeUser(row as SystemUserRecord)"
              >
                {{ $t('page.users.delete') }}
              </ElButton>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty :description="$t('page.users.empty')" :image-size="80" />
          </template>
        </ElTable>
      </div>

      <div class="mt-4 flex justify-end">
        <ElPagination
          background
          :current-page="pagination.page"
          layout="total, sizes, prev, pager, next"
          :page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50]"
          :total="pagination.total"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      destroy-on-close
      :title="dialogTitle"
      width="min(520px, 92vw)"
    >
      <ElForm
        ref="formRef"
        label-position="top"
        :model="form"
        :rules="formRules"
      >
        <ElFormItem :label="$t('page.users.username')" prop="username">
          <ElInput
            v-model="form.username"
            :disabled="dialogMode === 'edit'"
            maxlength="32"
            placeholder="例如：13800000000"
          />
        </ElFormItem>
        <ElFormItem :label="$t('page.users.realName')" prop="realName">
          <ElInput v-model="form.realName" maxlength="40" />
        </ElFormItem>
        <ElFormItem label="登录密码" prop="password">
          <ElInput
            v-model="form.password"
            autocomplete="new-password"
            :placeholder="
              dialogMode === 'edit' ? '不修改请留空' : '至少 6 位密码'
            "
            type="password"
            show-password
          />
        </ElFormItem>
        <ElFormItem :label="$t('page.users.role')" prop="role">
          <ElSelect v-model="form.role" class="w-full">
            <ElOption
              v-for="option in roleOptions"
              :key="option.value"
              :label="$t(option.label)"
              :value="option.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('page.users.status')" prop="status">
          <ElSelect v-model="form.status" class="w-full">
            <ElOption
              :label="$t('page.users.statuses.active')"
              value="active"
            />
            <ElOption
              :label="$t('page.users.statuses.disabled')"
              value="disabled"
            />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">
          {{ $t('page.users.cancel') }}
        </ElButton>
        <ElButton :loading="submitting" type="primary" @click="submitForm">
          {{ $t('page.users.save') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
