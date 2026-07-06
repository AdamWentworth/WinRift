import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { expect, test } from '@playwright/test';

const outputRoot = resolve(process.env.WINRIFT_DEMO_MEDIA_OUT ?? '.artifacts/demo-media/winrift');
const screenshotsDir = resolve(outputRoot, 'screenshots');
const videosDir = resolve(outputRoot, 'videos');
const manifestPath = resolve(outputRoot, 'manifest.json');

const viewports = [
  { key: 'desktop', width: 1440, height: 1000, isMobile: false },
  { key: 'mobile', width: 390, height: 844, isMobile: true },
];

const captured = {
  generatedAt: new Date().toISOString(),
  screenshots: [],
  videos: [],
};

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
  rmSync(screenshotsDir, { recursive: true, force: true });
  rmSync(videosDir, { recursive: true, force: true });
  mkdirSync(screenshotsDir, { recursive: true });
  mkdirSync(videosDir, { recursive: true });
});

test('captures WinRift feature screenshots', async ({ browser }) => {
  for (const viewport of viewports) {
    const context = await browser.newContext({
      viewport: { width: viewport.width, height: viewport.height },
      deviceScaleFactor: 1,
      isMobile: viewport.isMobile,
      hasTouch: viewport.isMobile,
    });
    const page = await context.newPage();
    await installPointerOverlay(page, viewport);

    await captureScreenshot(page, viewport, 'home', async () => {
      await gotoAndSettle(page, '/');
      await expect(page.getByText('Guides, Profiles, Live Games')).toBeVisible();
    });

    await captureScreenshot(page, viewport, 'champion-directory', async () => {
      await gotoAndSettle(page, '/champions');
      await expect(page.getByText('Champion Index')).toBeVisible();
      await expect(page.locator('.champion-directory-card').first()).toBeVisible();
    });

    await captureScreenshot(page, viewport, 'champion-guide', async () => {
      await gotoAndSettle(page, '/champions/Aatrox');
      await expect(page.getByText('WinRift Build Atlas')).toBeVisible();
      await expect(page.getByText('Build Advice Source')).toBeVisible();
    });

    await captureScreenshot(page, viewport, 'tier-list', async () => {
      await gotoAndSettle(page, '/tier-list');
      await expect(page.getByText('WinRift Tier List')).toBeVisible();
      await expect(page.locator('.tier-table-row').first()).toBeVisible();
    });

    await captureScreenshot(page, viewport, 'summoner-profile', async () => {
      await gotoAndSettle(page, '/summoners/NA1/Meta%20Scout/NA1');
      await expect(page.getByText('Stored Match Form')).toBeVisible();
      await expect(page.getByText('Champion Highlights')).toBeVisible();
    });

    await captureScreenshot(page, viewport, 'live-match', async () => {
      await openLiveMatch(page, viewport);
      await expect(page.getByText('Match Overview')).toBeVisible();
    });

    await captureScreenshot(page, viewport, 'live-builds', async () => {
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Show Builds mode' }));
      await expect(page.getByLabel('Focused build matchup')).toBeVisible();
      await expect(page.getByText('Actual build paths').first()).toBeVisible();
    });

    await captureScreenshot(page, viewport, 'win-conditions', async () => {
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Show Win Conditions mode' }));
      await expect(page.getByText("Your Team's Win Condition")).toBeVisible();
      await expect(page.getByText('Winrate By Game Length')).toBeVisible();
    });

    await context.close();
  }
});

