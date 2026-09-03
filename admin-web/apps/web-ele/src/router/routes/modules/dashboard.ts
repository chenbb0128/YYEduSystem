import type { RouteRecordRaw } from 'vue-router';

import { $t } from '#/locales';

const routes: RouteRecordRaw[] = [
  {
    name: 'Dashboard',
    path: '/dashboard',
    component: () => import('#/views/dashboard/index.vue'),
    meta: {
      affixTab: true,
      authority: ['dashboard:view'],
      icon: 'lucide:layout-dashboard',
      order: -1,
      title: $t('page.dashboard.title'),
    },
  },
  {
    name: 'PickupSchedules',
    path: '/pickup-schedules',
    component: () => import('#/views/pickup-schedules/index.vue'),
    meta: {
      authority: ['assignment:view'],
      icon: 'lucide:calendar-clock',
      order: 4.5,
      title: '接送排班',
    },
  },
];

export default routes;
