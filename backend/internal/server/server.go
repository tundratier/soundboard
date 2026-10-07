// Package server liefert die Board-Ansicht (öffentlich im Netz) und die Admin-Oberfläche (nur localhost),
// dazu REST- und WebSocket-Schnittstellen.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"

	"soundboard/internal/audio"
	"soundboard/internal/library"
	"soundboard/internal/store"
)

// Platform kapselt Betriebssystem-Funktionen, damit der Server plattformneutral bleibt (und testbar ist).
type Platform interface {
	// SetDefaultDevice macht ein Gerät zum Standard. role: "communications" oder "all".
	SetDefaultDevice(deviceID, role string) error
}

type Deps struct {
	Engine   *audio.Engine
	Lib      *library.Library
	Store    *store.Store
	IconsDir string
	Web      fs.FS // Frontend-Build (index.html im Wurzelverzeichnis)
	Platform Platform
	DataDir  string
	Version  string
}

type Server struct {
	d   Deps
	hub *hub
	mux *http.ServeMux
	srv *http.Server
}

func New(d Deps) *Server {
	s := &Server{d: d, hub: newHub(), mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler liefert den HTTP-Handler (für Tests und httptest).
func (s *Server) Handler() http.Handler { return s.mux }

// Broadcast sendet ein Ereignis an alle WebSocket-Clients.
func (s *Server) Broadcast(v any) { s.hub.broadcast(v) }

// Run verteilt Engine-Ereignisse und startet den HTTP-Server. Blockiert bis Shutdown.
func (s *Server) Run() error {
	cfg := s.d.Store.Get().Server
	host := "127.0.0.1"
	if cfg.BindLAN {
		host = ""
	}
	addr := net.JoinHostPort(host, fmt.Sprint(cfg.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Port %d belegt? %w", cfg.Port, err)
	}
	s.srv = &http.Server{Handler: s.mux, ReadHeaderTimeout: 10 * time.Second}
	go s.pumpEvents()
	log.Printf("server: lauscht auf %s", ln.Addr())
	if err := s.srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) {
	if s.srv != nil {
		s.srv.Shutdown(ctx)
	}
	s.hub.closeAll()
}

func (s *Server) pumpEvents() {
	for ev := range s.d.Engine.Events() {
		s.hub.broadcast(ev)
	}
}

// ---------- Routing ----------

func (s *Server) routes() {
	m := s.mux
	// öffentlich (Board im Netzwerk)
	m.HandleFunc("GET /api/board", s.getBoard)
	m.HandleFunc("POST /api/trigger/{tile}", s.postTrigger)
	m.HandleFunc("POST /api/stop/{tile}", s.postStop)
	m.HandleFunc("POST /api/stopall", s.postStopAll)
	m.HandleFunc("GET /ws", s.ws)
	m.HandleFunc("GET /icons/{name}", s.getIcon)

	// Admin: nur localhost
	a := s.admin
	m.HandleFunc("GET /api/admin/state", a(s.adminState))
	m.HandleFunc("GET /api/admin/status", a(s.adminStatus))
	m.HandleFunc("PUT /api/admin/audio", a(s.putAudio))
	m.HandleFunc("PUT /api/admin/board", a(s.putBoard))
	m.HandleFunc("PUT /api/admin/server", a(s.putServer))
	m.HandleFunc("POST /api/admin/sounds", a(s.uploadSounds))
	m.HandleFunc("DELETE /api/admin/sounds/{name}", a(s.deleteSound))
	m.HandleFunc("GET /api/admin/sounds/{name}/file", a(s.getSoundFile))
	m.HandleFunc("POST /api/admin/preview", a(s.postPreview))
	m.HandleFunc("POST /api/admin/preview/stop", a(s.postPreviewStop))
	m.HandleFunc("POST /api/admin/icons", a(s.uploadIcon))
	m.HandleFunc("DELETE /api/admin/icons/{name}", a(s.deleteIcon))
	m.HandleFunc("POST /api/admin/default-device", a(s.postDefaultDevice))
	m.HandleFunc("GET /api/admin/qr", a(s.getQR))
	m.HandleFunc("GET /admin", a(s.index))
	m.HandleFunc("GET /admin/", a(s.index))

	m.HandleFunc("GET /", s.static)
}

// admin schützt Handler: nur Loopback-Clients, nur localhost-Host-Header (gegen DNS-Rebinding),
// und mutierende Requests brauchen einen eigenen Header (verhindert CSRF aus fremden Webseiten).
func (s *Server) admin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
			httpErr(w, 403, "Admin ist nur auf diesem Rechner (localhost) erreichbar")
			return
		}
		hh := r.Host
		if h, _, err := net.SplitHostPort(hh); err == nil {
			hh = h
		}
		switch strings.Trim(strings.ToLower(hh), "[]") {
		case "localhost", "127.0.0.1", "::1":
		default:
			httpErr(w, 403, "Admin nur über http://localhost:<Port>/admin erreichbar")
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-Soundboard") == "" {
			httpErr(w, 403, "X-Soundboard-Header fehlt")
			return
		}
		h(w, r)
	}
}

