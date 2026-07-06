import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { responseForApiRequest } from './fixtures.mjs';

const port = Number(process.env.WINRIFT_DEMO_MEDIA_PORT ?? 4184);
const host = process.env.WINRIFT_DEMO_MEDIA_HOST ?? '127.0.0.1';
const distDir = fileURLToPath(new URL('../../dist/', import.meta.url));

const contentTypes = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.webp': 'image/webp',
};

const server = createServer(async (req, res) => {
  if (!req.url) {
    res.writeHead(400).end('Missing URL');
    return;
  }

  if (req.url.startsWith('/api/')) {
    await serveApi(req, res);
    return;
  }

  serveStatic(req.url, res);
});

server.listen(port, host, () => {
  console.log(`WinRift demo media server listening at http://${host}:${port}`);
});

async function serveApi(req, res) {
  const body = await readBody(req);
  const response = responseForApiRequest(req.url ?? '/', req.method ?? 'GET', body);
  res.writeHead(response.status ?? 200, {
    'Cache-Control': 'no-store',
    'Content-Type': 'application/json; charset=utf-8',
  });
  res.end(JSON.stringify(response.body));
}

function serveStatic(requestURL, res) {
  const { pathname } = new URL(requestURL, `http://${host}:${port}`);
  const safePath = normalize(decodeURIComponent(pathname)).replace(/^(\.\.[/\\])+/, '');
  let filePath = join(distDir, safePath);
  if (safePath === '/' || !existsSync(filePath) || statSync(filePath).isDirectory()) {
    filePath = join(distDir, 'index.html');
  }

  const extension = extname(filePath);
  res.writeHead(200, {
    'Cache-Control': 'no-store',
    'Content-Type': contentTypes[extension] ?? 'application/octet-stream',
  });
  createReadStream(filePath).pipe(res);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let body = '';
    req.setEncoding('utf8');
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => {
      if (!body) {
        resolve(undefined);
        return;
      }
      try {
        resolve(JSON.parse(body));
      } catch (error) {
        reject(error);
      }
    });
    req.on('error', reject);
  });
}
