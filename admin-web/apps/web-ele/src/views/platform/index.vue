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
  createPlatformInviteApi,
  getPlatformInvitesApi,
  getPlatformOrganizationsApi,
  getPlatformRegistrationsApi,
  reviewPlatformRegistrationApi,
  revokePlatformInviteApi,
  setPlatformOrganizationStatusApi,
} from '#/api/platform';

defineOptions({ name: 'PlatformManagement' });

const activeTab = ref('organizations');
const loading = ref(false);
const loadError = ref('');
const organizations = ref<PlatformApi.Organization[]>([]);
const invites = ref<PlatformApi.Invite[]>([]);
const registrations = ref<PlatformApi.Registration[]>([]);
const inviteStatus = ref('');
const registrationStatus = ref('pending');
const inviteDialogVisible = ref(false);
const submittingInvite = ref(false);
const inviteForm = reactive({ expiresAt: '', maxUses: 1, note: '' });

async function loadAll() {
  loading.value = true;
  loadError.value = '';
  try {
    const [organizationResult, inviteResult, registrationResult] =
      await Promise.all([
        getPlatformOrganizationsApi(),
        getPlatformInvitesApi(inviteStatus.value),
        getPlatformRegistrationsApi(registrationStatus.value),
      ]);
    organizations.value = organizationResult.items;
    invites.value = inviteResult.items;
    registrations.value = registrationResult.items;
  } catch {
    loadError.value = '平台数据加载失败，请检查平台管理员权限和后端连接。';
  } finally {
    loading.value = false;
  }
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
            <ElTableColumn label="创建时间" min-width="160">
              <template #default="{ row }">{{
                dayjs(row.createdAt).format('YYYY-MM-DD HH:mm')
              }}</template>
            </ElTableColumn>
            <ElTableColumn
              align="right"
              fixed="right"
              label="操作"
              min-width="120"
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
  </div>
</template>
