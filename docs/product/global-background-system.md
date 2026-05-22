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
- root `.app-shell.background-*` contrast profiles

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

Broad pages use the full art pool:

- Home
- Champions index
- Tier list
- Broad build/stat pages that are not about one specific champion or summoner

Specific champion guide pages pass `championScopeId`. When that prop is set, the background pool is filtered to that champion's base splash and skins. If the skin manifest is unavailable, the component falls back to that champion's base splash.

Summoner profile pages pass `championScopeIds` after the stored profile loads. That pool is built from recent stored matches first, then top champion comfort rows. The result is still atmospheric and dimmed, but it feels tied to the summoner rather than random global art.

Live match pages use the global art system too, but keep the dense contrast profile because the card grid and middle analytics row are high-information surfaces. If we later scope them, the most natural pool is the ten live participants plus any focused player/opponent pair in Builds mode.

This gives champion pages a stronger identity without needing separate page-specific background systems.

## User-Facing Rules

The background is allowed to create product mood, but it should never compete with the search console, live-game cards, or analytics tables.

- Search controls, buttons, inputs, and card content must render above the dimming layers, not inherit the decorative opacity.
- Dense pages should prefer `background-dense` or `background-data`.
- Champion pages can be more characterful because their data is scoped to one champion.
- Profile pages should use the player's recently played champions when available, then fall back to global art.
- The topbar should stay visually attached to the app shell; avoid extra full-width bars that break the background composition.

## Contrast Profiles

The root app shell adds page-aware classes, and CSS variables tune the same background component for each surface.

Current classes:

- `background-showcase`: home/search page. Brighter art, stronger map-line accents, and less center dimming because the main UI is a single focused console.
- `background-directory`: champion index. Moderate dimming so the grid stays readable while still showing the broad art pool.
- `background-champion-scope`: specific champion guide. Uses that champion's art pool with a stronger vignette so guide panels and stat cards remain the first read.
- `background-data`: tier list and broad analytics pages. Dimmer art, lower line opacity, and heavier edge/vertical vignette.
- `background-dense`: summoner/profile/live surfaces. The most restrained profile because live match cards and numbers are dense.

The shared variables are:

- `--global-art-active-opacity`
- `--global-art-img-opacity`
- `--global-art-brightness`
- `--global-art-saturate`
- `--global-art-line-opacity`
- `--global-art-stage-top`
- `--global-art-stage-bottom`
- `--global-art-vignette-*`

New page types should pick one of the existing profiles first. Add a new profile only when a page has a clearly different reading density.

## Motion Rules

The backdrop should feel atmospheric, not busy.

- Slides are shuffled into a deck so the first image is not always the same.
- Each slide gets a deterministic pan direction from a small set of CSS pan classes.
- The current slide fades out while the next slide fades in.
- Movement is intentionally slow and subtle to avoid fighting dense UI panels.
- The end of a slide should not snap before the fade. If it starts to feel jerky, tune the keyframes or duration in CSS before adding more React state.

The component renders only the active and previous slides during a transition. That keeps DOM size small even when the splash manifest contains every champion skin.

## Deployment Policy

Default deployment should keep using Data Dragon CDN URLs. That avoids repo bloat and avoids storing thousands of large binary files on the server.

If CDN latency or availability becomes a real issue later, add an optional external asset cache outside git, for example:

- `/srv/winrift/assets/ddragon/splash/...`
- a NAS-backed mounted volume
- an object-storage bucket

Do not commit Riot splash art into the repository.

For a home-server deployment, the best future cache shape is a mounted directory or NAS-backed read-through cache. The app should still treat cached art as disposable generated/runtime data, not source code.

## Future Tuning

If a page feels too loud or too muddy, adjust its `.background-*` variables before touching the slideshow logic. The animation, shuffle deck, and champion scoping should remain shared.

Open tuning items:

- Recheck contrast on the live page after the mode system settles.
- Add reduced-motion handling if the background bothers users who prefer less animation.
- Consider preloading the next slide once the app has been idle long enough to avoid stealing bandwidth from API requests.
