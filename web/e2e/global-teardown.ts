import { readFileSync, unlinkSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const DIR = dirname(fileURLToPath(import.meta.url));
const TMP = join(DIR, 'server.pid');

export default async function globalTeardown() {
  let pid: number | undefined;
  try {
    pid = Number(readFileSync(TMP, 'utf8').trim());
  } catch {
    return;
  }
  try {
    process.kill(pid, 'SIGTERM');
    await new Promise((r) => setTimeout(r, 300));
    try {
      process.kill(pid, 'SIGKILL');
    } catch {
      /* already gone */
    }
  } catch (e) {
    /* process already exited */
  }
  try {
    unlinkSync(TMP);
  } catch {
    /* ignore */
  }
}
