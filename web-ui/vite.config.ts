import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';

// https://vite.dev/config/
export default defineConfig({
  plugins: [preact()],
  build: {
    target: 'es2015',
    minify: 'terser',
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'preact-vendor': ['preact'],
        },
      },
    },
  },
  server: {
    port: 3000,
    host: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
        ws: true,
        configure: (proxy, options) => {
          // Handle WebSocket proxy errors
          proxy.on('error', (err, req, res) => {
            console.log('WebSocket proxy error:', err);
          });
          
          // Handle WebSocket close events
          proxy.on('close', (res, socket, head) => {
            console.log('WebSocket proxy connection closed');
          });
          
          // Handle proxy timeout
          proxy.timeout = 30000; // 30 seconds
        },
      },
    },
  },
  preview: {
    port: 3000,
    host: true,
  },
});