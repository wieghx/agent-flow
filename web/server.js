const http = require('http');
const fs = require('fs');
const path = require('path');

const API_URL = process.env.API_URL || 'http://localhost:8082';
const PORT = Number(process.env.PORT || 3000);
const WEB_ROOT = __dirname;
const DIST_ROOT = path.join(WEB_ROOT, 'dist');

const API_PREFIXES = ['/chat', '/tasks', '/conversation', '/outputs', '/workflows', '/novels', '/observability'];

const MIME = {
  '.html': 'text/html',
  '.js': 'application/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.svg': 'image/svg+xml',
  '.woff2': 'font/woff2',
};

function sendCors(res) {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  res.setHeader('Access-Control-Allow-Credentials', 'true');
}

function serveStatic(filePath, res) {
  fs.readFile(filePath, (err, data) => {
    if (err) {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end('Not Found');
      return;
    }
    const ext = path.extname(filePath);
    res.writeHead(200, { 'Content-Type': MIME[ext] || 'application/octet-stream' });
    res.end(data);
  });
}

function proxyRequest(req, res) {
  const apiUrl = new URL(API_URL);
  const options = {
    hostname: apiUrl.hostname,
    port: apiUrl.port,
    path: req.url,
    method: req.method,
    headers: req.headers,
    timeout: 120000, // 2 min overall
  };

  const proxyReq = http.request(options, (proxyRes) => {
    sendCors(res);
    if ((proxyRes.headers['content-type'] || '').includes('text/event-stream')) {
      proxyRes.headers['cache-control'] = 'no-cache';
      proxyRes.headers['x-accel-buffering'] = 'no';
    }
    res.writeHead(proxyRes.statusCode, proxyRes.headers);
    proxyRes.pipe(res);
  });

  proxyReq.setTimeout(120000, () => {
    proxyReq.destroy(new Error('proxy timeout'));
  });

  proxyReq.on('error', (e) => {
    if (!res.headersSent) {
      res.writeHead(502, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Proxy error: ' + e.message }));
    }
  });

  req.pipe(proxyReq);
}

const server = http.createServer((req, res) => {
  sendCors(res);
  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    res.end();
    return;
  }

  const urlPath = req.url.split('?')[0];
  if (API_PREFIXES.some((p) => urlPath === p || urlPath.startsWith(p + '/'))) {
    return proxyRequest(req, res);
  }

  // SPA static assets from dist/
  if (fs.existsSync(DIST_ROOT)) {
    const safePath = path.normalize(urlPath).replace(/^(\.\.[/\\])+/, '');
    const filePath = path.join(DIST_ROOT, safePath === '/' ? 'index.html' : safePath);
    if (fs.existsSync(filePath) && fs.statSync(filePath).isFile()) {
      return serveStatic(filePath, res);
    }
    // client-side route fallback
    return serveStatic(path.join(DIST_ROOT, 'index.html'), res);
  }

  res.writeHead(503, { 'Content-Type': 'text/plain' });
  res.end('Web UI not built. Run: cd web && npm run build');
});

server.listen(PORT, () => {
  console.log(`Web server at http://localhost:${PORT}`);
  console.log(`API proxy -> ${API_URL}`);
  console.log(fs.existsSync(DIST_ROOT) ? 'Serving dist/ (React build)' : 'dist/ missing');
});