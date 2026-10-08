package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"soundboard/internal/audio"
	"soundboard/internal/library"
	"soundboard/internal/store"
)

type fakeStream struct{}

func (fakeStream) Close() {}

type fakeBackend struct {
	mu    sync.Mutex
	fills []func([]float32)
}

func (f *fakeBackend) Devices() ([]audio.DeviceInfo, error) {
	return []audio.DeviceInfo{
		{ID: "spk", Name: "Lautsprecher", Kind: audio.KindPlayback, IsDefault: true},
		{ID: "ci", Name: "CABLE Input (VB-Audio Virtual Cable)", Kind: audio.KindPlayback},
		{ID: "co", Name: "CABLE Output (VB-Audio Virtual Cable)", Kind: audio.KindCapture},
	}, nil
}
func (f *fakeBackend) OpenCapture(string, int, func([]float32), func()) (audio.Stream, error) {
	return fakeStream{}, nil
}
func (f *fakeBackend) OpenPlayback(_ string, _ int, fill func([]float32), _ func()) (audio.Stream, error) {
	f.mu.Lock()
	f.fills = append(f.fills, fill)
	f.mu.Unlock()
	return fakeStream{}, nil
}
func (f *fakeBackend) Close() {}

type fakePlatform struct{ id, role string }

func (p *fakePlatform) SetDefaultDevice(id, role string) error { p.id, p.role = id, role; return nil }

type env struct {
	srv  *httptest.Server
	h    http.Handler
	eng  *audio.Engine
	st   *store.Store
	lib  *library.Library
	fb   *fakeBackend
	plat *fakePlatform
	dir  string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	lib, _ := library.New(filepath.Join(dir, "sounds"), nil)
	if err := library.WriteTestTone(filepath.Join(dir, "sounds", "ton.wav")); err != nil {
		t.Fatal(err)
	}
	lib.Scan()
	st, _, _ := store.Open(filepath.Join(dir, "config.json"))
	st.Update(func(c *store.Config) error {
		c.Boards[0].Tiles = []store.Tile{{ID: "t1", X: 0, Y: 0, Sound: "ton.wav", Gain: 1, Mode: audio.ModeOneShot}, {ID: "t2", X: 1, Y: 0, Sound: "weg.wav", Gain: 1}}
		return nil
	})
	fb := &fakeBackend{}
	eng := audio.NewEngine(fb)
	eng.ApplyNow(audio.Settings{MainID: "ci", MonitorID: "spk", MicGain: 1, FxGainMain: 1, FxGainMonitor: 1, PeriodMs: 10})
	plat := &fakePlatform{}
	web := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}, "assets/a.js": {Data: []byte("x")}}
	s := New(Deps{Engine: eng, Lib: lib, Store: st, IconsDir: filepath.Join(dir, "icons"), Web: web, Platform: plat, DataDir: dir})
	go s.pumpEvents()
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); eng.Close() })
	return &env{srv: ts, h: s.Handler(), eng: eng, st: st, lib: lib, fb: fb, plat: plat, dir: dir}
}

// adminReq calls the handler directly so RemoteAddr/Host can be controlled.
func (e *env) adminReq(method, url string, body io.Reader, mut ...func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, body)
	r.RemoteAddr = "127.0.0.1:5555"
	r.Host = "localhost:8723"
	r.Header.Set("X-Soundboard", "1")
	for _, m := range mut {
		m(r)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func TestAdminGuard(t *testing.T) {
	e := newEnv(t)
	// Foreign IP
	if w := e.adminReq("GET", "/api/admin/state", nil, func(r *http.Request) { r.RemoteAddr = "192.168.1.50:4000" }); w.Code != 403 {
		t.Errorf("LAN-Client: %d", w.Code)
	}
	// DNS rebinding: loopback IP, but foreign Host
	if w := e.adminReq("GET", "/api/admin/state", nil, func(r *http.Request) { r.Host = "evil.example:8723" }); w.Code != 403 {
		t.Errorf("fremder Host: %d", w.Code)
	}
	// CSRF: mutating request without header
	if w := e.adminReq("PUT", "/api/admin/boards", strings.NewReader("[]"), func(r *http.Request) { r.Header.Del("X-Soundboard") }); w.Code != 403 {
		t.Errorf("ohne Header: %d", w.Code)
	}
	if w := e.adminReq("GET", "/admin", nil, func(r *http.Request) { r.RemoteAddr = "10.0.0.2:1" }); w.Code != 403 {
		t.Errorf("/admin aus dem LAN: %d", w.Code)
	}
	if w := e.adminReq("GET", "/admin", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "app") {
		t.Errorf("/admin lokal: %d", w.Code)
	}
	// Board is open
	resp, err := http.Get(e.srv.URL + "/api/boards/main")
	if err != nil || resp.StatusCode != 200 {
		t.Fatal(err, resp)
	}
	resp, _ = http.Get(e.srv.URL + "/")
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "app") {
		t.Errorf("Index: %s", b)
	}
	if resp, _ = http.Get(e.srv.URL + "/../config.json"); resp.StatusCode == 200 {
		t.Error("Pfad-Traversal")
	}
}

