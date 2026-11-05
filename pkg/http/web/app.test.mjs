import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('offline shell declares its local assets', () => {
  const manifest = JSON.parse(readFileSync(new URL('./manifest.webmanifest', import.meta.url)));
  assert.equal(manifest.display, 'standalone');
  assert.equal(manifest.start_url, '/');
  const worker = readFileSync(new URL('./sw.js', import.meta.url), 'utf8');
  assert.match(worker, /\/api\//);
});
