import { spawn } from 'node:child_process';
import { execFileSync } from 'node:child_process';
import { createServer } from 'node:net';
import { setTimeout as sleep } from 'node:timers/promises';

const children = [];
const launch = (command, args) => {
  const child = spawn(command, args, { stdio: ['ignore', 'pipe', 'pipe'] });
  children.push(child);
  return child;
};
const freePort = () => new Promise((resolve, reject) => {
  const server = createServer();
  server.once('error', reject);
  server.listen(0, '127.0.0.1', () => { const port = server.address().port; server.close(() => resolve(port)); });
});
const waitFor = async (condition, label) => {
  for (let i = 0; i < 100; i++) {
    const result = await condition();
    if (result) return result;
    await sleep(100);
  }
  throw new Error(`Timed out waiting for ${label}`);
};

let sessionID, driverURL;
try {
  execFileSync('go', ['build', './...'], { cwd: 'cmd/dmostd', stdio: 'inherit', env: { ...process.env, GOWORK: 'off', GOCACHE: process.env.GOCACHE || '/tmp/dmost-go-build-cache' } });
  const hostPort = await freePort();
  const host = launch('cmd/dmostd/dmostd', ['--local', '--advertise', '127.0.0.1', '--listen', `127.0.0.1:${hostPort}`]);
  let hostOutput = '';
  host.stdout.on('data', chunk => { hostOutput += chunk; });
  host.stderr.on('data', chunk => { hostOutput += chunk; });
  const code = await waitFor(() => hostOutput.match(/^Join code: (\S+)/m)?.[1], 'host join code');
  const driverPort = await freePort();
  driverURL = `http://127.0.0.1:${driverPort}`;
  const driver = launch('geckodriver', [`--port=${driverPort}`]);
  let driverOutput = '';
  driver.stderr.on('data', chunk => { driverOutput += chunk; });
  await waitFor(async () => { try { return (await fetch(`${driverURL}/status`)).ok; } catch { return false; } }, 'browser driver');

  const wd = async (method, path, body) => {
    const response = await fetch(`${driverURL}${path}`, { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
    const payload = await response.json();
    if (!response.ok || payload.value?.error) throw new Error(JSON.stringify(payload));
    return payload.value;
  };
  const created = await wd('POST', '/session', { capabilities: { alwaysMatch: { browserName: 'firefox', 'moz:firefoxOptions': { args: ['-headless'] } } } });
  sessionID = created.sessionId;
  const basePath = `/session/${sessionID}`;
  const hostURL = `http://127.0.0.1:${hostPort}/`;
  await wd('POST', `${basePath}/url`, { url: hostURL });
  const element = async selector => (await wd('POST', `${basePath}/element`, { using: 'css selector', value: selector }))['element-6066-11e4-a52e-4f735466cecf'];
  const type = async (selector, value) => wd('POST', `${basePath}/element/${await element(selector)}/value`, { text: value });
  await type('input[name="name"]', 'Ada');
  await type('input[name="code"]', code);
  await wd('POST', `${basePath}/element/${await element('#join-form button')}/click`, {});
  try {
    await waitFor(async () => (await wd('GET', `${basePath}/element/${await element('#status')}/text`)).includes('Connected'), 'browser join');
  } catch (error) {
    console.error('Browser status:', await wd('GET', `${basePath}/element/${await element('#status')}/text`));
    console.error('Join error:', await wd('GET', `${basePath}/element/${await element('#join-error')}/text`));
    throw error;
  }

  const joined = await fetch(`${hostURL}api/local/v1/join`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code, name: 'Bob' }) }).then(r => r.json());
  await fetch(`${hostURL}api/local/v1/messages`, { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${joined.token}` }, body: JSON.stringify({ text: 'hello from Bob' }) });
  await waitFor(async () => (await wd('GET', `${basePath}/element/${await element('#messages')}/text`)).includes('hello from Bob'), 'browser message');
  console.log('Browser join and live message: pass');
} catch (error) {
  console.error(error);
  process.exitCode = 1;
} finally {
  if (sessionID && driverURL) {
    await fetch(`${driverURL}/session/${sessionID}`, { method: 'DELETE' }).catch(() => {});
  }
  for (const child of children.reverse()) child.kill('SIGTERM');
}