test('records WinRift demo videos', async ({ browser }) => {
  for (const viewport of viewports) {
    await recordVideo(browser, viewport, 'champion-discovery', async (page) => {
      await gotoAndSettle(page, '/');
      await expect(page.getByText('Guides, Profiles, Live Games')).toBeVisible();
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Champions' }), 450);
      await expect(page.getByText('Champion Index')).toBeVisible();
      await fillWithPointer(page, viewport, page.getByPlaceholder('Search champions'), 'Kai', 450);
      await clickWithPointer(page, viewport, page.getByRole('button', { name: /Kai'Sa/i }).first(), 450);
      await expect(page.getByText('WinRift Build Atlas')).toBeVisible();
      await clickWithPointer(page, viewport, page.getByRole('button', { name: /Bot/i }).first(), 650);
      await selectWithPointer(page, viewport, page.locator('.guide-select-control.matchup select'), 'Jinx', 800);
      await smoothScrollTo(page, page.locator('.rune-guide-card'), 1300);
      await smoothScrollTo(page, page.locator('.guide-item-grid'), 1500);
      await smoothScrollTo(page, page.locator('.guide-matchups-section'), 1300);
    });

    await recordVideo(browser, viewport, 'tier-list', async (page) => {
      await gotoAndSettle(page, '/tier-list');
      await expect(page.getByText('WinRift Tier List')).toBeVisible();
      await clickWithPointer(page, viewport, page.getByRole('button', { name: /Jungle/i }).first(), 650);
      await selectWithPointer(page, viewport, page.locator('.tier-filter-bar label').filter({ hasText: 'Sort' }).locator('select'), 'Win Rate', 650);
      await fillWithPointer(page, viewport, page.getByPlaceholder('Search champions in this tier list'), 'Lee', 600);
      await clickWithPointer(page, viewport, page.locator('.tier-table-row').filter({ hasText: 'Lee Sin' }).first(), 650);
      await expect(page.getByText('WinRift Build Atlas')).toBeVisible();
      await page.waitForTimeout(1400);
    });

    await recordVideo(browser, viewport, 'summoner-profile', async (page) => {
      await gotoAndSettle(page, '/summoners');
      await expect(page.getByText('Profiles and Stored Ranked Ladder')).toBeVisible();
      await clickWithPointer(page, viewport, page.locator('.summoner-leaderboard-row').filter({ hasText: 'Meta Scout' }).first(), 800);
      await expect(page.getByText('Stored Match Form')).toBeVisible();
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Champion Stats' }), 850);
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Builds' }), 850);
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Match History' }), 850);
      await page.waitForTimeout(1400);
    });

    await recordVideo(browser, viewport, 'live-match-analysis', async (page) => {
      await gotoAndSettle(page, '/');
      await expect(page.getByText('Guides, Profiles, Live Games')).toBeVisible();
      await fillWithPointer(page, viewport, page.getByPlaceholder('Champion or Riot ID'), 'Rift Pilot#NA1', 450);
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Search WinRift' }), 650);
      await expect(page.getByText('Match Overview')).toBeVisible();
      await page.waitForTimeout(900);
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Show Builds mode' }), 800);
      await expect(page.getByLabel('Focused build matchup')).toBeVisible();
      await clickWithPointer(page, viewport, page.getByLabel('Against Wind Check'), 800);
      await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Show Win Conditions mode' }), 800);
      await expect(page.getByText("Your Team's Win Condition")).toBeVisible();
      await page.waitForTimeout(1400);
    });
  }
});

test.afterAll(() => {
  mkdirSync(dirname(manifestPath), { recursive: true });
  writeFileSync(manifestPath, `${JSON.stringify(captured, null, 2)}\n`);
});

async function captureScreenshot(page, viewport, name, prepare) {
  await prepare();
  await waitForVisualReady(page);
  const fileName = `winrift-${name}-${viewport.key}.png`;
  const path = resolve(screenshotsDir, fileName);
  await page.screenshot({ path, fullPage: false });
  captured.screenshots.push({
    name,
    viewport: viewport.key,
    path: relativeArtifactPath(path),
  });
}

async function recordVideo(browser, viewport, name, flow) {
  const context = await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    deviceScaleFactor: 1,
    isMobile: viewport.isMobile,
    hasTouch: viewport.isMobile,
    recordVideo: {
      dir: videosDir,
      size: { width: viewport.width, height: viewport.height },
    },
  });
  const page = await context.newPage();
  await installPointerOverlay(page, viewport);
  await flow(page);
  await waitForVisualReady(page);
  const video = page.video();
  await page.close();
  await context.close();

  const fileName = `winrift-${name}-${viewport.key}.webm`;
  const path = resolve(videosDir, fileName);
  const temporaryPath = await video?.path().catch(() => undefined);
  await video?.saveAs(path);
  if (temporaryPath && temporaryPath !== path) {
    rmSync(temporaryPath, { force: true });
  }
  captured.videos.push({
    name,
    viewport: viewport.key,
    path: relativeArtifactPath(path),
  });
}

async function openLiveMatch(page, viewport) {
  await gotoAndSettle(page, '/');
  await expect(page.getByText('Guides, Profiles, Live Games')).toBeVisible();
  await fillWithPointer(page, viewport, page.getByPlaceholder('Champion or Riot ID'), 'Rift Pilot#NA1', 250);
  await clickWithPointer(page, viewport, page.getByRole('button', { name: 'Search WinRift' }), 250);
  await expect(page.getByText('Match Overview')).toBeVisible();
  await waitForVisualReady(page);
}

async function gotoAndSettle(page, path) {
  await page.goto(path, { waitUntil: 'domcontentloaded' });
  await waitForVisualReady(page);
}

async function waitForVisualReady(page) {
  await page.waitForLoadState('networkidle', { timeout: 8_000 }).catch(() => undefined);
  await page.evaluate(() => document.fonts?.ready).catch(() => undefined);
  await page.waitForFunction(() => {
    const images = [...document.images].filter((image) => {
      const rect = image.getBoundingClientRect();
      return rect.width > 8
        && rect.height > 8
        && rect.bottom > 0
        && rect.right > 0
        && rect.top < window.innerHeight
        && rect.left < window.innerWidth;
    });
    return images.every((image) => image.complete);
  }, { timeout: 12_000 }).catch(() => undefined);
  await page.waitForTimeout(300);
}

async function installPointerOverlay(page, viewport) {
  const css = `
    .demo-pointer-dot {
      position: fixed;
      left: 0;
      top: 0;
      z-index: 2147483647;
      pointer-events: none;
      transform: translate(-999px, -999px);
      transition: transform 240ms cubic-bezier(.2,.8,.2,1), opacity 160ms ease;
      opacity: 0;
    }
    .demo-pointer-dot.desktop {
      width: 24px;
      height: 28px;
      filter: drop-shadow(0 2px 2px rgba(0,0,0,0.72));
    }
    .demo-pointer-dot.desktop svg {
      display: block;
      width: 24px;
      height: 28px;
    }
    .demo-pointer-dot.mobile {
      width: 34px;
      height: 34px;
      border: 2px solid rgba(255,255,255,0.95);
      border-radius: 999px;
      background: rgba(45, 212, 191, 0.22);
      box-shadow: 0 0 0 4px rgba(15,23,42,0.32), 0 8px 24px rgba(0,0,0,0.28);
    }
    .demo-pointer-dot.active {
      opacity: 1;
    }
    .demo-pointer-dot.mobile.tap {
      width: 54px;
      height: 54px;
      background: rgba(45, 212, 191, 0.18);
    }
    .demo-pointer-dot.desktop.tap::after {
      content: "";
      position: absolute;
      left: 1px;
      top: 1px;
      width: 16px;
      height: 16px;
      border: 2px solid rgba(255,255,255,0.72);
      border-radius: 999px;
      animation: demo-cursor-pulse 360ms ease-out;
    }
    @keyframes demo-cursor-pulse {
      from {
        opacity: 0.95;
        transform: scale(0.45);
      }
      to {
        opacity: 0;
        transform: scale(1.7);
      }
    }
  `;
  await page.evaluate(({ css, isMobile }) => {
    if (!document.getElementById('demo-pointer-style')) {
      const style = document.createElement('style');
      style.id = 'demo-pointer-style';
      style.textContent = css;
      document.head.append(style);
    }
    let dot = document.querySelector('.demo-pointer-dot');
    if (!dot) {
      dot = document.createElement('div');
      dot.setAttribute('aria-hidden', 'true');
      document.body.append(dot);
    }
    dot.className = `demo-pointer-dot ${isMobile ? 'mobile' : 'desktop'}`;
    if (!isMobile && !dot.innerHTML.trim()) {
      dot.innerHTML = `
        <svg viewBox="0 0 24 28" aria-hidden="true">
          <path d="M3 2.75v19.2l5.58-5.23 3 7.17 3.18-1.34-2.88-6.9h7.87L3 2.75Z" fill="#fff" stroke="#050505" stroke-width="1.65" stroke-linejoin="round"/>
          <path d="M5.1 7.52v9.47l3.98-3.73 3.25 7.75" fill="none" stroke="#fff" stroke-width="1.05" stroke-linecap="round" stroke-linejoin="round" opacity="0.65"/>
        </svg>
      `;
    }
    if (isMobile) {
      dot.innerHTML = '';
    }
  }, { css, isMobile: viewport.isMobile }).catch(() => undefined);
}

async function clickWithPointer(page, viewport, locator, pauseMs = 500) {
  const target = locator.first();
  await target.scrollIntoViewIfNeeded({ timeout: 10_000 }).catch(() => undefined);
  await expect(target).toBeVisible();
  const point = await centerPoint(target);
  await showPointer(page, viewport, point.x, point.y);
  if (!viewport.isMobile) {
    await page.mouse.move(point.x, point.y, { steps: 8 });
  }
  await target.click({ timeout: 10_000 });
  await pulsePointer(page, viewport, point.x, point.y);
  await waitForVisualReady(page);
  await page.waitForTimeout(pauseMs);
}

async function fillWithPointer(page, viewport, locator, value, pauseMs = 500) {
  const target = locator.first();
  await target.scrollIntoViewIfNeeded({ timeout: 10_000 }).catch(() => undefined);
  await expect(target).toBeVisible();
  const point = await centerPoint(target);
  await showPointer(page, viewport, point.x, point.y);
  if (!viewport.isMobile) {
    await page.mouse.move(point.x, point.y, { steps: 8 });
  }
  await target.click({ timeout: 10_000 });
  await target.fill(value);
  await pulsePointer(page, viewport, point.x, point.y);
  await waitForVisualReady(page);
  await page.waitForTimeout(pauseMs);
}

async function selectWithPointer(page, viewport, locator, label, pauseMs = 500) {
  const target = locator.first();
  await target.scrollIntoViewIfNeeded({ timeout: 10_000 }).catch(() => undefined);
  await expect(target).toBeVisible();
  const point = await centerPoint(target);
  await showPointer(page, viewport, point.x, point.y);
  await target.selectOption({ label }, { timeout: 10_000 });
  await pulsePointer(page, viewport, point.x, point.y);
  await waitForVisualReady(page);
  await page.waitForTimeout(pauseMs);
}

async function smoothScrollTo(page, locator, pauseMs = 900) {
  const target = locator.first();
  await expect(target).toBeVisible();
  await target.evaluate((element) => {
    element.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'nearest' });
  });
  await page.waitForTimeout(pauseMs);
  await waitForVisualReady(page);
}

