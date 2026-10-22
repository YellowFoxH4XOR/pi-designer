import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';

export default defineConfig({
  plugins: [preact()],
  build: { outDir: 'dist', emptyOutDir: true },
  server: {
    port: 5174,
    proxy: {
      '/api': { target: 'http://127.0.0.1:7457', ws: true },
      '/preview': 'http://127.0.0.1:7457',
    },
  },
});