func TestBoardTriggerAndMissing(t *testing.T) {
	e := newEnv(t)
	resp, _ := http.Get(e.srv.URL + "/api/boards/main")
	var br boardResp
	json.NewDecoder(resp.Body).Decode(&br)
	if len(br.Board.Tiles) != 2 || len(br.Missing) != 1 || br.Missing[0] != "t2" {
		t.Fatalf("%+v", br)
	}
	resp, _ = http.Post(e.srv.URL+"/api/trigger/t1", "", nil)
	if resp.StatusCode != 204 {
		t.Fatal(resp.StatusCode)
	}
	resp, _ = http.Post(e.srv.URL+"/api/trigger/nix", "", nil)
	if resp.StatusCode != 404 {
		t.Fatal(resp.StatusCode)
	}
	time.Sleep(50 * time.Millisecond)
	if p := e.eng.Playing(); len(p) != 1 || p[0] != "t1" {
		t.Fatalf("playing %v", p)
	}
}

func TestWebSocketTrigger(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	read := func() map[string]any {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		json.Unmarshal(b, &m)
		return m
	}
	if m := read(); m["type"] != "hello" {
		t.Fatalf("%v", m)
	}
	c.Write(ctx, websocket.MessageText, []byte(`{"type":"trigger","tile":"t1"}`))
	if m := read(); m["type"] != "started" || m["tile"] != "t1" {
		t.Fatalf("%v", m)
	}
	// Pump the audio callbacks of both buses until the short tone ends → "stopped"
	e.fb.mu.Lock()
	fills := append([]func([]float32){}, e.fb.fills...)
	e.fb.mu.Unlock()
	out := make([]float32, 480*2)
	for i := 0; i < 100; i++ {
		for _, fill := range fills {
			fill(out)
		}
	}
	if m := read(); m["type"] != "stopped" || m["tile"] != "t1" {
		t.Fatalf("%v", m)
	}
}

func TestAdminUploadBoardAndDefaultDevice(t *testing.T) {
	e := newEnv(t)
	// Upload
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	tone := filepath.Join(e.dir, "up.wav")
	library.WriteTestTone(tone)
	f, _ := os.Open(tone)
	fw, _ := mw.CreateFormFile("file", "Neuer Sound.wav")
	io.Copy(fw, f)
	f.Close()
	fw, _ = mw.CreateFormFile("file", "boese.exe")
	fw.Write([]byte("MZ"))
	mw.Close()
	w := e.adminReq("POST", "/api/admin/sounds", &buf, func(r *http.Request) { r.Header.Set("Content-Type", mw.FormDataContentType()) })
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Neuer Sound.wav") || !strings.Contains(w.Body.String(), "boese.exe") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if e.lib.Get("Neuer Sound.wav") == nil {
		t.Fatal("Upload nicht geladen")
	}

	// Save boards: invalid → 400, valid → 200
	w = e.adminReq("PUT", "/api/admin/boards", strings.NewReader(`[{"name":"X","cols":0,"rows":2,"tiles":[]}]`))
	if w.Code != 400 {
		t.Fatalf("ungültiges Raster: %d", w.Code)
	}
	w = e.adminReq("PUT", "/api/admin/boards", strings.NewReader(`[]`))
	if w.Code != 400 {
		t.Fatalf("leere Board-Liste: %d", w.Code)
	}
	w = e.adminReq("PUT", "/api/admin/boards", strings.NewReader(`[{"name":"X","cols":5,"rows":2,"tiles":[{"x":0,"y":0,"sound":"ton.wav","pan":-1,"gain":1,"mode":"toggle"}]}]`))
	if b := e.st.Get().Boards[0]; w.Code != 200 || b.Cols != 5 || b.ID != "x" || b.Tiles[0].ID == "" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}

	// Default communications device
	w = e.adminReq("POST", "/api/admin/default-device", strings.NewReader(`{"deviceId":"co"}`))
	if w.Code != 204 || e.plat.id != "co" || e.plat.role != "communications" {
		t.Fatalf("%d %+v", w.Code, e.plat)
	}
	w = e.adminReq("POST", "/api/admin/default-device", strings.NewReader(`{"deviceId":"unbekannt"}`))
	if w.Code != 404 {
		t.Fatalf("%d", w.Code)
	}

	// State contains cable detection
	w = e.adminReq("GET", "/api/admin/state", nil)
	var st stateResp
	json.Unmarshal(w.Body.Bytes(), &st)
	if !st.Cable.Found || st.Cable.OutputID != "co" || st.Cable.InputID != "ci" || len(st.Sounds) < 2 {
		t.Fatalf("%+v", st.Cable)
	}

	// Audio settings
	w = e.adminReq("PUT", "/api/admin/audio", strings.NewReader(`{"mainDeviceId":"ci","micGain":9,"fxGainMain":1,"fxGainMonitor":1,"periodMs":10}`))
	if w.Code != 200 || e.st.Get().Audio.MicGain != 4 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}

	// Preview
	w = e.adminReq("POST", "/api/admin/preview", strings.NewReader(`{"sound":"ton.wav","gain":1,"pan":0}`))
	if w.Code != 204 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// QR
	w = e.adminReq("GET", "/api/admin/qr?text=http://192.168.1.2:8723/", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("%d", w.Code)
	}
}

