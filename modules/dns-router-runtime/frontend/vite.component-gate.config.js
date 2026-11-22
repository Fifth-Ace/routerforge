// Non-shipping component test build. The caller must provide an isolated output.
import {defineConfig} from 'vite';
import {svelte} from '@sveltejs/vite-plugin-svelte';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const here=path.dirname(fileURLToPath(import.meta.url));
const outDir=process.env.RF_RUNTIME_COMPONENT_GATE_OUTDIR;
if(!outDir||!path.isAbsolute(outDir))throw Error('COMPONENT_GATE_ISOLATED_OUTDIR_REQUIRED');
if(path.resolve(outDir).startsWith(path.resolve(here)+path.sep))
  throw Error('COMPONENT_GATE_OUTDIR_MUST_BE_OUTSIDE_FRONTEND');
export default defineConfig({
  root:here,base:'./',plugins:[svelte()],
  build:{outDir,emptyOutDir:true,sourcemap:false,
    lib:{entry:path.join(here,'component-gate-entry.test.js'),
      name:'RouterForgeRuntimeComponentGate',formats:['iife'],
      fileName:()=> 'component-gate.js'}
  }
});