// ---------- Hilfsfunktionen ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		httpErr(w, 400, "ungültiges JSON: "+err.Error())
		return false
	}
	return true
}

// ---------- Öffentliche Endpunkte ----------

type boardResp struct {
	Board   store.Board `json:"board"`
	Playing []string    `json:"playing"`
	Missing []string    `json:"missing"` // Kachel-IDs, deren Sound fehlt oder defekt ist
}

func (s *Server) boardResp() boardResp {
	cfg := s.d.Store.Get()
	resp := boardResp{Board: cfg.Board, Playing: s.d.Engine.Playing(), Missing: []string{}}
	for _, t := range cfg.Board.Tiles {
		if s.d.Lib.Get(t.Sound) == nil {
			resp.Missing = append(resp.Missing, t.ID)
		}
	}
	return resp
}

func (s *Server) getBoard(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.boardResp())
}

// trigger löst eine Kachel aus; ok=false bei unbekannter Kachel.
func (s *Server) trigger(id string) bool {
	t, ok := s.d.Store.Get().Tile(id)
	if !ok {
		return false
	}
	s.d.Engine.Trigger(t.ID, s.d.Lib.Get(t.Sound), t.Gain, t.Pan, t.Mode)
	return true
}

func (s *Server) postTrigger(w http.ResponseWriter, r *http.Request) {
	if !s.trigger(r.PathValue("tile")) {
		httpErr(w, 404, "Kachel unbekannt")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postStop(w http.ResponseWriter, r *http.Request) {
	s.d.Engine.Stop(r.PathValue("tile"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postStopAll(w http.ResponseWriter, r *http.Request) {
	s.d.Engine.StopAll()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getIcon(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name != library.SanitizeName(name) || !iconExt(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox") // SVG darf keine Skripte ausführen
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, filepath.Join(s.d.IconsDir, name))
}

func iconExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return true
	}
	return false
}

// ---------- Statische Dateien ----------

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, r, "index.html")
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if p == "" {
		p = "index.html"
	}
	s.serveFile(w, r, p)
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	f, err := s.d.Web.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if name == "index.html" || name == "sw.js" || name == "manifest.webmanifest" {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, name, st.ModTime(), rs)
		return
	}
	io.Copy(w, f)
}

// ---------- Admin-Endpunkte ----------

type cableInfo struct {
	Found    bool   `json:"found"`
	InputID  string `json:"inputId,omitempty"`  // "CABLE Input" (Wiedergabe) → Main-Ausgang dieser App
	OutputID string `json:"outputId,omitempty"` // "CABLE Output" (Aufnahme) → Mikrofon für Discord & Co.
}

func detectCable(devs []audio.DeviceInfo) cableInfo {
	var c cableInfo
	for _, d := range devs {
		n := strings.ToLower(d.Name)
		if d.Kind == audio.KindPlayback && strings.Contains(n, "cable input") {
			c.InputID = d.ID
		}
		if d.Kind == audio.KindCapture && strings.Contains(n, "cable output") {
			c.OutputID = d.ID
		}
	}
	c.Found = c.InputID != "" && c.OutputID != ""
	return c
}

// LANURLs liefert die URLs, unter denen das Board im Netzwerk erreichbar ist.
func LANURLs(port int) []string {
	var out []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() && !ipn.IP.IsLinkLocalUnicast() {
			out = append(out, fmt.Sprintf("http://%s:%d/", ipn.IP, port))
		}
	}
	sort.Strings(out)
	return out
}

