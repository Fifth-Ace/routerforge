import '@fontsource/inter/400.css';
import '@fontsource/inter/500.css';
import '@fontsource/inter/600.css';
import '@fontsource/inter/700.css';
import '@fontsource/roboto-mono/400.css';
import '@fontsource/roboto-mono/500.css';
import '@fontsource/roboto-mono/600.css';
import '../src/app.css';
import '../src/admin-shell.css';
import '../src/routerforge-beta.css';
import './module.css';
import { mount } from 'svelte';
import DNSModuleApp from './DNSModuleApp.svelte';

mount(DNSModuleApp, { target: document.getElementById('app') });