func TestIconUploadAndServe(t *testing.T) {
	e := newEnv(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "../lampe.svg")
	fw.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	mw.Close()
	w := e.adminReq("POST", "/api/admin/icons", &buf, func(r *http.Request) { r.Header.Set("Content-Type", mw.FormDataContentType()) })
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	resp, _ := http.Get(e.srv.URL + "/icons/lampe.svg")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Security-Policy") != "sandbox" {
		t.Fatalf("%d %v", resp.StatusCode, resp.Header)
	}
	resp, _ = http.Get(e.srv.URL + "/icons/..%2Fconfig.json")
	if resp.StatusCode == 200 {
		t.Fatal("Traversal")
	}
}

func TestMultipleBoardsRoutes(t *testing.T) {
	e := newEnv(t)
	// a second board with its own tile; tile positions may repeat across boards
	w := e.adminReq("PUT", "/api/admin/boards", strings.NewReader(`[
		{"id":"main","name":"Soundboard","cols":4,"rows":3,"tiles":[{"id":"t1","x":0,"y":0,"sound":"ton.wav","gain":1,"mode":"oneshot"}]},
		{"name":"Zum Lachen","cols":2,"rows":2,"tiles":[{"id":"l1","x":0,"y":0,"sound":"ton.wav","gain":1,"color":"#ff0000","icon":{"kind":"emoji","value":"😂"}},{"id":"far","x":5,"y":5,"sound":"ton.wav","gain":1}]}
	]`))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}

	// overview: public, in order, with previews (tile outside the grid is left out)
	resp, err := http.Get(e.srv.URL + "/api/boards")
	if err != nil || resp.StatusCode != 200 {
		t.Fatal(err, resp)
	}
	var sums []boardSummary
	json.NewDecoder(resp.Body).Decode(&sums)
	if len(sums) != 2 || sums[0].ID != "main" || sums[1].ID != "zum-lachen" || sums[1].Name != "Zum Lachen" {
		t.Fatalf("%+v", sums)
	}
	if p := sums[1].Tiles; len(p) != 1 || p[0].Color != "#ff0000" || p[0].Icon == nil || p[0].Icon.Value != "😂" {
		t.Fatalf("preview: %+v", p)
	}

	// single board by ID
	resp, _ = http.Get(e.srv.URL + "/api/boards/zum-lachen")
	var br boardResp
	json.NewDecoder(resp.Body).Decode(&br)
	if resp.StatusCode != 200 || br.Board.Cols != 2 || len(br.Board.Tiles) != 2 {
		t.Fatalf("%d %+v", resp.StatusCode, br)
	}
	if resp, _ = http.Get(e.srv.URL + "/api/boards/gibt-es-nicht"); resp.StatusCode != 404 {
		t.Fatalf("unknown board: %d", resp.StatusCode)
	}

	// tiles of any board can be triggered by their (global) ID
	if resp, _ = http.Post(e.srv.URL+"/api/trigger/l1", "", nil); resp.StatusCode != 204 {
		t.Fatalf("trigger on board 2: %d", resp.StatusCode)
	}

	// pages: root redirects to the overview, board paths serve the app, assets stay reachable
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, _ = noRedirect.Get(e.srv.URL + "/")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/boards" {
		t.Fatalf("root: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	for _, p := range []string{"/boards", "/boards/", "/boards/zum-lachen", "/boards/gibt-es-nicht"} {
		resp, _ = http.Get(e.srv.URL + p)
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 || !strings.Contains(string(b), "app") {
			t.Errorf("%s: %d %s", p, resp.StatusCode, b)
		}
	}
	if resp, _ = http.Get(e.srv.URL + "/assets/a.js"); resp.StatusCode != 200 {
		t.Errorf("asset: %d", resp.StatusCode)
	}
}
