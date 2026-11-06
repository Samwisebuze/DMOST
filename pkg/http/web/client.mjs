export function parseSSE(block) {
  let id = 0, data = '';
  for (const line of block.split(/\r?\n/)) {
    if (line.startsWith('id:')) id = Number(line.slice(3).trim());
    if (line.startsWith('data:')) data += line.slice(5).trimStart();
  }
  if (!data || !Number.isSafeInteger(id) || id < 1) return null;
  const event = JSON.parse(data);
  return event.sequence === id ? event : null;
}

export async function* eventStream(body) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n');
      let end;
      while ((end = buffer.indexOf('\n\n')) >= 0) {
        const event = parseSSE(buffer.slice(0, end));
        buffer = buffer.slice(end + 2);
        if (event) yield event;
      }
    }
  } finally {
    reader.releaseLock();
  }
}

const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

export class LocalClient {
  constructor({ fetcher = (...args) => fetch(...args), storage = localStorage, onState = () => {}, onEvent = () => {}, onStatus = () => {} } = {}) {
    this.fetcher = fetcher;
    this.storage = storage;
    this.onState = onState;
    this.onEvent = onEvent;
    this.onStatus = onStatus;
    this.token = storage.getItem('dmost-token') || '';
    this.last = Number(storage.getItem('dmost-sequence') || 0);
    this.running = false;
    this.controller = null;
    this.epoch = 0;
  }

  async request(path, options = {}) {
    const response = await this.fetcher(`/api/local/v1${path}`, {
      ...options,
      headers: { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}) },
      cache: 'no-store',
    });
    if (!response.ok) {
      let problem = {};
      try { problem = await response.json(); } catch { /* disconnected response */ }
      const error = new Error(problem.message || `Request failed (${response.status})`);
      error.status = response.status;
      error.code = problem.error;
      throw error;
    }
    return response;
  }

  async join(code, name) {
    this.forget();
    const response = await this.request('/join', { method: 'POST', body: JSON.stringify({ code, name }) });
    const joined = await response.json();
    this.token = joined.token;
    this.storage.setItem('dmost-token', this.token);
    this.storage.setItem('dmost-name', name);
    this.storage.setItem('dmost-sequence', '0');
    this.start();
    return joined.participant;
  }

  async send(text) {
    const response = await this.request('/messages', { method: 'POST', body: JSON.stringify({ text }) });
    return response.json();
  }

  start() {
    if (!this.token || this.running) return;
    this.running = true;
    this.run(++this.epoch);
  }

  stop() {
    this.running = false;
    this.epoch++;
    this.controller?.abort();
  }

  forget() {
    this.stop();
    this.token = '';
    this.last = 0;
    this.storage.removeItem('dmost-token');
    this.storage.removeItem('dmost-sequence');
  }

  async run(epoch) {
    while (this.running && epoch === this.epoch) {
      this.controller = new AbortController();
      try {
        const stateResponse = await this.request('/state', { signal: this.controller.signal });
        const state = await stateResponse.json();
        this.onState(state);
        if (this.last === 0) this.setLast(state.sequence);
        const response = await this.request(`/events?after=${this.last}`, { signal: this.controller.signal });
        this.onStatus('connected');
        for await (const event of eventStream(response.body)) {
          if (!this.running || epoch !== this.epoch) break;
          if (event.sequence <= this.last) continue;
          this.setLast(event.sequence);
          this.onEvent(event);
          if (event.type === 'ended') {
            this.onStatus('ended');
            this.forget();
            return;
          }
        }
      } catch (error) {
        if (!this.running || epoch !== this.epoch) return;
        if (error.code === 'replay_expired') {
          this.setLast(0);
          this.onStatus('resync');
          continue;
        }
        if (error.code === 'session_ended') {
          this.onStatus('ended');
          this.forget();
          return;
        }
        if (error.code === 'invalid_token') {
          this.onStatus('rejected');
          this.forget();
          return;
        }
        this.onStatus('disconnected');
      }
      if (this.running && epoch === this.epoch) await pause(1200);
    }
  }

  setLast(sequence) {
    this.last = sequence;
    this.storage.setItem('dmost-sequence', String(sequence));
  }
}
