import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'line' : 'list',
  use: {
    baseURL: 'http://127.0.0.1:4173',
    ...devices['Desktop Chrome'],
  },
  webServer: {
    command: 'npx vite preview --outDir ../server/internal/router/webui --host 127.0.0.1 --port 4173 --strictPort',
    url: 'http://127.0.0.1:4173/app/',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
})