async function centerPoint(locator) {
  const box = await locator.boundingBox();
  if (!box) {
    throw new Error('Could not measure locator for demo pointer.');
  }
  return {
    x: box.x + box.width / 2,
    y: box.y + box.height / 2,
  };
}

async function showPointer(page, viewport, x, y) {
  await installPointerOverlay(page, viewport);
  await page.evaluate(({ x, y, isMobile }) => {
    const dot = document.querySelector('.demo-pointer-dot');
    if (!dot) return;
    dot.classList.toggle('tap', isMobile);
    dot.classList.add('active');
    const left = isMobile ? x - dot.clientWidth / 2 : x - 2;
    const top = isMobile ? y - dot.clientHeight / 2 : y - 2;
    dot.style.transform = `translate(${left}px, ${top}px)`;
  }, { x, y, isMobile: viewport.isMobile });
  await page.waitForTimeout(170);
}

async function pulsePointer(page, viewport, x, y) {
  await installPointerOverlay(page, viewport);
  await page.evaluate(({ x, y, isMobile }) => {
    const dot = document.querySelector('.demo-pointer-dot');
    if (!dot) return;
    dot.classList.remove('tap');
    void dot.getBoundingClientRect();
    dot.classList.add('tap');
    dot.classList.add('active');
    const left = isMobile ? x - dot.clientWidth / 2 : x - 2;
    const top = isMobile ? y - dot.clientHeight / 2 : y - 2;
    dot.style.transform = `translate(${left}px, ${top}px)`;
    window.setTimeout(() => {
      dot.classList.toggle('tap', isMobile);
    }, 180);
  }, { x, y, isMobile: viewport.isMobile });
  await page.waitForTimeout(220);
}

function relativeArtifactPath(path) {
  return path.replace(`${process.cwd()}/`, '');
}
