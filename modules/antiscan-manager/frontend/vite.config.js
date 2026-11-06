import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
export default defineConfig({base:'./',plugins:[svelte()],build:{target:'es2020',outDir:'build',emptyOutDir:true,sourcemap:false}});
