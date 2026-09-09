/// <reference types="vitest/config" />
import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
  server: {
    proxy: {
      // Default targets the Docker service name (compose dev); override for
      // bare-metal dev with VITE_API_PROXY_TARGET=http://localhost:8080.
      '/api': {
        target: process.env.VITE_API_PROXY_TARGET ?? 'http://api:8080',
        changeOrigin: true,
      },
    },
  },
})
