# Local session protocol v1

This protocol is transient: restarting the daemon creates a new table and invalidates all tokens. JSON is UTF-8. API paths are under `/api/local/v1`. Requests and responses use `application/json`; errors use `{ "error": "code", "message": "human-readable text" }` with an appropriate HTTP status.

| Method and path | Request | Response |
| --- | --- | --- |
| `POST /api/local/v1/join` | `{ "code": string, "name": string }` | `{ "token": string, "participant": Participant }` |
| `GET /api/local/v1/state` | `Authorization: Bearer <token>` | `{ "participants": Participant[], "sequence": number }` |
| `POST /api/local/v1/messages` | Bearer token; `{ "text": string }` | `Message` |
| `GET /api/local/v1/events?after=<sequence>` | EventSource cookie set by join, or Bearer token; `Last-Event-ID` is also accepted | SSE stream |
| `POST /api/local/v1/end` | Host control token | `{ "ended": true }` |

`Participant` is `{ "id": string, "name": string, "online": boolean }`. `Message` is `{ "id": number, "participant": string, "text": string }`. Each SSE event has `id: <sequence>`, `event: <type>`, and `data: <Event JSON>`. Event JSON contains `sequence` and `type`, plus `participant` for `participant` events or `message` for `message` events. `ended` has neither. Sequence numbers increase across all event types. Message IDs equal their event sequence.

The server retains the most recent 256 events. A reconnect sends `after` or `Last-Event-ID` and receives only later events, followed by live events. The client deduplicates by sequence. If the requested sequence predates the retained window, the server answers `409` with `replay_expired`; the client fetches `/state`, clears any gap-dependent view, and reconnects from that state's sequence. A slow event reader is disconnected and reconnects the same way. `/state` supplies current presence, not historical messages. The service worker never caches API responses.

Errors include `invalid_code` (403), `invalid_token` (401), `invalid_name`/`invalid_message` (400), `session_full` (409), `session_ended` (410), and `replay_expired` (409). Names and messages are trimmed. Names are 1–60 UTF-8 bytes; messages are 1–2000 UTF-8 bytes; at most 32 participants may join. Join codes have 72 random bits, are case insensitive, and are intended for a trusted local table. Participant tokens are 192-bit bearer credentials and must not be placed in URLs or logs.
