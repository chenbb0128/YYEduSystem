<script lang="ts" setup>
import type { VbenFormSchema } from '@vben/common-ui';

import { computed } from 'vue';

import { AuthenticationLogin, z } from '@vben/common-ui';

import { $t } from '#/locales';
import { useAuthStore } from '#/store';

defineOptions({ name: 'Login' });

const authStore = useAuthStore();

const formSchema = computed((): VbenFormSchema[] => [
  {
    component: 'VbenInput',
    componentProps: {
      placeholder: $t('authentication.usernameTip'),
    },
    defaultValue: 'admin',
    fieldName: 'username',
    label: $t('authentication.username'),
    rules: z.string().min(1, { message: $t('authentication.usernameTip') }),
  },
  {
    component: 'VbenInputPassword',
    componentProps: {
      placeholder: $t('authentication.password'),
    },
    defaultValue: '123456',
    fieldName: 'password',
    label: $t('authentication.password'),
    rules: z.string().min(1, { message: $t('authentication.passwordTip') }),
  },
]);
</script>

<template>
  <div class="sprout-login-content">
    <div class="sprout-login-mini-brand">
      <span class="sprout-login-mini-mark">芽</span>
      <span>每天都看见孩子的进步</span>
    </div>
    <AuthenticationLogin
      :form-schema="formSchema"
      :loading="authStore.loginLoading"
      :show-code-login="false"
      :show-forget-password="false"
      :show-qrcode-login="false"
      register-path="/auth/register"
      :show-register="true"
      :show-third-party-login="false"
      :sub-title="$t('page.auth.loginSubtitle')"
      :title="$t('page.auth.loginTitle')"
      @submit="authStore.authLogin"
    />
  </div>
</template>
