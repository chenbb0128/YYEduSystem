import type { RouteRecordRaw } from 'vue-router';

import { $t } from '#/locales';

const routes: RouteRecordRaw[] = [
  {
    name: 'PickupOperations',
    path: '/pickup-operations',
    component: () => import('#/views/pickup/index.vue'),
    meta: {
      icon: 'lucide:clipboard-check',
      order: 4,
      title: '今日接送',
    },
  },
  {
    name: 'MasterData',
    path: '/master-data',
    component: () => import('#/views/master-data/index.vue'),
    meta: {
      icon: 'lucide:database',
      order: 5,
      title: '档案中心',
    },
  },
  {
    name: 'Homework',
    path: '/homework',
    component: () => import('#/views/homework/index.vue'),
    meta: {
      authority: ['homework:view'],
      icon: 'lucide:notebook-pen',
      order: 6,
      title: '作业管理',
    },
  },
  {
    name: 'Meals',
    path: '/meals',
    component: () => import('#/views/meals/index.vue'),
    meta: {
      authority: ['meal:view'],
      icon: 'lucide:utensils',
      order: 7,
      title: '餐食管理',
    },
  },
  {
    name: 'DailySummaries',
    path: '/daily-summaries',
    component: () => import('#/views/daily-summaries/index.vue'),
    meta: {
      authority: ['summary:view'],
      icon: 'lucide:notebook-tabs',
      order: 7.5,
      title: '每日总结',
    },
  },
  {
    name: 'TeacherAssignments',
    path: '/teacher-assignments',
    component: () => import('#/views/teacher-assignments/index.vue'),
    meta: {
      authority: ['assignment:view'],
      icon: 'lucide:clipboard-user',
      order: 8,
      title: '教师班级',
    },
  },
  {
    name: 'ChildApplications',
    path: '/child-applications',
    component: () => import('#/views/child-applications/index.vue'),
    meta: {
      icon: 'lucide:user-round-check',
      order: 8.5,
      title: '家长入班申请',
    },
  },
  {
    name: 'LeaveRequests',
    path: '/leave-requests',
    component: () => import('#/views/leave-requests/index.vue'),
    meta: {
      icon: 'lucide:calendar-check-2',
      order: 9,
      title: '请假审核',
    },
  },
  {
    name: 'NotificationDeliveryLogs',
    path: '/notification-delivery-logs',
    component: () => import('#/views/notification-delivery/index.vue'),
    meta: {
      authority: ['notification:view'],
      icon: 'lucide:send-horizontal',
      order: 9.5,
      title: '通知投递',
    },
  },
  {
    name: 'AnomalyOverview',
    path: '/reports/anomalies',
    component: () => import('#/views/reports/anomalies/index.vue'),
    meta: {
      authority: ['dashboard:view'],
      icon: 'lucide:triangle-alert',
      order: 9.7,
      title: '异常看板',
    },
  },
  {
    name: 'AuditLogs',
    path: '/audit-logs',
    component: () => import('#/views/audit/index.vue'),
    meta: {
      authority: ['notification:view'],
      icon: 'lucide:history',
      order: 9.6,
      title: '操作审计',
    },
  },
  {
    name: 'SystemUsers',
    path: '/system/users',
    component: () => import('#/views/system/users/index.vue'),
    meta: {
      authority: ['system:user:create'],
      icon: 'lucide:users',
      order: 10,
      title: $t('page.users.title'),
    },
  },
];

export default routes;
