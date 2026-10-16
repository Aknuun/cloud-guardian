package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aknuun/cg-server/internal/alerts"
	"github.com/aknuun/cg-server/internal/config"
	"github.com/aknuun/cg-server/internal/fosync"
	"github.com/aknuun/cg-server/internal/hostfilter"
	"github.com/aknuun/cg-server/internal/ndmon"
	"github.com/aknuun/cg-server/internal/pg"
	"github.com/aknuun/cg-server/internal/router"
	"github.com/aknuun/cg-server/internal/srvmon"
	"github.com/aknuun/cg-server/internal/sshop"
	"github.com/aknuun/cg-server/internal/store"
	"github.com/aknuun/cg-server/internal/tg"
	"github.com/aknuun/cg-server/internal/um"
)

func main() {
	cfgPath := getenv("CG_CONFIG", "config.json")
	dbPath := getenv("CG_DB", "data.db")
	healthAddr := getenv("CG_HEALTH", ":8787")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	bot := tg.New(cfg.BotToken)
	if me, err := bot.GetMe(); err != nil {
		log.Fatalf("getMe: %v", err)
	} else {
		log.Printf("bot online: %v", me["username"])
		if u, ok := me["username"].(string); ok && u != "" {
			st.Put("bot_username", "@"+u, 0)
		}
	}
	bot.DeleteWebhook()
	bot.SetChatMenuButton(true)
	if err := bot.SetMyCommands(router.BotCommands()); err != nil {
		log.Printf("setMyCommands: %v", err)
	}

	r := router.New(bot, st, cfg.AdminID, cfg.FailoverMap())
	r.CFToken = cfg.CFToken
	r.DBPath = dbPath

	// health + webhook endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "version": router.Version, "ts": time.Now().Unix(),
		})
	})
	mux.HandleFunc("/pghook/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != "POST" {
			http.Error(w, `{"ok":false}`, 405)
			return
		}
		secret := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		var payload any
		body, _ := io.ReadAll(io.LimitReader(req.Body, 1<<20))
		if len(body) > 0 {
			if json.Unmarshal(body, &payload) != nil {
				http.Error(w, `{"ok":false}`, 400)
				return
			}
		}
		panels := r.PanelsRaw()
		acfg := alerts.GetCfg(st)
		var hit *pg.Panel
		for i := range panels {
			if pc := acfg.Panels[panels[i].URL]; pc != nil && pc.Secret == secret {
				hit = &panels[i]
				break
			}
		}
		if hit == nil {
			http.Error(w, `{"ok":false}`, 403)
			return
		}
		n := alerts.HandleEvents(bot, st, *hit, hit.URL, payload, cfg.AdminID)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "buffered": n})
	})
	mux.HandleFunc("/ndhook/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != "POST" {
			http.Error(w, `{"ok":false}`, 405)
			return
		}
		tok := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		body, _ := io.ReadAll(io.LimitReader(req.Body, 1<<20))
		var payload map[string]any
		if len(body) > 0 {
			if json.Unmarshal(body, &payload) != nil {
				http.Error(w, `{"ok":false}`, 400)
				return
			}
		}
		var hit *ndmon.Monitor
		for _, m := range ndmon.Monitors(st) {
			if m.HookToken != "" && m.HookToken == tok {
				mm := m
				hit = &mm
				break
			}
		}
		if hit == nil {
			http.Error(w, `{"ok":false}`, 403)
			return
		}
		ok := ndmon.HandleNodeHook(bot, *hit, payload, cfg.AdminID)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": ok})
	})
	mux.HandleFunc("/hub/telemetry/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != "POST" {
			http.Error(w, `{"ok":false}`, 405)
			return
		}
		secret := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		var rec map[string]any
		body, _ := io.ReadAll(io.LimitReader(req.Body, 1<<20))
		if len(body) == 0 || json.Unmarshal(body, &rec) != nil {
			http.Error(w, `{"ok":false}`, 400)
			return
		}
		if !r.HubIngest(secret, rec) {
			http.Error(w, `{"ok":false}`, 403)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/hub/installs/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		secret := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		list, ok := r.HubInstallsFor(secret)
		if !ok {
			http.Error(w, `{"ok":false}`, 403)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "installs": list})
	})
	go func() {
		if err := http.ListenAndServe(healthAddr, mux); err != nil {
			log.Printf("health: %v", err)
		}
	}()
	go reminderLoop(r)
	go alertLoop(r, st, bot, cfg.AdminID, dbPath)
	go hostfilterLoop(r, st, bot, cfg.AdminID)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-stop; log.Println("shutting down"); os.Exit(0) }()

	log.Printf("polling started (%s)", router.Version)
	var offset int64
	for {
		ups, err := bot.GetUpdates(offset, 50)
		if err != nil {
			log.Printf("getUpdates: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, u := range ups {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			r.Handle(u)
		}
	}
}

