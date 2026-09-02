const appConfig = Object.freeze({
  defaultAvatar: '/avatar.svg',
  defaultHomePath: '/dashboard',
  logo: '/logo.svg',
  name: import.meta.env.VITE_APP_TITLE || '豆芽成长助手',
  namespace: import.meta.env.VITE_APP_NAMESPACE || 'tuoguan-admin',
});

type AppConfig = typeof appConfig;

export { appConfig, type AppConfig };
