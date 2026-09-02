import { createRequire } from 'node:module'
import path from 'node:path'
import { TDesignResolver } from 'weapp-vite/auto-import-components/resolvers'
import { defineConfig } from 'weapp-vite/config'

const requirePackage = createRequire(import.meta.url)
const { WeappTailwindcss } = requirePackage('weapp-tailwindcss/vite') as typeof import('weapp-tailwindcss/vite')

export default defineConfig({
  weapp: {
    web: {
      vite: {
        server: {
          proxy: {
            '/tuoguan-api': {
              changeOrigin: true,
              rewrite: requestPath => requestPath.replace(/^\/tuoguan-api/, ''),
              target: 'http://localhost:8080',
            },
          },
        },
      },
    },
    srcRoot: 'src',
    typescript: {
      app: {
        compilerOptions: {
          paths: {
            'tdesign-miniprogram/*': ['./node_modules/tdesign-miniprogram/miniprogram_dist/*'],
          },
        },
      },
    },
    autoImportComponents: {
      resolvers: [TDesignResolver()],
    },
    // Generated file format for pnpm g.
    // https://vite.icebreaker.top/guide/generate.html
    generate: {
      extensions: {
        js: 'ts',
        wxss: 'css',
      },
      dirs: {
        component: 'src/components',
        page: 'src/pages',
      },
      // Uncomment to generate components as Example/index instead of Example/Example.
      // filenames: {
      //   component: 'index',
      //   page: 'index',
      // },
    },
  },
  plugins: [
    WeappTailwindcss({
      rem2rpx: true,
      cssEntries: [path.resolve(import.meta.dirname, 'src/app.css')],
    }),
  ],
})
