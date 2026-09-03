<script lang="ts" setup>
import type { PlatformApi } from '#/api/platform';

import { onMounted, reactive, ref } from 'vue';

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
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTabPane,
  ElTabs,
  ElTag,
} from 'element-plus';

import {
  createPlatformAdminApi,
  createPlatformInviteApi,
  getPlatformAdminsApi,
  getPlatformInvitesApi,
  getPlatformOrganizationsApi,
  getPlatformOverviewApi,
  getPlatformRegistrationsApi,
  reviewPlatformRegistrationApi,
  revokePlatformInviteApi,
  setPlatformAdminStatusApi,
  setPlatformOrganizationAuthorizationApi,
  setPlatformOrganizationStatusApi,
  updatePlatformAdminApi,
} from '#/api/platform';

defineOptions({ name: 'PlatformManagement' });

const activeTab = ref('organizations');
const loading = ref(false);
const loadError = ref('');
const overview = ref<PlatformApi.Overview>({
  activeOrganizationCount: 0,
  availableInviteCount: 0,
  disabledOrganizationCount: 0,
  exhaustedInviteCount: 0,
  organizationCount: 0,
  pendingRegistrationCount: 0,
});
const organizations = ref<PlatformApi.Organization[]>([]);
const invites = ref<PlatformApi.Invite[]>([]);
const registrations = ref<PlatformApi.Registration[]>([]);
const platformAdmins = ref<PlatformApi.PlatformAdmin[]>([]);
const organizationKeyword = ref('');
const organizationStatus = ref<'' | PlatformApi.Organization['status']>('');
const inviteStatus = ref('');
const registrationStatus = ref('pending');
const adminKeyword = ref('');
const adminStatus = ref<'' | 'active' | 'disabled'>('');
const inviteDialogVisible = ref(false);
const authorizationDialogVisible = ref(false);
const adminDialogVisible = ref(false);
const adminDialogMode = ref<'create' | 'edit'>('create');
const editingAdminId = ref<null | number>(null);
const submittingInvite = ref(false);
const submittingAdmin = ref(false);
const inviteForm = reactive({ expiresAt: '', maxUses: 1, note: '' });
const authorizationForm = reactive({ organizationId: 0, authorizedUntil: '' });
const adminForm = reactive({
  password: '',
  realName: '',
  status: 'active' as 'active' | 'disabled',
  username: '',
});

async function loadAll() {
  loading.value = true;
  loadError.value = '';
  try {
    const [
      overviewResult,
      organizationResult,
      inviteResult,
      registrationResult,
      adminResult,
    ] = await Promise.all([
      getPlatformOverviewApi(),
      getPlatformOrganizationsApi({
        keyword: organizationKeyword.value || undefined,
        status: organizationStatus.value,
      }),
      getPlatformInvitesApi(inviteStatus.value),
      getPlatformRegistrationsApi(registrationStatus.value),
      getPlatformAdminsApi({
        keyword: adminKeyword.value || undefined,
        status: adminStatus.value,
      }),
    ]);
    overview.value = overviewResult;
    organizations.value = organizationResult.items;
    invites.value = inviteResult.items;
    registrations.value = registrationResult.items;
    platformAdmins.value = adminResult.items;
  } catch {
    loadError.value = '平台数据加载失败，请检查平台管理员权限和后端连接。';
  } finally {
    loading.value = false;
  }
}

function searchOrganizations() {
  void loadAll();
}

function resetOrganizationSearch() {
  organizationKeyword.value = '';
  organizationStatus.value = '';
  void loadAll();
}

function searchPlatformAdmins() {
  void loadAll();
}

function resetPlatformAdminSearch() {
  adminKeyword.value = '';
  adminStatus.value = '';
  void loadAll();
}

function openInviteDialog() {
  inviteForm.expiresAt = '';
  inviteForm.maxUses = 1;
  inviteForm.note = '';
  inviteDialogVisible.value = true;
}

