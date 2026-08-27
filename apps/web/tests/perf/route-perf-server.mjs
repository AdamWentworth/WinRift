import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { request as httpRequest } from 'node:http';
import { request as httpsRequest } from 'node:https';
import { extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';

const port = Number(process.env.WINRIFT_ROUTE_PERF_PORT ?? 4173);
const host = process.env.WINRIFT_ROUTE_PERF_HOST ?? '127.0.0.1';
const apiBaseURL =
  process.env.WINRIFT_ROUTE_PERF_API_URL ?? 'http://127.0.0.1:8000';
const upstreamBaseURL = new URL(apiBaseURL);
const distDir = fileURLToPath(new URL('../../dist/', import.meta.url));

if (!['http:', 'https:'].includes(upstreamBaseURL.protocol)) {
  throw new Error(
    `Unsupported route perf API protocol: ${upstreamBaseURL.protocol}`,
  );
}

const contentTypes = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.webp': 'image/webp',
};

const server = createServer((req, res) => {
  if (!req.url) {
    res.writeHead(400).end('Missing URL');
    return;
  }
  if (req.url.startsWith('/api/')) {
    proxyRequest(req, res);
    return;
  }
  serveStatic(req.url, res);
});

server.listen(port, host, () => {
  console.log(`WinRift route perf server listening at http://${host}:${port}`);
  console.log(`Proxying /api to ${apiBaseURL}`);
});

function proxyRequest(req, res) {
  const incoming = new URL(req.url ?? '/', 'http://route-perf.invalid');
  const target = new URL(upstreamBaseURL);
  target.pathname = incoming.pathname;
  target.search = incoming.search;
  const request = target.protocol === 'https:' ? httpsRequest : httpRequest;
  const headers = { ...req.headers, host: target.host };
  const upstream = request(
    {
      protocol: target.protocol,
      hostname: target.hostname,
      port: target.port,
      path: `${target.pathname}${target.search}`,
      method: req.method,
      headers,
    },
    (upstreamResponse) => {
      res.writeHead(
        upstreamResponse.statusCode ?? 502,
        upstreamResponse.headers,
      );
      upstreamResponse.pipe(res);
    },
  );

  upstream.on('error', (error) => {
    res.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' });
    res.end(
      JSON.stringify({ detail: `Route perf proxy failed: ${error.message}` }),
    );
  });

  req.pipe(upstream);
}

function serveStatic(requestURL, res) {
  const { pathname } = new URL(requestURL, `http://${host}:${port}`);
  const safePath = normalize(decodeURIComponent(pathname)).replace(
    /^(\.\.[/\\])+/,
    '',
  );
  let filePath = join(distDir, safePath);
  if (
    safePath === '/' ||
    !existsSync(filePath) ||
    statSync(filePath).isDirectory()
  ) {
    filePath = join(distDir, 'index.html');
  }

  const extension = extname(filePath);
  res.writeHead(200, {
    'Cache-Control': 'no-store',
    'Content-Type': contentTypes[extension] ?? 'application/octet-stream',
  });
  createReadStream(filePath).pipe(res);
}
