import { LocalClient } from './client.mjs';

const status = document.querySelector('#status');
const joinForm = document.querySelector('#join-form');
const chatForm = document.querySelector('#chat-form');
const members = document.querySelector('#members');
const messages = document.querySelector('#messages');
const joinError = document.querySelector('#join-error');
const participants = new Map();
const seenMessages = new Set();

function renderMembers() {
  members.replaceChildren();
  for (const p of [...participants.values()].sort((a, b) => a.name.localeCompare(b.name))) {
    const li = document.createElement('li');
    li.textContent = `${p.name} ${p.online ? '● online' : '○ offline'}`;
    members.append(li);
  }
}

function showStatus(value) {
  const labels = { connected: 'Connected to the table', disconnected: 'Disconnected. Reconnecting…', resync: 'Some events were missed. Resyncing…', ended: 'The DM ended this table.', rejected: 'Your session is no longer valid. Join again.' };
  status.textContent = labels[value] || value;
  status.dataset.state = value;
  if (value === 'ended' || value === 'rejected') {
    chatForm.hidden = true;
    joinForm.hidden = false;
  }
  if (value === 'resync') {
    messages.replaceChildren();
    seenMessages.clear();
  }
}

const client = new LocalClient({
  onStatus: showStatus,
  onState: state => {
    participants.clear();
    for (const p of state.participants) participants.set(p.id, p);
    renderMembers();
    joinForm.hidden = true;
    chatForm.hidden = false;
  },
  onEvent: event => {
    if (event.type === 'participant') {
      participants.set(event.participant.id, event.participant);
      renderMembers();
    }
    if (event.type === 'message' && !seenMessages.has(event.message.id)) {
      seenMessages.add(event.message.id);
      const li = document.createElement('li');
      const sender = participants.get(event.message.participant)?.name || 'Player';
      const strong = document.createElement('strong');
      strong.textContent = `${sender}: `;
      li.append(strong, document.createTextNode(event.message.text));
      messages.append(li);
      messages.scrollTop = messages.scrollHeight;
    }
  },
});

joinForm.addEventListener('submit', async event => {
  event.preventDefault();
  joinError.textContent = '';
  const data = new FormData(joinForm);
  try {
    await client.join(String(data.get('code')).trim(), String(data.get('name')).trim());
  } catch (error) {
    joinError.textContent = error.message;
    showStatus('rejected');
  }
});

chatForm.addEventListener('submit', async event => {
  event.preventDefault();
  const input = chatForm.querySelector('input[name="message"]');
  const text = input.value.trim();
  if (!text) return;
  try {
    await client.send(text);
    input.value = '';
  } catch (error) {
    showStatus(error.code === 'session_ended' ? 'ended' : 'disconnected');
  }
});

if (client.token) client.start();
if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(() => {});
