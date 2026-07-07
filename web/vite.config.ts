import path from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 5173,
    proxy: {
      '/chat': 'http://localhost:8082',
      '/tasks': 'http://localhost:8082',
      '/workflows': 'http://localhost:8082',
      '/novels': 'http://localhost:8082',
      '/conversation': 'http://localhost:8082',
      '/outputs': 'http://localhost:8082',
      '/observability': 'http://localhost:8082',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});