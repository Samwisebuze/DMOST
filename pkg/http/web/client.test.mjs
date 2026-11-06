import test from 'node:test';
import assert from 'node:assert/strict';
import { LocalClient, eventStream, parseSSE } from './client.mjs';

const storage = () => {
  const values = new Map();
  return { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) };
};

test('SSE parsing preserves ordered event IDs across chunks', async () => {
  const event = { sequence: 7, type: 'message', message: { id: 7, text: 'hello' } };
  assert.deepEqual(parseSSE(`id: 7\nevent: message\ndata: ${JSON.stringify(event)}`), event);
  const body = new ReadableStream({ start(controller) {
    controller.enqueue(new TextEncoder().encode('id: 7\nevent: message\nda'));
    controller.enqueue(new TextEncoder().encode(`ta: ${JSON.stringify(event)}\n\n`));
    controller.close();
  } });
  const received = [];
  for await (const item of eventStream(body)) received.push(item);
  assert.deepEqual(received, [event]);
});

test('join rejects an invalid code without storing credentials', async () => {
  const store = storage();
  const client = new LocalClient({ storage: store, fetcher: async () => new Response(JSON.stringify({ error: 'invalid_code', message: 'invalid code' }), { status: 403 }) });
  await assert.rejects(client.join('bad', 'Ada'), { code: 'invalid_code' });
  assert.equal(store.getItem('dmost-token'), null);
});

test('client joins, receives a message, and handles the ended event', async () => {
  const store = storage();
  const statuses = [], received = [];
  let complete;
  const done = new Promise(resolve => { complete = resolve; });
  const fetcher = async (url) => {
    if (url.endsWith('/join')) return new Response(JSON.stringify({ token: 'abc', participant: { id: 'p1', name: 'Ada' } }));
    if (url.endsWith('/state')) return new Response(JSON.stringify({ participants: [{ id: 'p1', name: 'Ada', online: false }], sequence: 1 }));
    if (url.includes('/events')) {
      const data = [
        { sequence: 2, type: 'message', message: { id: 2, participant: 'p1', text: 'hello' } },
        { sequence: 3, type: 'ended' },
      ].map(e => `id: ${e.sequence}\nevent: ${e.type}\ndata: ${JSON.stringify(e)}\n\n`).join('');
      return new Response(data, { headers: { 'Content-Type': 'text/event-stream' } });
    }
    throw new Error(`unexpected ${url}`);
  };
  const client = new LocalClient({ fetcher, storage: store, onStatus: s => { statuses.push(s); if (s === 'ended') complete(); }, onEvent: e => received.push(e) });
  await client.join('CODE', 'Ada');
  await Promise.race([done, new Promise((_, reject) => setTimeout(() => reject(new Error('timed out')), 1000))]);
  assert.deepEqual(received.map(e => e.type), ['message', 'ended']);
  assert.equal(store.getItem('dmost-token'), null);
  assert.deepEqual(statuses, ['connected', 'ended']);
});

test('expired replay resyncs from current state', async () => {
  const store = storage();
  store.setItem('dmost-token', 'abc');
  store.setItem('dmost-sequence', '1');
  const statuses = [], eventURLs = [];
  let finish;
  const done = new Promise(resolve => { finish = resolve; });
  let stateCalls = 0;
  const fetcher = async url => {
    if (url.endsWith('/state')) {
      stateCalls++;
      return new Response(JSON.stringify({ participants: [], sequence: stateCalls === 1 ? 260 : 300 }));
    }
    if (url.includes('/events')) {
      eventURLs.push(url);
      if (eventURLs.length === 1) return new Response(JSON.stringify({ error: 'replay_expired' }), { status: 409 });
      return new Response(`id: 301\nevent: ended\ndata: {"sequence":301,"type":"ended"}\n\n`);
    }
    throw new Error(url);
  };
  const client = new LocalClient({ fetcher, storage: store, onStatus: value => { statuses.push(value); if (value === 'ended') finish(); } });
  client.start();
  await Promise.race([done, new Promise((_, reject) => setTimeout(() => reject(new Error('timed out')), 1000))]);
  assert.deepEqual(eventURLs, ['/api/local/v1/events?after=1', '/api/local/v1/events?after=300']);
  assert.deepEqual(statuses, ['resync', 'connected', 'ended']);
});

test('network loss shows disconnected and retries', async () => {
  const store = storage();
  store.setItem('dmost-token', 'abc');
  const statuses = [];
  let finish;
  const done = new Promise(resolve => { finish = resolve; });
  let stateCalls = 0;
  const fetcher = async url => {
    if (url.endsWith('/state')) {
      stateCalls++;
      if (stateCalls === 1) throw new TypeError('network offline');
      return new Response(JSON.stringify({ participants: [], sequence: 1 }));
    }
    if (url.includes('/events')) return new Response('id: 2\nevent: ended\ndata: {"sequence":2,"type":"ended"}\n\n');
    throw new Error(url);
  };
  const client = new LocalClient({ fetcher, storage: store, onStatus: value => { statuses.push(value); if (value === 'ended') finish(); } });
  client.start();
  await Promise.race([done, new Promise((_, reject) => setTimeout(() => reject(new Error('timed out')), 3000))]);
  assert.deepEqual(statuses, ['disconnected', 'connected', 'ended']);
  assert.equal(stateCalls, 2);
});
