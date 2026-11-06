# Run a local table

On Linux, connect the host and players to the same Wi-Fi network or hotspot. The network does not need internet access. Install Go once and cache the module dependencies before going offline. From the repository root run:

```sh
./scripts/demo-local.sh
```

The script builds and starts `dmostd`, then prints one address to open on a player device and a join code. Keep it running during play; Ctrl+C ends the table. The CA and host log stay in `.dmost-local/`, which is excluded from Git. Reusing that directory avoids reinstalling trust every session. Remove the trusted CA from player devices when no longer needed.

If automatic LAN address selection chooses the wrong interface, set `DMOST_ADVERTISE` to the host's Wi-Fi IPv4 address. `DMOST_LISTEN` and `DMOST_BOOTSTRAP_LISTEN` override the default HTTPS `:8443` and HTTP bootstrap `:8081` ports. Allow inbound TCP on those ports in the Linux firewall. A router with client isolation enabled may block players even though the host is running.

The printed address opens an HTTP bootstrap page. It carries a local CA certificate and a link to the HTTPS PWA. Compare the CA fingerprint on the bootstrap page with the fingerprint printed by the host before trusting it. The HTTP page itself is not authenticated, so this comparison matters on shared networks. The CA private key never leaves the Linux host; treat `.dmost-local/ca-key.pem` as a secret. The join code is a separate table entry control. The host control token appears only in the host log and can end the session through `POST /api/local/v1/end` with `Authorization: Bearer <token>`.

## iPhone and iPad

1. With internet disconnected and local Wi-Fi connected, open the single printed address in Safari.
2. Compare the displayed CA fingerprint with the host output. Download the certificate. In Settings, install the downloaded profile; on current iOS this is normally under **Profile Downloaded**.
3. In **Settings → General → About → Certificate Trust Settings**, enable full trust for **DMOST Local Table CA**.
4. Return to the bootstrap page and tap **Open the secure app**. Safari should show a normal trusted HTTPS page. Use **Share → Add to Home Screen** to install it, then enter the join code.

The user will validate these physical-device steps after the build. Record the iOS/iPadOS version, Safari version, whether certificate trust and home-screen install succeeded, and any error message. Do not bypass a certificate warning: confirm the IP and fingerprint first.

## Android and desktop Chromium

On Android Chrome, open the printed bootstrap address, compare the fingerprint, download and install the CA certificate through device security settings, then return and open the HTTPS app. Chrome's install menu can add the PWA. Device-specific certificate menus vary. On desktop Chromium, import the CA into the browser or system trust store, open the HTTPS link, and use the install control. In all cases, confirm no certificate warning and test while the WAN is disconnected.

## Automated smoke check

`./scripts/check-local-demo.sh` starts a loopback-only host, fetches the single bootstrap address and CA download, follows the HTTPS link with the generated CA, and confirms the PWA shell is served. `./scripts/quality-gate.sh` runs the three Go modules and web checks. These checks establish the host-side path; physical browser trust and installation remain device validation.

If Firefox and geckodriver are installed, `node scripts/browser-smoke.mjs` opens the PWA in headless Firefox, joins as one player, sends a message from a second HTTP client, and verifies the browser receives it live. It uses a loopback HTTP host because browsers treat localhost as a secure context; the separate HTTPS smoke check verifies the actual LAN certificate path.
