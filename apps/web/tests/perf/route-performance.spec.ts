import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { performance } from 'node:perf_hooks';
import { expect, test, type Page, type Request, type Response } from '@playwright/test';

type RouteCheck = {
  name: string;
  path: string;
  ready: (page: Page) => Promise<void>;
  maxReadyMs: number;
  maxApiRequests: number;
  minApiRequests: number;
};

type ApiRequestTiming = {
  path: string;
  method: string;
  status: number | null;
  durationMs: number | null;
  failedText?: string;
};

type RouteTimingResult = {
  name: string;
  path: string;
  readyMs: number;
  domContentLoadedMs: number | null;
  loadMs: number | null;
  apiRequests: ApiRequestTiming[];
  maxReadyMs: number;
  maxApiRequests: number;
  warnings: string[];
};

const strict = process.env.WINRIFT_ROUTE_PERF_STRICT === '1' || process.env.WINRIFT_ROUTE_PERF_STRICT === 'true';
const jsonPath = process.env.WINRIFT_ROUTE_PERF_JSON ?? 'test-results/route-performance.json';
const results: RouteTimingResult[] = [];

const routeChecks: RouteCheck[] = [
  {
    name: 'Home search',
    path: '/',
    ready: async (page) => {
      await expect(page.getByText('Guides, Profiles, Live Games')).toBeVisible();
      await expect(page.getByPlaceholder('Champion or Riot ID')).toBeVisible();
    },
    maxReadyMs: 2_500,
    maxApiRequests: 3,
    minApiRequests: 2,
  },
  {
    name: 'Champion directory',
    path: '/champions',
    ready: async (page) => {
      await expect(page.getByText('Champion Index')).toBeVisible();
      await expect(page.getByText('All Champions')).toBeVisible();
      await expect(page.getByText(/\d+ champions shown/)).toBeVisible();
    },
    maxReadyMs: 3_000,
    maxApiRequests: 3,
    minApiRequests: 2,
  },
  {
    name: 'Aatrox guide',
    path: '/champions/Aatrox',
    ready: async (page) => {
      await expect(page.getByText('WinRift Build Atlas')).toBeVisible();
      await expect(page.getByText('Build Advice Source')).toBeVisible();
      await expect(page.getByText('Loading items...')).toHaveCount(0);
    },
    maxReadyMs: 4_500,
    maxApiRequests: 7,
    minApiRequests: 4,
  },
  {
    name: 'Tier list',
    path: '/tier-list',
    ready: async (page) => {
      await expect(page.getByText('WinRift Tier List')).toBeVisible();
      await expect(page.getByText('All Roles Rankings')).toBeVisible();
    },
    maxReadyMs: 3_500,
    maxApiRequests: 3,
    minApiRequests: 2,
  },
  {
    name: 'Summoners hub',
    path: '/summoners',
    ready: async (page) => {
      await expect(page.getByText('Profiles and Stored Ranked Ladder')).toBeVisible();
      await expect(page.getByText('Stored Ranked Ladder', { exact: true })).toBeVisible();
      await expect(page.locator('.summoner-leaderboard-row').first()).toBeVisible();
    },
    maxReadyMs: 3_500,
    maxApiRequests: 3,
    minApiRequests: 3,
  },
];

test.describe.configure({ mode: 'serial' });

