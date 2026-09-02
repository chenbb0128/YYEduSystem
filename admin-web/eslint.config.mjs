import { defineConfig } from '@vben/eslint-config';

export default defineConfig([
  {
    files: ['**/*.vue'],
    rules: {
      // Oxfmt is the canonical Vue formatter in this workspace; these two
      // ESLint layout rules otherwise fight its output on multiline tags.
      'vue/html-closing-bracket-newline': 'off',
      'vue/multiline-html-element-content-newline': 'off',
    },
  },
]);
