/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.tsx'],
    reporters: ['default', 'junit'],
    outputFile: { junit: '../../.local/test-results/vitest-dashboard.xml' }
  }
});
