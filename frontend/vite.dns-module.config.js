import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  root: path.join(here, 'dns-module'),
  base: './',
  plugins: [svelte()],
  build: {
    outDir: path.join(here, 'dns-module-build'),
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      input: path.join(here, 'dns-module', 'index.html')
    }
  }
});
