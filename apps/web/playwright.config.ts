import { defineConfig } from '@playwright/test';

const routePerfPort = Number(process.env.WINRIFT_ROUTE_PERF_PORT ?? 4173);
const baseURL = process.env.WINRIFT_ROUTE_PERF_BASE_URL ?? `http://127.0.0.1:${routePerfPort}`;
const apiURL = process.env.WINRIFT_ROUTE_PERF_API_URL ?? process.env.VITE_API_URL ?? 'http://127.0.0.1:8000';
const skipServer = process.env.WINRIFT_ROUTE_PERF_SKIP_SERVER === '1' || process.env.WINRIFT_ROUTE_PERF_SKIP_SERVER === 'true';

const webServerEnv: Record<string, string> = {
  VITE_API_URL: process.env.WINRIFT_ROUTE_PERF_CLIENT_API_URL ?? '',
  WINRIFT_ROUTE_PERF_API_URL: apiURL,
  WINRIFT_ROUTE_PERF_PORT: String(routePerfPort),
};
for (const [key, value] of Object.entries(process.env)) {
  if (typeof value === 'string') {
    webServerEnv[key] = value;
  }
}

export default defineConfig({
  testDir: './tests/perf',
  fullyParallel: false,
  workers: 1,
  timeout: Number(process.env.WINRIFT_ROUTE_PERF_TEST_TIMEOUT_MS ?? 60_000),
  expect: {
    timeout: Number(process.env.WINRIFT_ROUTE_PERF_EXPECT_TIMEOUT_MS ?? 20_000),
  },
  reporter: [['list']],
  use: {
    baseURL,
    browserName: 'chromium',
    viewport: { width: 1440, height: 1000 },
    trace: 'retain-on-failure',
  },
  webServer: skipServer
    ? undefined
    : {
        command: 'npm run build && node tests/perf/route-perf-server.mjs',
        url: baseURL,
        reuseExistingServer: process.env.WINRIFT_ROUTE_PERF_REUSE_SERVER === '1' || process.env.WINRIFT_ROUTE_PERF_REUSE_SERVER === 'true',
        timeout: Number(process.env.WINRIFT_ROUTE_PERF_SERVER_TIMEOUT_MS ?? 120_000),
        env: webServerEnv,
      },
});
