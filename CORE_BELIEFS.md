# Core Beliefs

DMOST is the Dungeon Master's Open Source Toolkit. These beliefs guide product, design, and engineering decisions for the people and agents building it. They describe the direction of the project, including capabilities that may still need to be built.

## The table owns its experience

The Dungeon Master, Players, and the people at the table decide how they play. DMOST should support their choices without making one workflow, rules interpretation, or style of play mandatory. When a feature makes an assumption, make that assumption visible and give people a way to change or leave it behind.

## Local first, with data people can keep

People own the characters, campaigns, notes, and other material they create. Their work should remain available on their own devices and useful without a service connection. Sync and collaboration can add value, but should not become prerequisites for using or keeping that work.

Make data understandable and portable. A person should be able to inspect what is stored, understand where it came from and how it changed, export it in a documented format, and import or move it to another tool. Prefer formats and data models that preserve meaning outside DMOST. Treat deletion, backup, and recovery as part of ownership too.

## Malleable software

People should be able to adapt their tools to the needs of their table with minimal friction. Favor editable content, composable pieces, and clear extension points over hard-coded assumptions. A useful change should be possible without having to understand the whole system or ask the project maintainers to add a special case.

Start with common paths that work well. Leave room for people to change those paths when their game calls for it.

## Open source in practice

DMOST is an open source project. People should be able to study, run, modify, and share the software. Keep design decisions, data formats, and interfaces documented well enough for contributors to participate and for others to build on the project. Prefer open standards and dependencies that keep the project and its users free to move.

## Reasonable, simple, and fun

Build software that solves a real problem at a cost people can understand. Choose the smallest design that works for a concrete use case, then grow it as real needs appear. A complex system grows from a simple thing that works; added complexity should earn its place through a clear benefit.

Make the experience pleasant to use and to build. Reduce ceremony where it interrupts play or contribution. Preserve the sense of creativity and discovery that brings people to tabletop games.

## Make the system understandable

People should be able to reason about what DMOST does and why. Use clear names, visible state, traceable changes, and explanations where the software makes a consequential choice. Keep architecture and rules understandable to contributors, including newcomers. When a design requires hidden behavior or extensive special knowledge, simplify it or explain the need plainly.

## Using these beliefs

For a product or design choice, ask:

1. Does this give the people at the table more control over their experience and their data?
2. Can they inspect, understand, change, and take their work elsewhere?
3. Can someone adapt this to a different table without changing the core for everyone?
4. Is this the simplest working step toward a demonstrated need?
5. Can a newcomer understand the behavior and the reason for it?

These questions are guides, not a scoring system. When they pull in different directions, record the tradeoff and choose the path that best preserves user agency and keeps the next change possible.
