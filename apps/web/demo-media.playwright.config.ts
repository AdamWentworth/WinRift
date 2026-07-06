import { defineConfig } from '@playwright/test';

const demoPort = Number(process.env.WINRIFT_DEMO_MEDIA_PORT ?? 4184);
const baseURL = process.env.WINRIFT_DEMO_MEDIA_BASE_URL ?? `http://127.0.0.1:${demoPort}`;
const skipServer = process.env.WINRIFT_DEMO_MEDIA_SKIP_SERVER === '1'
  || process.env.WINRIFT_DEMO_MEDIA_SKIP_SERVER === 'true';

const webServerEnv: Record<string, string> = {
  VITE_API_URL: process.env.WINRIFT_DEMO_MEDIA_CLIENT_API_URL ?? '',
  WINRIFT_DEMO_MEDIA_PORT: String(demoPort),
};

for (const [key, value] of Object.entries(process.env)) {
  if (typeof value === 'string') {
    webServerEnv[key] = value;
  }
}

export default defineConfig({
  testDir: './tests/demo-media',
  testMatch: '**/*.playwright.mjs',
  fullyParallel: false,
  workers: 1,
  timeout: Number(process.env.WINRIFT_DEMO_MEDIA_TEST_TIMEOUT_MS ?? 180_000),
  expect: {
    timeout: Number(process.env.WINRIFT_DEMO_MEDIA_EXPECT_TIMEOUT_MS ?? 20_000),
  },
  reporter: [['list']],
  use: {
    baseURL,
    browserName: 'chromium',
    trace: 'retain-on-failure',
  },
  webServer: skipServer
    ? undefined
    : {
        command: 'npm run build && node tests/demo-media/demo-media-server.mjs',
        url: baseURL,
        reuseExistingServer: process.env.WINRIFT_DEMO_MEDIA_REUSE_SERVER === '1'
          || process.env.WINRIFT_DEMO_MEDIA_REUSE_SERVER === 'true',
        timeout: Number(process.env.WINRIFT_DEMO_MEDIA_SERVER_TIMEOUT_MS ?? 120_000),
        env: webServerEnv,
      },
});
