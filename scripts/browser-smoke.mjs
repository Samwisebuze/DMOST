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

const sessions = [];
try {
  execFileSync('go', ['build', './...'], { cwd: 'cmd/dmostd', stdio: 'inherit', env: { ...process.env, GOWORK: 'off', GOCACHE: process.env.GOCACHE || '/tmp/dmost-go-build-cache' } });
  const hostPort = await freePort();
  const host = launch('cmd/dmostd/dmostd', ['--local', '--advertise', '127.0.0.1', '--listen', `127.0.0.1:${hostPort}`]);
  let hostOutput = '';
  host.stdout.on('data', chunk => { hostOutput += chunk; });
  host.stderr.on('data', chunk => { hostOutput += chunk; });
  const code = await waitFor(() => hostOutput.match(/^Join code: (\S+)/m)?.[1], 'host join code');
  const hostURL = `http://127.0.0.1:${hostPort}/`;
  const openBrowser = async name => {
    const driverPort = await freePort();
    const driverURL = `http://127.0.0.1:${driverPort}`;
    launch('geckodriver', [`--port=${driverPort}`]);
    await waitFor(async () => { try { return (await fetch(`${driverURL}/status`)).ok; } catch { return false; } }, `${name} browser driver`);
    const wd = async (method, path, body) => {
      const response = await fetch(`${driverURL}${path}`, { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
      const payload = await response.json();
      if (!response.ok || payload.value?.error) throw new Error(JSON.stringify(payload));
      return payload.value;
    };
    const created = await wd('POST', '/session', { capabilities: { alwaysMatch: { browserName: 'firefox', 'moz:firefoxOptions': { args: ['-headless'] } } } });
    const basePath = `/session/${created.sessionId}`;
    sessions.push({ driverURL, sessionID: created.sessionId });
    await wd('POST', `${basePath}/url`, { url: hostURL });
    const element = async selector => (await wd('POST', `${basePath}/element`, { using: 'css selector', value: selector }))['element-6066-11e4-a52e-4f735466cecf'];
    const type = async (selector, value) => wd('POST', `${basePath}/element/${await element(selector)}/value`, { text: value });
    const text = async selector => wd('GET', `${basePath}/element/${await element(selector)}/text`);
    await type('input[name="name"]', name);
    await type('input[name="code"]', code);
    await wd('POST', `${basePath}/element/${await element('#join-form button')}/click`, {});
    try {
      await waitFor(async () => (await text('#status')).includes('Connected'), `${name} browser join`);
    } catch (error) {
      console.error(`${name} status:`, await text('#status'));
      console.error(`${name} join error:`, await text('#join-error'));
      throw error;
    }
    return { wd, basePath, element, type, text };
  };
  const ada = await openBrowser('Ada');
  const bob = await openBrowser('Bob');
  const send = async (browser, message) => {
    await browser.type('input[name="message"]', message);
    await browser.wd('POST', `${browser.basePath}/element/${await browser.element('#chat-form button')}/click`, {});
  };
  await send(ada, 'hello from Ada');
  await send(bob, 'hello from Bob');
  for (const browser of [ada, bob]) {
    await waitFor(async () => {
      const text = await browser.text('#messages');
      return text.includes('hello from Ada') && text.includes('hello from Bob');
    }, 'two browser messages');
  }
  console.log('Two browsers joined and exchanged live messages: pass');
} catch (error) {
  console.error(error);
  process.exitCode = 1;
} finally {
  for (const { driverURL, sessionID } of sessions) await fetch(`${driverURL}/session/${sessionID}`, { method: 'DELETE' }).catch(() => {});
  for (const child of children.reverse()) child.kill('SIGTERM');
}
