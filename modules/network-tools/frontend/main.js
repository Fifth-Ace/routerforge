import './module.css';
import './policy-picker.css';
import { mount } from 'svelte';
import NetworkToolsApp from './NetworkToolsApp.svelte';
mount(NetworkToolsApp, { target: document.getElementById('app') });
