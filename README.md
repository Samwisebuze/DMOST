# DMOST 

## Dungeon Master's Open Source Toolkit

## Description 

This is a personal project to explore building native and local first applications with golang. 

### Outline 

* [CONTRIBUTING.md] before open any issues or pull requests. 
* [ARCHITECTURE.md] explains the layout of modules and artifacts, along with a high level over view of the whole system.
* [MAINTENANCE.md] explains regular SDLC maintenance to keep the project healthy and secure.

## Checks

Run `./scripts/check-web.sh` for the local web client and `./scripts/quality-gate.sh` for web and all three Go modules. The web client uses plain browser assets and Node's built-in test runner; no package installation or CDN is needed. Cache Go module dependencies once before running these checks without internet access.

## Local table demo

Run `./scripts/demo-local.sh` on Linux to print one address for player devices. See [LOCAL_DEMO.md](docs/LOCAL_DEMO.md) for offline certificate setup and the iOS handoff.