type stateResp struct {
	Config   store.Config       `json:"config"`
	Devices  []audio.DeviceInfo `json:"devices"`
	Status   audio.Status       `json:"status"`
	Sounds   []library.Info     `json:"sounds"`
	Icons    []string           `json:"icons"`
	Cable    cableInfo          `json:"cable"`
	URLs     []string           `json:"urls"`
	DataDir  string             `json:"dataDir"`
	Version  string             `json:"version"`
	Playing  []string           `json:"playing"`
	DevError string             `json:"devicesError,omitempty"`
}

func (s *Server) listIcons() []string {
	out := []string{}
	ents, _ := os.ReadDir(s.d.IconsDir)
	for _, e := range ents {
		if !e.IsDir() && iconExt(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func (s *Server) adminState(w http.ResponseWriter, r *http.Request) {
	cfg := s.d.Store.Get()
	devs, err := s.d.Engine.Devices()
	resp := stateResp{
		Config: cfg, Devices: devs, Status: s.d.Engine.Status(), Sounds: s.d.Lib.List(),
		Icons: s.listIcons(), Cable: detectCable(devs), URLs: LANURLs(cfg.Server.Port),
		DataDir: s.d.DataDir, Version: s.d.Version, Playing: s.d.Engine.Playing(),
	}
	if err != nil {
		resp.DevError = err.Error()
	}
	if resp.Devices == nil {
		resp.Devices = []audio.DeviceInfo{}
	}
	writeJSON(w, 200, resp)
}

func (s *Server) adminStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.d.Engine.Status())
}

func (s *Server) putAudio(w http.ResponseWriter, r *http.Request) {
	var a audio.Settings
	if !readJSON(w, r, &a) {
		return
	}
	cfg, err := s.d.Store.Update(func(c *store.Config) error { c.Audio = a; return nil })
	if err != nil {
		httpErr(w, 400, err.Error())
		return
	}
	s.d.Engine.Apply(cfg.Audio)
	writeJSON(w, 200, cfg.Audio)
}

func (s *Server) putBoard(w http.ResponseWriter, r *http.Request) {
	var b store.Board
	if !readJSON(w, r, &b) {
		return
	}
	if b.Tiles == nil {
		b.Tiles = []store.Tile{}
	}
	cfg, err := s.d.Store.Update(func(c *store.Config) error { c.Board = b; return nil })
	if err != nil {
		httpErr(w, 400, err.Error())
		return
	}
	s.hub.broadcast(map[string]string{"type": "board"})
	writeJSON(w, 200, cfg.Board)
}

func (s *Server) putServer(w http.ResponseWriter, r *http.Request) {
	var sv store.Server
	if !readJSON(w, r, &sv) {
		return
	}
	cfg, err := s.d.Store.Update(func(c *store.Config) error { c.Server = sv; return nil })
	if err != nil {
		httpErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"server": cfg.Server, "restartRequired": true})
}

const (
	maxSoundBytes = 100 << 20
	maxIconBytes  = 2 << 20
)

