<script lang="ts" setup>
import type { VbenFormSchema } from '@vben/common-ui';

import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';

import { AuthenticationRegister, z } from '@vben/common-ui';

import { registerOrganizationApi } from '#/api/platform';

defineOptions({ name: 'OrganizationRegister' });

const router = useRouter();
const loading = ref(false);

const formSchema = computed((): VbenFormSchema[] => [
  {
    component: 'VbenInput',
    componentProps: { placeholder: '请输入平台提供的邀请码' },
    fieldName: 'inviteCode',
    label: '邀请码',
    rules: z.string().min(8, { message: '请输入有效邀请码' }),
  },
  {
    component: 'VbenInput',
    componentProps: { placeholder: '例如：阳光托管中心' },
    fieldName: 'organizationName',
    label: '机构名称',
    rules: z.string().min(2, { message: '请输入机构名称' }),
  },
  {
    component: 'VbenInput',
    componentProps: { placeholder: '负责人姓名（选填）' },
    fieldName: 'contactName',
    label: '负责人',
  },
  {
    component: 'VbenInput',
    componentProps: { placeholder: '联系电话（选填）' },
    fieldName: 'contactPhone',
    label: '联系电话',
  },
  {
    component: 'VbenInput',
    componentProps: { placeholder: '设置机构管理员登录账号' },
    fieldName: 'adminUsername',
    label: '管理员账号',
    rules: z.string().min(3, { message: '账号至少 3 位' }),
  },
  {
    component: 'VbenInputPassword',
    componentProps: { placeholder: '设置至少 6 位登录密码' },
    fieldName: 'adminPassword',
    label: '管理员密码',
    rules: z.string().min(6, { message: '密码至少 6 位' }),
  },
]);

async function handleSubmit(values: Record<string, string>) {
  loading.value = true;
  try {
    await registerOrganizationApi(
      values as Parameters<typeof registerOrganizationApi>[0],
    );
    await router.push('/auth/login?registered=1');
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="sprout-login-content">
    <div class="sprout-login-mini-brand">
      <span class="sprout-login-mini-mark">芽</span>
      <span>豆芽成长助手 · 机构入驻</span>
    </div>
    <AuthenticationRegister
      :form-schema="formSchema"
      :loading="loading"
      login-path="/auth/login"
      submit-button-text="提交注册申请"
      sub-title="使用平台邀请码提交机构注册申请，审核通过后即可登录"
      title="注册托管机构"
      @submit="handleSubmit"
    />
    <p class="mt-4 text-center text-xs text-muted-foreground">
      注册申请仅用于创建机构管理员，不会自动获得平台总管理权限。
    </p>
  </div>
</template>
