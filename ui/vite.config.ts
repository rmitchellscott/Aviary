import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';
import fs from 'fs';

const serviceWorkerPlugin = () => {
  const buildTime = Date.now();
  return {
    name: 'service-worker-plugin',
    writeBundle() {
      const swPath = path.resolve(__dirname, 'dist/sw.js');
      if (fs.existsSync(swPath)) {
        let content = fs.readFileSync(swPath, 'utf-8');
        content = content.replace('__BUILD_TIME__', buildTime.toString());
        fs.writeFileSync(swPath, content);
      }
    }
  };
};

export default defineConfig({
  plugins: [react(), serviceWorkerPlugin()],
  build: {
    outDir: 'dist',
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
      '@locales': path.resolve(__dirname, '../locales'),
    },
  },
});