// uploadFiles liest multipart-Dateien (Feld "file") und übergibt sie an save.
func uploadFiles(w http.ResponseWriter, r *http.Request, limit int64, save func(name string, r io.Reader) (string, error)) {
	r.Body = http.MaxBytesReader(w, r.Body, limit*20)
	mr, err := r.MultipartReader()
	if err != nil {
		httpErr(w, 400, "multipart erwartet")
		return
	}
	saved := []string{}
	errs := []string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, err.Error())
			break
		}
		if part.FileName() == "" {
			continue
		}
		name, err := save(part.FileName(), io.LimitReader(part, limit))
		if err != nil {
			errs = append(errs, part.FileName()+": "+err.Error())
			continue
		}
		saved = append(saved, name)
	}
	code := 200
	if len(saved) == 0 && len(errs) > 0 {
		code = 400
	}
	writeJSON(w, code, map[string]any{"saved": saved, "errors": errs})
}

func (s *Server) uploadSounds(w http.ResponseWriter, r *http.Request) {
	uploadFiles(w, r, maxSoundBytes, s.d.Lib.Save)
}

func (s *Server) deleteSound(w http.ResponseWriter, r *http.Request) {
	if err := s.d.Lib.Delete(r.PathValue("name")); err != nil {
		httpErr(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getSoundFile liefert die Originaldatei (Vorhören im Browser ohne Audio-Engine).
func (s *Server) getSoundFile(w http.ResponseWriter, r *http.Request) {
	p := s.d.Lib.Path(r.PathValue("name"))
	if p == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, p)
}

func (s *Server) postPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sound string  `json:"sound"`
		Gain  float32 `json:"gain"`
		Pan   float32 `json:"pan"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	snd := s.d.Lib.Get(req.Sound)
	if snd == nil {
		httpErr(w, 404, "Sound nicht gefunden oder defekt")
		return
	}
	if req.Gain == 0 {
		req.Gain = 1
	}
	s.d.Engine.Preview(snd, req.Gain, req.Pan)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postPreviewStop(w http.ResponseWriter, r *http.Request) {
	s.d.Engine.StopPreview()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) uploadIcon(w http.ResponseWriter, r *http.Request) {
	os.MkdirAll(s.d.IconsDir, 0o755)
	uploadFiles(w, r, maxIconBytes, func(name string, rd io.Reader) (string, error) {
		name = library.SanitizeName(name)
		if name == "" || !iconExt(name) {
			return "", errors.New("Dateityp nicht unterstützt (png, jpg, gif, webp, svg)")
		}
		f, err := os.CreateTemp(s.d.IconsDir, ".icon-*.part")
		if err != nil {
			return "", err
		}
		_, err = io.Copy(f, rd)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(f.Name())
			return "", err
		}
		// bestehende Datei gleichen Namens wird ersetzt (Icon-Austausch)
		if err := os.Rename(f.Name(), filepath.Join(s.d.IconsDir, name)); err != nil {
			os.Remove(f.Name())
			return "", err
		}
		return name, nil
	})
}

func (s *Server) deleteIcon(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name != library.SanitizeName(name) || !iconExt(name) {
		httpErr(w, 400, "ungültiger Name")
		return
	}
	if err := os.Remove(filepath.Join(s.d.IconsDir, name)); err != nil && !os.IsNotExist(err) {
		httpErr(w, 500, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postDefaultDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"deviceId"`
		Role     string `json:"role"` // "communications" (Standard) | "all"
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Role == "" {
		req.Role = "communications"
	}
	if req.Role != "communications" && req.Role != "all" {
		httpErr(w, 400, "role: communications oder all")
		return
	}
	devs, _ := s.d.Engine.Devices()
	known := false
	for _, d := range devs {
		if d.ID == req.DeviceID {
			known = true
		}
	}
	if !known {
		httpErr(w, 404, "Gerät nicht gefunden")
		return
	}
	if s.d.Platform == nil {
		httpErr(w, 501, "auf dieser Plattform nicht verfügbar")
		return
	}
	if err := s.d.Platform.SetDefaultDevice(req.DeviceID, req.Role); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getQR(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	if text == "" || len(text) > 300 {
		httpErr(w, 400, "text fehlt oder zu lang")
		return
	}
	png, err := qrcode.Encode(text, qrcode.Medium, 256)
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
