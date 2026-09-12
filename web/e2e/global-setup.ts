import { spawn } from 'child_process';
import http from 'http';
import { writeFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const DIR = dirname(fileURLToPath(import.meta.url));
const PORT = 6181;
const TMP = join(DIR, 'server.pid');

export default async function globalSetup() {
  const child = spawn('node', [join(DIR, 'server.js')], {
    detached: true,
    stdio: ['ignore', 'ignore', 'ignore'],
  });
  child.unref();
  writeFileSync(TMP, String(child.pid));

  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    try {
      await new Promise<void>((resolve, reject) => {
        const req = http.request({ host: '127.0.0.1', port: PORT, path: '/', method: 'HEAD', timeout: 500 }, (res) => {
          res.resume();
          resolve();
        });
        req.on('error', reject);
        req.on('timeout', () => req.destroy(new Error('timeout')));
        req.end();
      });
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 150));
    }
  }
  throw new Error('civora static server did not start in time');
}