func reminderLoop(r *router.Router) {
	for {
		r.FireDue()
		time.Sleep(30 * time.Second)
	}
}

func alertLoop(r *router.Router, st *store.Store, bot *tg.Client, adminID int64, dbPath string) {
	lastDigestU := time.Now()
	lastDigestD := time.Now()
	lastPurge := time.Now()
	lastSync := time.Now()
	lastBackup := time.Now()
	lastDomexp := time.Now()
	lastTelemetry := time.Now()
	for {
		time.Sleep(60 * time.Second)
		now := time.Now()
		panels := r.PanelsRaw()
		pmap := map[string]pg.Panel{}
		for _, p := range panels {
			pmap[p.URL] = p
		}
		if qm := r.QuotaAlert(); qm != "" {
			for _, t := range r.Admins() {
				bot.Send(t, qm, nil)
			}
		}
		if !r.QuotaPaused() {
			alerts.FlushBuffers(bot, st, pmap, adminID, false)
			um.Run(bot, st, panels, adminID)
			ndmon.RunNodePoll(bot, st, panels, adminID, false)
			srvmon.Run(st, r.SrvList(), adminID, srvStats, bot, false)
		}
		if now.Sub(lastDigestU) > time.Hour {
			alerts.RunDigest(bot, st, panels, "usage", adminID)
			lastDigestU = now
		}
		if now.Sub(lastDigestD) > 24*time.Hour {
			alerts.RunDigest(bot, st, panels, "days", adminID)
			lastDigestD = now
		}
		if now.Sub(lastDomexp) > 24*time.Hour {
			r.DomexpDaily()
			lastDomexp = now
		}
		r.HfdTick()
		if now.Sub(lastPurge) > time.Hour {
			st.Purge()
			lastPurge = now
		}
		if now.Sub(lastSync) > 10*time.Minute {
			r.SyncPush()
			lastSync = now
		}
		if now.Sub(lastBackup) > 30*time.Minute {
			tok, acc, ns := r.SyncCredentials()
			if tok != "" && acc != "" && ns != "" {
				path := dbPath
				fosync.PushAsync(func() { fosync.BackupDB(tok, acc, ns, path) })
			}
			lastBackup = now
		}
		if now.Sub(lastTelemetry) > 6*time.Hour {
			r.HubTelemetry()
			lastTelemetry = now
		}
	}
}
func srvStats(s map[string]any) map[string]any {
	host, _ := s["host"].(string)
	user, _ := s["user"].(string)
	if user == "" {
		user = "root"
	}
	secret, _ := s["secret"].(string)
	port := 22
	switch p := s["port"].(type) {
	case float64:
		port = int(p)
	case int:
		port = p
	}
	if port == 0 {
		port = 22
	}
	return sshop.Stats(host, user, secret, port)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func hostfilterLoop(r *router.Router, st *store.Store, bot *tg.Client, adminID int64) {
	for {
		time.Sleep(5 * time.Minute)
		if r.QuotaPaused() {
			continue
		}
		accs := r.CFAccounts()
		hostfilter.Run(bot, st, r.PanelsRaw(), adminID, accs, false, "")
	}
}
