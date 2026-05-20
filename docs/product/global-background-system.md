# Global Background System

WinRift uses one shared animated art backdrop across the app. The goal is to make every page feel like the same product while keeping dense match and analytics panels readable.

## Component

Frontend entrypoint:

- `apps/web/src/components/GlobalBackgroundStage.tsx`

The component is mounted once in `App.tsx`, inside the root `.app-shell`, behind the topbar and page content. It is decorative only and renders with `aria-hidden="true"`.

CSS lives in `apps/web/src/styles/app.css` under:

- `.global-art-stage`
- `.global-art-slide`
- `globalArt*` keyframes

## Image Sources

The app does not commit Riot splash art to git.

Current source flow:

1. The API exposes `GET /api/static/champion-splashes`.
2. The web app loads that manifest with `getChampionSplashes`.
3. `GlobalBackgroundStage` shuffles those URLs client-side.
4. The browser fetches splash art directly from Data Dragon CDN URLs.

Fallback order:

1. Full champion/skin splash manifest from the API.
2. Base champion splash URLs derived from Data Dragon champion metadata.
3. A small hardcoded set of fallback splash URLs, used only while static metadata is unavailable.

## Scoping Rules

General pages use the full art pool:

- Home
- Summoner/profile/live pages
- Champions index
- Tier list

Specific champion guide pages pass `championScopeId`. When that prop is set, the background pool is filtered to that champion's base splash and skins. If the skin manifest is unavailable, the component falls back to that champion's base splash.

This gives champion pages a stronger identity without needing separate page-specific background systems.

## Motion Rules

The backdrop should feel atmospheric, not busy.

- Slides are shuffled into a deck so the first image is not always the same.
- Each slide gets a deterministic pan direction from a small set of CSS pan classes.
- The current slide fades out while the next slide fades in.
- Movement is intentionally slow and subtle to avoid fighting dense UI panels.

## Deployment Policy

Default deployment should keep using Data Dragon CDN URLs. That avoids repo bloat and avoids storing thousands of large binary files on the server.

If CDN latency or availability becomes a real issue later, add an optional external asset cache outside git, for example:

- `/srv/winrift/assets/ddragon/splash/...`
- a NAS-backed mounted volume
- an object-storage bucket

Do not commit Riot splash art into the repository.

## Future Tuning

Contrast tuning is intentionally a separate task. Different pages may eventually need per-page density classes, for example:

- lighter art on the home page
- dimmer art behind live-match cards
- champion-scoped art with stronger vignette on guide pages

Those adjustments should happen through root page classes or props, not by creating a second background component.
