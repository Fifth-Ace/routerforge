import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../../..');

export default defineConfig({
  root: here,
  base: './',
  plugins: [svelte()],
  server: {
    fs: {
      allow: [repoRoot]
    }
  },
  build: {
    outDir: path.join(here, 'build'),
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      input: path.join(here, 'index.html')
    }
  }
});
