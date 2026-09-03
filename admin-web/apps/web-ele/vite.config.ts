import { cwd } from 'node:process';

import { defineConfig, viteCssLayerPlugin } from '@vben/vite-config';

import ElementPlus from 'unplugin-element-plus/vite';
import { loadEnv } from 'vite';

export default defineConfig(async ({ mode }) => {
  const env = loadEnv(mode, cwd(), '');
  const tuoguanApiTarget =
    env.VITE_TUOGUAN_PROXY_TARGET || 'http://localhost:8081';

  return {
    application: {
      license: false,
      print: false,
      pwa: false,
      vxeTableLazyImport: false,
    },
    vite: {
      plugins: [
        viteCssLayerPlugin({ layerName: 'el', packageName: 'element-plus' }),
        ElementPlus({ format: 'esm' }),
      ],
      server: {
        proxy: {
          '/api': {
            changeOrigin: true,
            rewrite: (path) => path.replace(/^\/api/, ''),
            target: 'http://localhost:5320/api',
            ws: true,
          },
          '/tuoguan-api': {
            changeOrigin: true,
            rewrite: (path) => path.replace(/^\/tuoguan-api/, ''),
            target: tuoguanApiTarget,
          },
        },
      },
    },
  };
});
