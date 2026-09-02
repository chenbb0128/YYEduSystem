import { defineConfig, viteCssLayerPlugin } from '@vben/vite-config';

import ElementPlus from 'unplugin-element-plus/vite';

export default defineConfig(async () => {
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
            target: 'http://localhost:8081',
          },
        },
      },
    },
  };
});
