import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  cacheDir: '../../.vite-cache/web',
  plugins: [react()],
  test: {
    setupFiles: './src/setupTests.ts',
    environment: 'jsdom',
  },
  server: {
    port: 5173,
  },
});
