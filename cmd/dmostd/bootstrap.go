package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"html/template"
	"net"
	"net/http"
)

var bootstrapPage = template.Must(template.New("bootstrap").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>DMOST Local Table setup</title><style>body{font:1.1rem system-ui;margin:2rem auto;max-width:38rem;padding:0 1rem;line-height:1.5}a{display:inline-block;padding:.7rem 1rem;background:#211b35;color:white;border-radius:.5rem}</style></head>
<body><h1>Join the local table</h1><p>This page is served by the DM's computer on the local network. Before installing its certificate, compare this fingerprint with the one on the DM's computer: <strong>{{.Fingerprint}}</strong>.</p>
<ol><li><a href="/dmost-ca.cer">Download the local table certificate</a>.</li><li>On iPhone or iPad, open Settings → Profile Downloaded and install the profile. Then open Settings → General → About → Certificate Trust Settings and enable full trust for “DMOST Local Table CA”.</li><li><a href="{{.AppURL}}">Open the secure app</a>. In Safari, use Share → Add to Home Screen.</li><li>Enter the join code shown on the DM's computer.</li></ol>
<p>Only trust this certificate for a DM you know. You can remove it from your device settings after the game.</p></body></html>`))

func (m *Main) startBootstrap(ip string) (int, error) {
	addr := m.BootstrapAddr
	if addr == "" {
		addr = ":8081"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, err
	}
	appURL := fmt.Sprintf("https://%s:%d/", ip, m.HTTPServer.Port())
	caBlock, _ := pem.Decode(m.localCAPEM)
	if caBlock == nil {
		_ = ln.Close()
		return 0, fmt.Errorf("invalid local CA certificate")
	}
	sum := sha256.Sum256(caBlock.Bytes)
	fingerprint := hex.EncodeToString(sum[:])
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = bootstrapPage.Execute(w, struct{ AppURL, Fingerprint string }{appURL, fingerprint})
	})
	mux.HandleFunc("GET /dmost-ca.cer", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", "attachment; filename=dmost-ca.cer")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(caBlock.Bytes)
	})
	m.BootstrapServer = &http.Server{Handler: mux}
	go m.BootstrapServer.Serve(ln)
	return ln.Addr().(*net.TCPAddr).Port, nil
}