function asOrganization(item: unknown) {
  return item as PlatformApi.Organization;
}

function asInvite(item: unknown) {
  return item as PlatformApi.Invite;
}

function asRegistration(item: unknown) {
  return item as PlatformApi.Registration;
}

async function submitInvite() {
  if (!Number.isInteger(inviteForm.maxUses) || inviteForm.maxUses < 1) {
    ElMessage.warning('使用次数至少为 1');
    return;
  }
  submittingInvite.value = true;
  try {
    const result = await createPlatformInviteApi({
      expiresAt: inviteForm.expiresAt || undefined,
      maxUses: inviteForm.maxUses,
      note: inviteForm.note || undefined,
    });
    inviteDialogVisible.value = false;
    await ElMessageBox.alert(
      `<div style="line-height:1.8">邀请码：<strong style="font-size:20px;letter-spacing:1px">${result.code}</strong><br/>${result.warning}</div>`,
      '邀请码已生成',
      { dangerouslyUseHTMLString: true, confirmButtonText: '我已保存' },
    );
    await loadAll();
  } finally {
    submittingInvite.value = false;
  }
}

async function revokeInvite(item: PlatformApi.Invite) {
  try {
    await ElMessageBox.confirm(
      `确定撤销邀请码 ${item.codeHint}… 吗？撤销后不能继续注册。`,
      '撤销邀请码',
      {
        type: 'warning',
        confirmButtonText: '确认撤销',
        cancelButtonText: '取消',
      },
    );
    await revokePlatformInviteApi(item.id);
    ElMessage.success('邀请码已撤销');
    await loadAll();
  } catch {
    // 用户取消或接口失败由请求拦截器提示。
  }
}

