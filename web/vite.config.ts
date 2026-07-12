import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [svelte()],
  publicDir: 'static',
  build: {
    target: 'es2022',
    outDir: '../internal/webembed/dist',
    emptyOutDir: true,
  },
  server: {
    host: '127.0.0.1',
  },
});
