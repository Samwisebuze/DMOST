package main_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	main "github.com/samwisebuze/dmost/cmd/dmostd"
)

func TestLocalSessionTwoClients(t *testing.T) {
	m := main.NewMain()
	m.LocalMode = true
	m.AdvertiseIP = "127.0.0.1"
	m.HTTPServer.Addr = "127.0.0.1:0"
	if err := m.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	base := m.HTTPServer.URL() + "/api/local/v1"
	if code := postJSON(t, base+"/join", "", map[string]string{"code": "wrong", "name": "X"}).StatusCode; code != 403 {
		t.Fatalf("bad code status %d", code)
	}
	a := join(t, base, m.LocalSession.Code(), "Ada")
	b := join(t, base, m.LocalSession.Code(), "Bob")
	streamA := openEvents(t, base, a)
	defer streamA.Body.Close()
	streamB := openEvents(t, base, b)
	defer streamB.Body.Close()
	for _, tc := range []struct{ token, text string }{{a, "one"}, {b, "two"}} {
		resp := postJSON(t, base+"/messages", tc.token, map[string]string{"text": tc.text})
		if resp.StatusCode != 200 {
			t.Fatalf("send: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
	for _, resp := range []*http.Response{streamA, streamB} {
		reader := bufio.NewReader(resp.Body)
		found := []string{}
		for len(found) < 2 {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"message"`) {
				found = append(found, line)
			}
		}
		if !strings.Contains(found[0], `"text":"one"`) || !strings.Contains(found[1], `"text":"two"`) {
			t.Fatalf("events: %v", found)
		}
	}
	resp := postJSON(t, base+"/end", "bad", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("bad host token: %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = postJSON(t, base+"/end", m.HostToken, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("end: %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = postJSON(t, base+"/messages", a, map[string]string{"text": "late"})
	if resp.StatusCode != 410 {
		t.Fatalf("send after end: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestLocalHTTPSHostBrowserNeedsNoCA(t *testing.T) {
	m := main.NewMain()
	m.LocalMode = true
	m.LocalHTTPS = true
	m.AdvertiseIP = "127.0.0.1"
	m.HTTPServer.Addr = "127.0.0.1:0"
	m.BootstrapAddr = "127.0.0.1:0"
	m.BrowserAddr = "127.0.0.1:0"
	m.CertDir = t.TempDir()
	if err := m.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if !strings.HasPrefix(m.BrowserURL, "http://localhost:") {
		t.Fatalf("host browser URL %q", m.BrowserURL)
	}
	resp, err := http.Get(m.BrowserURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("host browser app status %d", resp.StatusCode)
	}
	token := join(t, m.BrowserURL+"api/local/v1", m.LocalSession.Code(), "Host")
	if token == "" {
		t.Fatal("host browser join failed")
	}
}

func join(t *testing.T, base, code, name string) string {
	t.Helper()
	resp := postJSON(t, base+"/join", "", map[string]string{"code": code, "name": name})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("join %s: %d", name, resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Token
}

func postJSON(t *testing.T, url, token string, body any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func openEvents(t *testing.T, base, token string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/events", base), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("events: %d", resp.StatusCode)
	}
	return resp
}
