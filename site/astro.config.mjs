import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://desireco.github.io',
  base: '/zhex',
  outDir: '../_site',
  build: { format: 'file' },
  markdown: { syntaxHighlight: false },
});
