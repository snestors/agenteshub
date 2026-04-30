#!/usr/bin/env node
import { spawn } from 'node:child_process';
import { writeFile, rm, mkdir } from 'node:fs/promises';
import { request } from 'node:http';
import crypto from 'node:crypto';

const args = process.argv.slice(2);
const outDir = args[0];
if (!outDir) {
  console.error('usage: capture-agenthub-ui.mjs <out-dir> [base-url] [routeSpec...]');
  console.error('routeSpec format: name=/path, e.g. chat=/ system=/system');
  process.exit(2);
}
const base = args[1] || process.env.AGENTHUB_CAPTURE_BASE_URL || 'http://127.0.0.1:8093';
const routeSpecs = args.slice(2);
const routes = routeSpecs.length
  ? routeSpecs.map((s) => {
      const idx = s.indexOf('=');
      if (idx < 1) throw new Error(`bad routeSpec: ${s}`);
      return [s.slice(0, idx), s.slice(idx + 1) || '/'];
    })
  : [
      ['chat-main', '/'],
      ['system-cronjobs', '/system'],
      ['projects', '/projects'],
      ['skills', '/skills'],
      ['releases', '/releases'],
    ];

const port = 9333 + Math.floor(Math.random() * 1000);
const profile = `/tmp/agenthub-capture-chrome-${process.pid}`;

function b64url(obj) {
  const s = typeof obj === 'string' ? obj : JSON.stringify(obj);
  return Buffer.from(s).toString('base64url');
}
function jwt() {
  const secret = process.env.AGENTHUB_JWT_SECRET;
  if (!secret) throw new Error('missing AGENTHUB_JWT_SECRET; run with: set -a; source .env; set +a');
  const now = Math.floor(Date.now() / 1000);
  const header = { alg: 'HS256', typ: 'JWT' };
  const payload = {
    sub: '1',
    jti: crypto.randomUUID(),
    iat: now,
    exp: now + 3600 * 4,
    refresh_until: now + 3600 * 24 * 7,
  };
  const unsigned = `${b64url(header)}.${b64url(payload)}`;
  const sig = crypto.createHmac('sha256', secret).update(unsigned).digest('base64url');
  return `${unsigned}.${sig}`;
}
function getJSON(url, method = 'GET') {
  return new Promise((resolve, reject) => {
    request(url, { method }, (res) => {
      let data = '';
      res.setEncoding('utf8');
      res.on('data', (c) => (data += c));
      res.on('end', () => {
        try {
          resolve(JSON.parse(data));
        } catch (e) {
          reject(new Error(`${url}: ${String(data).slice(0, 160)}`));
        }
      });
    }).on('error', reject).end();
  });
}
async function waitForVersion() {
  for (let i = 0; i < 80; i++) {
    try {
      return await getJSON(`http://127.0.0.1:${port}/json/version`);
    } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error('chrome devtools not ready');
}
async function send(ws, method, params = {}) {
  const id = ++send.id;
  ws.send(JSON.stringify({ id, method, params }));
  return new Promise((resolve, reject) => {
    const onMessage = (ev) => {
      const msg = JSON.parse(ev.data.toString());
      if (msg.id !== id) return;
      ws.removeEventListener('message', onMessage);
      if (msg.error) reject(new Error(`${method}: ${JSON.stringify(msg.error)}`));
      else resolve(msg.result || {});
    };
    ws.addEventListener('message', onMessage);
  });
}
send.id = 0;

await mkdir(outDir, { recursive: true });
const chrome = spawn('/usr/bin/google-chrome', [
  '--headless=new',
  '--no-sandbox',
  '--disable-gpu',
  '--hide-scrollbars',
  `--remote-debugging-port=${port}`,
  `--user-data-dir=${profile}`,
  'about:blank',
], { stdio: 'ignore' });

try {
  await waitForVersion();
  const target = await getJSON(`http://127.0.0.1:${port}/json/new?${encodeURIComponent(`${base}/`)}`, 'PUT');
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve, { once: true });
    ws.addEventListener('error', reject, { once: true });
  });
  await send(ws, 'Page.enable');
  await send(ws, 'Network.enable');
  await send(ws, 'Runtime.enable');
  await send(ws, 'Emulation.setDeviceMetricsOverride', {
    width: 1440,
    height: 900,
    deviceScaleFactor: 1,
    mobile: false,
  });
  await send(ws, 'Network.setCookie', {
    name: 'agenthub_token',
    value: jwt(),
    url: base,
    httpOnly: true,
    secure: false,
    sameSite: 'Lax',
  });
  for (const [name, path] of routes) {
    await send(ws, 'Page.navigate', { url: base + path });
    await new Promise((r) => setTimeout(r, 2500));
    const result = await send(ws, 'Page.captureScreenshot', {
      format: 'png',
      captureBeyondViewport: false,
      fromSurface: true,
    });
    await writeFile(`${outDir}/${name}.png`, Buffer.from(result.data, 'base64'));
    console.log(`${name}.png`);
  }
  ws.close();
} finally {
  chrome.kill('SIGTERM');
  await new Promise((r) => setTimeout(r, 500));
  await rm(profile, { recursive: true, force: true });
}