for (const route of routeChecks) {
  test(`route timing: ${route.name}`, async ({ page }) => {
    const apiRequestsByRequest = new Map<Request, ApiRequestTiming>();
    const apiStartedAt = new Map<Request, number>();
    const isApiRequest = (url: string) => new URL(url).pathname.startsWith('/api/');
    const onRequest = (request: Request) => {
      if (!isApiRequest(request.url())) {
        return;
      }
      apiStartedAt.set(request, performance.now());
      const url = new URL(request.url());
      apiRequestsByRequest.set(request, {
        path: `${url.pathname}${url.search}`,
        method: request.method(),
        status: null,
        durationMs: null,
      });
    };
    const onResponse = (response: Response) => {
      if (!isApiRequest(response.url())) {
        return;
      }
      const request = response.request();
      const url = new URL(response.url());
      const startedAt = apiStartedAt.get(request);
      apiRequestsByRequest.set(request, {
        path: `${url.pathname}${url.search}`,
        method: request.method(),
        status: response.status(),
        durationMs: typeof startedAt === 'number' ? Math.round(performance.now() - startedAt) : null,
      });
    };
    const onRequestFailed = (request: Request) => {
      if (!isApiRequest(request.url())) {
        return;
      }
      const url = new URL(request.url());
      const startedAt = apiStartedAt.get(request);
      apiRequestsByRequest.set(request, {
        path: `${url.pathname}${url.search}`,
        method: request.method(),
        status: null,
        durationMs: typeof startedAt === 'number' ? Math.round(performance.now() - startedAt) : null,
        failedText: request.failure()?.errorText ?? 'request failed',
      });
    };

    page.on('request', onRequest);
    page.on('response', onResponse);
    page.on('requestfailed', onRequestFailed);
    const startedAt = performance.now();
    await page.goto(route.path, { waitUntil: 'domcontentloaded' });
    await route.ready(page);
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => undefined);
    const readyMs = Math.round(performance.now() - startedAt);
    const apiRequests = [...apiRequestsByRequest.values()];
    page.off('request', onRequest);
    page.off('response', onResponse);
    page.off('requestfailed', onRequestFailed);

    const nav = await page.evaluate(() => {
      const entry = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined;
      if (!entry) {
        return { domContentLoadedMs: null, loadMs: null };
      }
      return {
        domContentLoadedMs: Math.round(entry.domContentLoadedEventEnd),
        loadMs: Math.round(entry.loadEventEnd),
      };
    });

    const warnings = warningsFor(route, readyMs, apiRequests);
    const result: RouteTimingResult = {
      name: route.name,
      path: route.path,
      readyMs,
      domContentLoadedMs: nav.domContentLoadedMs,
      loadMs: nav.loadMs,
      apiRequests,
      maxReadyMs: route.maxReadyMs,
      maxApiRequests: route.maxApiRequests,
      warnings,
    };
    results.push(result);

    const apiSummary = apiRequests.map((request) => `${request.method} ${request.status ?? 'ERR'}:${request.durationMs ?? '?'}ms ${request.path}${request.failedText ? ` ${request.failedText}` : ''}`).join(' | ');
    console.log(`${route.name}: ready=${readyMs}ms dcl=${nav.domContentLoadedMs ?? '?'}ms load=${nav.loadMs ?? '?'}ms api=${apiRequests.length}`);
    if (apiSummary) {
      console.log(`  API ${apiSummary}`);
    }
    for (const warning of warnings) {
      console.warn(`  WARN ${warning}`);
    }

    const failedStatuses = apiRequests.filter((request) => (request.status === null && request.failedText !== 'net::ERR_ABORTED') || (request.status ?? 0) >= 400);
    expect(failedStatuses, 'route API requests should not fail').toEqual([]);
    if (strict) {
      expect(readyMs, `${route.name} exceeded route ready budget`).toBeLessThanOrEqual(route.maxReadyMs);
      expect(apiRequests.length, `${route.name} made fewer API requests than expected`).toBeGreaterThanOrEqual(route.minApiRequests);
      expect(apiRequests.length, `${route.name} made too many API requests`).toBeLessThanOrEqual(route.maxApiRequests);
    }
  });
}

test.afterAll(() => {
  mkdirSync(dirname(jsonPath), { recursive: true });
  writeFileSync(jsonPath, `${JSON.stringify({ strict, generatedAt: new Date().toISOString(), results }, null, 2)}\n`);
});

function warningsFor(route: RouteCheck, readyMs: number, apiRequests: ApiRequestTiming[]) {
  const warnings: string[] = [];
  if (readyMs > route.maxReadyMs) {
    warnings.push(`ready ${readyMs}ms exceeded ${route.maxReadyMs}ms`);
  }
  if (apiRequests.length < route.minApiRequests) {
    warnings.push(`API requests ${apiRequests.length} below expected ${route.minApiRequests}`);
  }
  if (apiRequests.length > route.maxApiRequests) {
    warnings.push(`API requests ${apiRequests.length} exceeded ${route.maxApiRequests}`);
  }
  return warnings;
}
