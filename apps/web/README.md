# WinRift Web App

`apps/web` is the Vite + React + TypeScript frontend for WinRift. It is built as a single-page app with lightweight route parsing, TanStack Query data fetching, and a League-styled analytics UI.

## Highlights

- Universal homepage lookup for champions and Riot IDs.
- Live match page with mode rail: match overview, focused builds, and win conditions.
- Champion guide pages with runes, stat shards, spells, starting items, item paths, skill paths, matchup cards, and build variants.
- Champion directory and role-aware tier list.
- Summoner profile pages with ranked summary, champion history, recent games, and build usage.
- Global animated splash-art background system with champion-scoped art on guide pages.

## Layout

```plaintext
apps/web/
├── src/
│   ├── api/           # API client and shared response types
│   ├── components/    # Pages and reusable page sections
│   ├── components/live-match/
│   │                  # Live-match mode panels and card components
│   ├── lib/           # Routing, role icons, static-data helpers, tier helpers
│   ├── styles/        # Modular page and component styles
│   └── main.tsx       # React entrypoint
├── index.html
├── Caddyfile           # Production SPA server and same-origin API proxy
├── package.json
├── vite.config.ts
└── Dockerfile
```

## Local Run

```bash
cd apps/web
npm ci
npm run dev
```

Default URL:

- `http://localhost:5173`

By default the local development app calls:

- `http://localhost:8000`

Point local frontend work at a server-hosted API:

```bash
VITE_API_URL=http://SERVER_LAN_IP:8000 npm run dev
```

Or create an ignored local env file:

```bash
cp apps/web/.env.example apps/web/.env.local
editor apps/web/.env.local
```

For a private server setup:

```env
VITE_API_URL=http://SERVER_LAN_IP:8000
```

## Environment

| Key | Purpose |
|-----|---------|
| `VITE_API_URL` | API base URL used by browser requests. |

No Riot key belongs in frontend environment variables. All Riot requests go through the Go API.

Production builds default to same-origin API calls. The Caddy image serves the compiled SPA, proxies `/api/*` to the internal `api:8000` service, and preserves React deep routes through an `index.html` fallback.

## App Routes

| Route | Purpose |
|-------|---------|
| `/` | Universal lookup homepage |
| `/champions` | Alphabetical champion directory |
| `/champions/:champion` | Champion guide page |
| `/tier-list` | Role-aware champion ranking |
| `/win-conditions` | Composition-strategy directory |
| `/teamfight`, `/splitpush`, `/pick`, `/siege`, `/control` | Win-condition detail pages |
| `/flex` | Flexible champion-archetype reference |
| `/summoners` | Summoner lookup shell |
| `/summoners/:platform/:gameName/:tagLine` | Summoner profile, with live-game state when available |

The app uses browser history directly instead of a router package. Route parsing and path generation live in [src/lib/appRouting.ts](src/lib/appRouting.ts); Riot ID path helpers live in [src/lib/lookup.ts](src/lib/lookup.ts).

## API Dependencies

The main API calls are:

- `GET /api/static/champions`
- `GET /api/static/items`
- `GET /api/static/runes`
- `GET /api/static/summoner-spells`
- `GET /api/live-game`
- `GET /api/summoner/profile`
- `GET /api/summoners/leaderboard`
- `GET /api/analytics/champion-page`
- `GET /api/analytics/champion-guides`
- `GET /api/analytics/build-advice`
- `GET /api/analytics/patches`
- `POST /api/analytics/win-conditions`

The frontend expects the backend to serve precomputed analytics. Heavy winrate/build aggregation should not happen in browser code.

## Testing

```bash
cd apps/web
npm test
npm run build
```

Tests are written with Vitest and React Testing Library. Build validation runs TypeScript first, then Vite.

## Demo Media Capture

```bash
cd apps/web
npm run capture:demo:media
```

The capture command builds the app, serves it with deterministic local demo API fixtures, and writes WinRift screenshots plus videos to `.artifacts/demo-media/winrift/`.

Default output:

- 16 screenshots: desktop and mobile for home, champion directory, champion guide, tier list, summoner profile, live match, focused builds, and win conditions.
- 8 videos: desktop and mobile for champion discovery, tier list, summoner profile, and live match analysis.
- `manifest.json` listing the generated media paths.

## UI Notes

- Use the global background stage rather than page-specific decorative backgrounds.
- Champion guide backgrounds should scope splash art to the active champion when possible.
- Live-match modes should keep the same surface width to avoid layout jumps.
- Build advice should distinguish exact matchup samples from champion-wide fallback data.
- Text should stay concise; dense analytics panels should favor clear labels and tables over explanatory paragraphs.

## Safety Notes

- Do not expose API keys in client code.
- Live game views should present contextual stats, not direct real-time commands.
- Riot/Data Dragon image URLs are fetched by the client; large art assets are not committed to the repo.