async function toggleOrganization(item: PlatformApi.Organization) {
  const nextStatus = item.status === 'active' ? 'disabled' : 'active';
  try {
    await ElMessageBox.confirm(
      `${nextStatus === 'active' ? '恢复' : '停用'}机构“${item.name}”吗？`,
      '机构状态调整',
      { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' },
    );
    await setPlatformOrganizationStatusApi(item.id, nextStatus);
    ElMessage.success('机构状态已更新');
    await loadAll();
  } catch {
    // 用户取消或接口失败由请求拦截器提示。
  }
}

function openAuthorizationDialog(item: PlatformApi.Organization) {
  authorizationForm.organizationId = item.id;
  authorizationForm.authorizedUntil = item.authorizedUntil || '';
  authorizationDialogVisible.value = true;
}

async function submitAuthorization() {
  if (!authorizationForm.organizationId) return;
  try {
    await setPlatformOrganizationAuthorizationApi(
      authorizationForm.organizationId,
      authorizationForm.authorizedUntil,
    );
    authorizationDialogVisible.value = false;
    ElMessage.success(
      authorizationForm.authorizedUntil ? '服务期限已更新' : '服务期限已清除',
    );
    await loadAll();
  } catch {
    // 接口错误由请求拦截器提示。
  }
}

function openAdminDialog(item?: PlatformApi.PlatformAdmin) {
  if (item) {
    adminDialogMode.value = 'edit';
    editingAdminId.value = item.id;
    adminForm.username = item.username;
    adminForm.realName = item.realName;
    adminForm.password = '';
    adminForm.status = item.status;
  } else {
    adminDialogMode.value = 'create';
    editingAdminId.value = null;
    adminForm.username = '';
    adminForm.realName = '';
    adminForm.password = '';
    adminForm.status = 'active';
  }
  adminDialogVisible.value = true;
}

async function submitAdmin() {
  if (!adminForm.username.trim() || !adminForm.realName.trim()) {
    ElMessage.warning('请填写账号和姓名');
    return;
  }
  if (adminDialogMode.value === 'create' && adminForm.password.length < 6) {
    ElMessage.warning('初始密码至少 6 位');
    return;
  }
  if (adminForm.password && adminForm.password.length < 6) {
    ElMessage.warning('密码至少 6 位');
    return;
  }
  submittingAdmin.value = true;
  try {
    if (adminDialogMode.value === 'create') {
      await createPlatformAdminApi({
        password: adminForm.password,
        realName: adminForm.realName.trim(),
        status: adminForm.status,
        username: adminForm.username.trim(),
      });
      ElMessage.success('平台管理员已创建');
    } else if (editingAdminId.value !== null) {
      await updatePlatformAdminApi(editingAdminId.value, {
        password: adminForm.password || undefined,
        realName: adminForm.realName.trim(),
        status: adminForm.status,
      });
      ElMessage.success('平台管理员信息已更新');
    }
    adminDialogVisible.value = false;
    await loadAll();
  } finally {
    submittingAdmin.value = false;
  }
}

async function togglePlatformAdmin(item: PlatformApi.PlatformAdmin) {
  const nextStatus = item.status === 'active' ? 'disabled' : 'active';
  try {
    await ElMessageBox.confirm(
      `${nextStatus === 'active' ? '启用' : '停用'}平台管理员“${item.realName || item.username}”吗？`,
      '调整平台管理员状态',
      { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' },
    );
    await setPlatformAdminStatusApi(item.id, nextStatus);
    ElMessage.success('平台管理员状态已更新');
    await loadAll();
  } catch {
    // 用户取消或接口失败由请求拦截器提示。
  }
}

async function reviewRegistration(
  item: PlatformApi.Registration,
  status: 'approved' | 'rejected',
) {
  const action = status === 'approved' ? '通过' : '拒绝';
  try {
    const result = await ElMessageBox.prompt(
      status === 'approved' ? '可填写审核备注（选填）' : '请输入拒绝原因',
      `${action}注册申请`,
      {
        inputPlaceholder: '审核备注',
        inputValue: '',
        inputValidator: (value) =>
          status === 'rejected' && !value.trim() ? '请填写拒绝原因' : true,
        confirmButtonText: `确认${action}`,
        cancelButtonText: '取消',
      },
    );
    await reviewPlatformRegistrationApi(item.id, {
      reviewNote: result.value,
      status,
    });
    ElMessage.success(`申请已${action}`);
    await loadAll();
  } catch {
    // 用户取消或接口失败由请求拦截器提示。
  }
}

function organizationStatusLabel(status: PlatformApi.Organization['status']) {
  return { active: '正常', disabled: '已停用', pending: '待开通' }[status];
}

function inviteStatusLabel(status: PlatformApi.Invite['status']) {
  return { active: '可使用', exhausted: '已用完', revoked: '已撤销' }[status];
}

function registrationStatusLabel(status: PlatformApi.Registration['status']) {
  return { approved: '已通过', pending: '待审核', rejected: '已拒绝' }[status];
}

function authorizationLabel(item: PlatformApi.Organization) {
  if (!item.authorizedUntil) return '未设置期限';
  const expiry = dayjs(item.authorizedUntil);
  const days = expiry.startOf('day').diff(dayjs().startOf('day'), 'day');
  if (days < 0) return '已到期';
  if (days <= 30) return `剩余 ${days} 天`;
  return expiry.format('YYYY-MM-DD');
}

function authorizationType(item: PlatformApi.Organization) {
  if (!item.authorizedUntil) return 'info';
  const days = dayjs(item.authorizedUntil)
    .startOf('day')
    .diff(dayjs().startOf('day'), 'day');
  if (days < 0) return 'danger';
  if (days <= 30) return 'warning';
  return 'success';
}

onMounted(loadAll);
</script>

<template>
  <div class="sprout-page">
    <div class="sprout-page-header">
      <div class="sprout-page-heading">
        <p class="sprout-page-kicker">平台拥有者 · 商业化运营</p>
        <h1 class="sprout-page-title">平台总管理</h1>
        <p class="sprout-page-description">
          管理托管机构、注册申请和入驻邀请码。这里不参与任何机构的日常接送、作业和家长业务。
        </p>
      </div>
      <div class="sprout-header-actions">
        <ElButton type="primary" @click="openInviteDialog">生成邀请码</ElButton>
        <ElButton :loading="loading" @click="loadAll">刷新</ElButton>
      </div>
    </div>

    <ElAlert
      :closable="false"
      show-icon
      title="平台管理员权限与托管机构管理员完全分离；邀请码明文只在生成成功后展示一次。"
      type="success"
    />
    <ElAlert
      v-if="loadError"
      :closable="false"
      show-icon
      :title="loadError"
      type="error"
    />

    <div class="platform-stat-grid mt-4">
      <ElCard shadow="never">
        <p class="platform-stat-label">机构总数</p>
        <p class="platform-stat-value">{{ overview.organizationCount }}</p>
        <p class="platform-stat-hint">
          正常 {{ overview.activeOrganizationCount }} · 停用
          {{ overview.disabledOrganizationCount }}
        </p>
      </ElCard>
      <ElCard shadow="never">
        <p class="platform-stat-label">待审核注册</p>
        <p class="platform-stat-value platform-stat-value--warning">
          {{ overview.pendingRegistrationCount }}
        </p>
        <p class="platform-stat-hint">需要平台管理员处理的入驻申请</p>
      </ElCard>
      <ElCard shadow="never">
        <p class="platform-stat-label">可用邀请码</p>
        <p class="platform-stat-value platform-stat-value--success">
          {{ overview.availableInviteCount }}
        </p>
        <p class="platform-stat-hint">
          已用完 {{ overview.exhaustedInviteCount }} 个
        </p>
      </ElCard>
      <ElCard shadow="never">
        <p class="platform-stat-label">平台管理员</p>
        <p class="platform-stat-value">{{ platformAdmins.length }}</p>
        <p class="platform-stat-hint">仅负责平台和机构生命周期管理</p>
      </ElCard>
    </div>

    <ElCard class="sprout-table-card mt-4" shadow="never">
      <ElTabs v-model="activeTab" @tab-change="loadAll">
        <ElTabPane label="机构管理" name="organizations">
          <div class="sprout-table-toolbar mb-4">
            <div>
              <h2 class="sprout-section-title">托管机构</h2>
              <p class="sprout-section-caption">
                机构停用后，机构管理员和教师不能继续使用业务接口。
              </p>
            </div>
            <span class="sprout-role-badge"
              >共 {{ organizations.length }} 家</span
            >
          </div>
          <div class="sprout-filter-panel mb-4">
            <ElInput
              v-model="organizationKeyword"
              class="md:max-w-72"
              clearable
              placeholder="搜索机构名称、标识或联系人"
              @keyup.enter="searchOrganizations"
            />
            <ElSelect
              v-model="organizationStatus"
              class="md:w-44"
              placeholder="机构状态"
              @change="searchOrganizations"
            >
              <ElOption label="全部状态" value="" />
              <ElOption label="正常" value="active" />
              <ElOption label="待开通" value="pending" />
              <ElOption label="已停用" value="disabled" />
            </ElSelect>
            <ElButton type="primary" @click="searchOrganizations"
              >搜索</ElButton
            >
            <ElButton @click="resetOrganizationSearch">重置</ElButton>
          </div>
          <ElTable v-loading="loading" :data="organizations" row-key="id">
            <ElTableColumn label="机构名称" min-width="180" prop="name" />
            <ElTableColumn label="标识" min-width="140" prop="slug" />
            <ElTableColumn label="联系人" min-width="140">
              <template #default="{ row }">{{
                row.contactName || '未填写'
              }}</template>
            </ElTableColumn>
            <ElTableColumn label="状态" min-width="100">
              <template #default="{ row }">
                <ElTag
                  :type="
                    row.status === 'active'
                      ? 'success'
                      : row.status === 'pending'
                        ? 'warning'
                        : 'info'
                  "
                >
                  {{ organizationStatusLabel(row.status) }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="服务期限" min-width="150">
              <template #default="{ row }">
                <ElTag
                  :type="authorizationType(row as PlatformApi.Organization)"
                >
                  {{ authorizationLabel(row as PlatformApi.Organization) }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="创建时间" min-width="160">
              <template #default="{ row }">{{
                dayjs(row.createdAt).format('YYYY-MM-DD HH:mm')
              }}</template>
            </ElTableColumn>
            <ElTableColumn
              align="right"
              fixed="right"
              label="操作"
              min-width="220"
            >
              <template #default="{ row }">
                <ElButton
                  v-if="row.id !== 1"
                  link
                  type="primary"
                  @click="toggleOrganization(asOrganization(row))"
                >
                  {{ row.status === 'active' ? '停用' : '恢复' }}
                </ElButton>
                <ElButton
                  link
                  type="primary"
                  @click="openAuthorizationDialog(asOrganization(row))"
                >
                  设置期限
                </ElButton>
              </template>
            </ElTableColumn>
            <template #empty
              ><ElEmpty description="暂无机构" :image-size="80"
            /></template>
          </ElTable>
        </ElTabPane>

        <ElTabPane label="邀请码" name="invites">
          <div class="sprout-filter-panel mb-4">
            <ElSelect
              v-model="inviteStatus"
              class="w-44"
              placeholder="筛选状态"
              @change="loadAll"
            >
              <ElOption label="全部状态" value="" />
              <ElOption label="可使用" value="active" />
              <ElOption label="已用完" value="exhausted" />
              <ElOption label="已撤销" value="revoked" />
            </ElSelect>
            <ElButton type="primary" @click="openInviteDialog"
              >生成邀请码</ElButton
            >
          </div>
          <ElTable v-loading="loading" :data="invites" row-key="id">
            <ElTableColumn label="邀请码提示" min-width="170" prop="codeHint" />
            <ElTableColumn label="使用情况" min-width="130">
              <template #default="{ row }"
                >{{ row.usedCount }} / {{ row.maxUses }}</template
              >
            </ElTableColumn>
            <ElTableColumn label="有效期" min-width="150">
              <template #default="{ row }">{{
                row.expiresAt
                  ? dayjs(row.expiresAt).format('YYYY-MM-DD')
                  : '长期有效'
              }}</template>
            </ElTableColumn>
            <ElTableColumn label="状态" min-width="100">
              <template #default="{ row }"
                ><ElTag :type="row.status === 'active' ? 'success' : 'info'">{{
                  inviteStatusLabel(row.status)
                }}</ElTag></template
              >
            </ElTableColumn>
            <ElTableColumn label="备注" min-width="180" prop="note" />
            <ElTableColumn align="right" label="操作" min-width="100">
              <template #default="{ row }"
                ><ElButton
                  v-if="row.status === 'active'"
                  link
                  type="danger"
                  @click="revokeInvite(asInvite(row))"
                  >撤销</ElButton
                ></template
              >
            </ElTableColumn>
            <template #empty
              ><ElEmpty description="暂无邀请码" :image-size="80"
            /></template>
          </ElTable>
        </ElTabPane>

        <ElTabPane label="注册审核" name="registrations">
          <div class="sprout-filter-panel mb-4">
            <ElSelect
              v-model="registrationStatus"
              class="w-44"
              placeholder="筛选状态"
              @change="loadAll"
            >
              <ElOption label="待审核" value="pending" />
              <ElOption label="已通过" value="approved" />
              <ElOption label="已拒绝" value="rejected" />
              <ElOption label="全部状态" value="" />
            </ElSelect>
          </div>
          <ElTable v-loading="loading" :data="registrations" row-key="id">
            <ElTableColumn
              label="机构名称"
              min-width="180"
              prop="organizationName"
            />
            <ElTableColumn
              label="管理员账号"
              min-width="150"
              prop="adminUsername"
            />
            <ElTableColumn label="联系人" min-width="140">
              <template #default="{ row }"
                >{{ row.contactName || '未填写'
                }}{{
                  row.contactPhone ? ` · ${row.contactPhone}` : ''
                }}</template
              >
            </ElTableColumn>
            <ElTableColumn label="状态" min-width="100">
              <template #default="{ row }"
                ><ElTag
                  :type="
                    row.status === 'approved'
                      ? 'success'
                      : row.status === 'pending'
                        ? 'warning'
                        : 'info'
                  "
                  >{{ registrationStatusLabel(row.status) }}</ElTag
                ></template
              >
            </ElTableColumn>
            <ElTableColumn label="提交时间" min-width="160">
              <template #default="{ row }">{{
                dayjs(row.createdAt).format('YYYY-MM-DD HH:mm')
              }}</template>
            </ElTableColumn>
            <ElTableColumn
              align="right"
              fixed="right"
              label="操作"
              min-width="170"
            >
              <template #default="{ row }">
                <template v-if="row.status === 'pending'">
                  <ElButton
                    link
                    type="primary"
                    @click="reviewRegistration(asRegistration(row), 'approved')"
                    >通过</ElButton
                  >
                  <ElButton
                    link
                    type="danger"
                    @click="reviewRegistration(asRegistration(row), 'rejected')"
                    >拒绝</ElButton
                  >
                </template>
                <span v-else class="text-gray-400">已处理</span>
              </template>
            </ElTableColumn>
            <template #empty
              ><ElEmpty description="暂无注册申请" :image-size="80"
            /></template>
          </ElTable>
        </ElTabPane>

        <ElTabPane label="平台管理员" name="admins">
          <div class="sprout-table-toolbar mb-4">
            <div>
              <h2 class="sprout-section-title">平台管理员</h2>
              <p class="sprout-section-caption">
                只管理豆芽成长助手的平台账号，不会进入任何托管班的日常业务。
              </p>
            </div>
            <ElButton type="primary" @click="openAdminDialog()"
              >新增管理员</ElButton
            >
          </div>
          <div class="sprout-filter-panel mb-4">
            <ElInput
              v-model="adminKeyword"
              class="md:max-w-72"
              clearable
              placeholder="搜索账号或姓名"
              @keyup.enter="searchPlatformAdmins"
            />
            <ElSelect
              v-model="adminStatus"
              class="md:w-44"
              placeholder="账号状态"
              @change="searchPlatformAdmins"
            >
              <ElOption label="全部状态" value="" />
              <ElOption label="正常" value="active" />
              <ElOption label="已停用" value="disabled" />
            </ElSelect>
            <ElButton type="primary" @click="searchPlatformAdmins"
              >搜索</ElButton
            >
            <ElButton @click="resetPlatformAdminSearch">重置</ElButton>
          </div>
          <ElTable v-loading="loading" :data="platformAdmins" row-key="id">
            <ElTableColumn label="登录账号" min-width="180" prop="username" />
            <ElTableColumn label="姓名" min-width="150" prop="realName" />
            <ElTableColumn label="状态" min-width="110">
              <template #default="{ row }">
                <ElTag :type="row.status === 'active' ? 'success' : 'info'">
                  {{ row.status === 'active' ? '正常' : '已停用' }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="创建时间" min-width="170">
              <template #default="{ row }">
                {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm') }}
              </template>
            </ElTableColumn>
            <ElTableColumn
              align="right"
              fixed="right"
              label="操作"
              min-width="180"
            >
              <template #default="{ row }">
                <ElButton
                  link
                  type="primary"
                  @click="openAdminDialog(row as PlatformApi.PlatformAdmin)"
                >
                  编辑
                </ElButton>
                <ElButton
                  link
                  :type="row.status === 'active' ? 'danger' : 'success'"
                  @click="togglePlatformAdmin(row as PlatformApi.PlatformAdmin)"
                >
                  {{ row.status === 'active' ? '停用' : '启用' }}
                </ElButton>
              </template>
            </ElTableColumn>
            <template #empty>
              <ElEmpty description="暂无平台管理员" :image-size="80" />
            </template>
          </ElTable>
        </ElTabPane>
      </ElTabs>
    </ElCard>

    <ElDialog
      v-model="inviteDialogVisible"
      title="生成机构邀请码"
      width="min(520px, 92vw)"
    >
      <ElForm label-position="top" :model="inviteForm">
        <ElFormItem label="可使用次数">
          <ElInput v-model.number="inviteForm.maxUses" type="number" min="1" />
        </ElFormItem>
        <ElFormItem label="有效期（选填）">
          <ElInput
            v-model="inviteForm.expiresAt"
            placeholder="例如：2026-12-31"
          />
        </ElFormItem>
        <ElFormItem label="备注（选填）">
          <ElInput
            v-model="inviteForm.note"
            maxlength="255"
            placeholder="例如：某渠道 2026 秋季入驻"
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="inviteDialogVisible = false">取消</ElButton>
        <ElButton
          :loading="submittingInvite"
          type="primary"
          @click="submitInvite"
          >生成并显示</ElButton
        >
      </template>
    </ElDialog>

    <ElDialog
      v-model="authorizationDialogVisible"
      title="设置机构服务期限"
      width="min(460px, 92vw)"
    >
      <ElForm label-position="top" :model="authorizationForm">
        <ElFormItem label="服务截止日期">
          <ElInput v-model="authorizationForm.authorizedUntil" type="date" />
          <p class="platform-dialog-tip">
            留空表示不设置截止日期。此处用于平台提醒，机构停用仍需单独操作。
          </p>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="authorizationDialogVisible = false">取消</ElButton>
        <ElButton type="primary" @click="submitAuthorization">保存</ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="adminDialogVisible"
      destroy-on-close
      :title="
        adminDialogMode === 'create' ? '新增平台管理员' : '编辑平台管理员'
      "
      width="min(520px, 92vw)"
    >
      <ElForm label-position="top" :model="adminForm">
        <ElFormItem label="登录账号">
          <ElInput
            v-model="adminForm.username"
            :disabled="adminDialogMode === 'edit'"
            maxlength="64"
            placeholder="至少 3 位字母、数字或手机号"
          />
        </ElFormItem>
        <ElFormItem label="姓名">
          <ElInput
            v-model="adminForm.realName"
            maxlength="64"
            placeholder="例如：平台运营"
          />
        </ElFormItem>
        <ElFormItem label="登录密码">
          <ElInput
            v-model="adminForm.password"
            autocomplete="new-password"
            :placeholder="
              adminDialogMode === 'edit' ? '不修改请留空' : '至少 6 位密码'
            "
            type="password"
            show-password
          />
        </ElFormItem>
        <ElFormItem label="账号状态">
          <ElSelect v-model="adminForm.status" class="w-full">
            <ElOption label="正常" value="active" />
            <ElOption label="已停用" value="disabled" />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="adminDialogVisible = false">取消</ElButton>
        <ElButton
          :loading="submittingAdmin"
          type="primary"
          @click="submitAdmin"
        >
          保存
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
.platform-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.platform-stat-grid :deep(.el-card__body) {
  padding: 20px;
}

.platform-stat-label,
.platform-stat-hint {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.platform-stat-value {
  margin: 10px 0 6px;
  font-size: 30px;
  font-weight: 700;
  line-height: 1;
  color: #0f172a;
}

.platform-stat-value--warning {
  color: #d97706;
}

.platform-stat-value--success {
  color: #059669;
}

.platform-dialog-tip {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: #64748b;
}

@media (max-width: 900px) {
  .platform-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .platform-stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>
