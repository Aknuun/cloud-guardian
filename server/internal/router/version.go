package router

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

const repo = "Aknuun/cloud-guardian"

// Public half of the release signing key (ed25519). The private key never ships;
// scripts/release.sh signs install-server.sh with it and the signature is
// published next to the script in the same tag.
const updatePubKeyHex = "d27f5aa6f107a2c6c86c93eb4a024e35527d3c72798e775497aeca0d88967628"

func updateScriptURL(tag, file string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, tag, file)
}

var tagRe = regexp.MustCompile(`^v?\d+(\.\d+){1,2}$`)

// latestVersion reads server/VERSION from the default branch (cheap, no API limits).
func latestVersion() (string, error) {
	hc := &http.Client{Timeout: 12 * time.Second}
	urls := []string{
		fmt.Sprintf("https://raw.githubusercontent.com/%s/main/server/VERSION", repo),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/master/server/VERSION", repo),
	}
	for _, u := range urls {
		resp, err := hc.Get(u)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v, nil
			}
		}
	}
	return "", fmt.Errorf("unreachable")
}

func verParts(s string) []int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if s == "" {
		return nil
	}
	var out []int
	for _, p := range strings.Split(s, ".") {
		n := 0
		if p == "" {
			return nil
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return nil
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}

// verGreater reports whether a is a strictly newer version than b.
func verGreater(a, b string) bool {
	pa, pb := verParts(a), verParts(b)
	if pa == nil || pb == nil {
		return false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func ghGet(path string, out any) error {
	req, err := http.NewRequest("GET", "https://api.github.com/repos/"+repo+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cg-server-updater")
	hc := &http.Client{Timeout: 15 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("github %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// versionTags returns up to 30 distinct version tags (newest first) from
// GitHub tags + stable releases.
func versionTags() ([]string, error) {
	seen := map[string]bool{}
	var add = func(v string) {
		v = strings.TrimSpace(v)
		if v != "" && tagRe.MatchString(v) {
			seen[v] = true
		}
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := ghGet("/tags?per_page=100", &tags); err == nil {
		for _, t := range tags {
			add(t.Name)
		}
	}
	var rels []struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := ghGet("/releases?per_page=30", &rels); err == nil {
		for _, r := range rels {
			if r.Draft || r.Prerelease {
				continue
			}
			add(r.Tag)
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("no versions")
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return verGreater(out[i], out[j]) })
	if len(out) > 30 {
		out = out[:30]
	}
	return out, nil
}

// latestTag returns the newest published version tag ("vX.Y.Z").
func latestTag() (string, error) {
	vs, err := versionTags()
	if err != nil {
		return "", err
	}
	return vs[0], nil
}

func (r *Router) curTag() string { return "v" + Version }

// ---------- UI ----------

func (r *Router) versHome(chat, mid int64) {
	latest := r.Store.Get("selfup_latest")
	line := "🆕 آخرین نسخه: <code>" + Esc(latest) + "</code>"
	if latest == "" {
		line = "🆕 آخرین نسخه: — (دکمهٔ «بررسی» را بزن)"
	}
	lines := []string{
		"🔄 <b>نسخه‌ها و آپدیت</b>", "",
		"🤖 نسخهٔ فعلی: <code>v" + Version + "</code>",
		line,
		"",
		"از «📋 فهرست نسخه‌ها» می‌توانی به هر نسخهٔ منتشرشده آپدیت یا برگردانی (rollback). آپدیت روی سرور خودت انجام و سرویس ری‌استارت می‌شود.",
	}
	kb := [][]Btn{
		{btnS("⬆️ آپدیت به آخرین نسخه", "verlatest", "primary")},
		{btn("📋 فهرست نسخه‌ها", "verlist")},
		{btn("🔙 بازگشت", "back:settings"), btn("🏠 خانه", "menu")},
	}
	if latest != "" && verGreater(latest, r.curTag()) {
		kb[0] = []Btn{btnS("⬆️ آپدیت به "+latest, "verdo:"+latest, "primary")}
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

// verLatest checks the published tag and offers an update if newer.
func (r *Router) verLatest(chat, mid int64) {
	r.Show(chat, mid, "⏳ در حال بررسی آخرین نسخه…", nil)
	tag, err := latestTag()
	if err != nil || tag == "" {
		r.Show(chat, mid, "❌ بررسی آخرین نسخه ممکن نشد (گیت‌هاب در دسترس نیست).",
			navKB("vers"))
		return
	}
	r.Store.Put("selfup_latest", tag, 0)
	if !verGreater(tag, r.curTag()) {
		r.Show(chat, mid, "✅ روی آخرین نسخه هستی (<code>"+r.curTag()+"</code>).",
			[][]Btn{{btn("📋 فهرست نسخه‌ها", "verlist")}, {btn("🔙 بازگشت", "back:vers"), btn("🏠 خانه", "menu")}})
		return
	}
	r.verDo(chat, mid, tag)
}

// updLater acknowledges a deferred update notice.
func (r *Router) updLater(chat, mid int64, tag string) {
	r.Show(chat, mid, "👍 باشه، بعداً.\nهر وقت خواستی از «⚙️ تنظیمات ← 🔄 نسخه‌ها» آپدیت کن.",
		[][]Btn{{btnS("🔄 نسخه‌ها", "vers", "primary"), btn("🏠 خانه", "menu")}})
}

// verList shows all published versions as install/rollback buttons.
func (r *Router) verList(chat, mid int64) {
	r.Show(chat, mid, "⏳ در حال گرفتن فهرست نسخه‌ها…", nil)
	vs, err := versionTags()
	if err != nil {
		r.Show(chat, mid, "❌ گرفتن فهرست نسخه‌ها ممکن نشد (گیت‌هاب در دسترس نیست).",
			navKB("vers"))
		return
	}
	r.Store.Put("selfup_latest", vs[0], 0)
	items := make([]Btn, 0, len(vs))
	cur := r.curTag()
	for _, v := range vs {
		mark := ""
		if v == cur {
			mark = "✅ "
		} else if verGreater(v, cur) {
			mark = "⬆️ "
		} else {
			mark = "⬇️ "
		}
		items = append(items, btnS(mark+v, "verdo:"+v, "plain"))
	}
	kb := gridBTN(items, 3)
	kb = append(kb, navRow("vers"))
	r.Show(chat, mid, fmt.Sprintf("📋 <b>نسخه‌ها</b>\n\nفعلی: <code>%s</code>\n⬆️ جدیدتر · ⬇️ قدیمی‌تر (برگشت). یکی را انتخاب کن:", cur), kb)
}

// verDo asks for confirmation before installing a specific version.
func (r *Router) verDo(chat, mid int64, tag string) {
	if !tagRe.MatchString(tag) {
		r.Show(chat, mid, "❌ نسخهٔ نامعتبر.", navKB("verlist"))
		return
	}
	word := "آپدیت"
	if verGreater(r.curTag(), tag) {
		word = "برگشت (rollback)"
	}
	r.Show(chat, mid,
		fmt.Sprintf("⚠️ %s به <code>%s</code>؟\n\nفعلی: <code>%s</code>\nربات چند لحظه قطع و دوباره وصل می‌شود.", word, Esc(tag), r.curTag()),
		[][]Btn{
			{btnS("✅ بله، انجام بده", "verdoYes:"+tag, "danger")},
			{btn("❌ انصراف", "verlist")},
		})
}

// verDoYes runs the updater detached so it survives the service restart.
func (r *Router) verDoYes(chat, mid int64, tag string) {
	if !tagRe.MatchString(tag) {
		r.Show(chat, mid, "❌ نسخهٔ نامعتبر.", navKB("verlist"))
		return
	}
	if err := startSelfUpdate(tag); err != nil {
		r.Show(chat, mid, "❌ شروع آپدیت ممکن نشد:\n"+Esc(err.Error()),
			navKB("vers"))
		return
	}
	r.Show(chat, mid,
		fmt.Sprintf("⏳ در حال آپدیت به <code>%s</code>…\nساخت و ری‌استارت ۱ تا ۳ دقیقه طول می‌کشد؛ ربات خودش برمی‌گردد.\nلاگ: <code>%s/update.log</code>", Esc(tag), installDir()),
		navKB(""))
}

func installDir() string {
	if d := strings.TrimSpace(os.Getenv("CG_INSTALL_DIR")); d != "" {
		return d
	}
	return "/opt/cloud-guardian"
}

// verifyScriptSig checks the detached ed25519 signature of the update script.
func verifyScriptSig(script []byte, sigB64, pubHex string) error {
	pub, err := hex.DecodeString(strings.TrimSpace(pubHex))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("کلید عمومی آپدیت نامعتبر است")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("امضای اسکریپت آپدیت نامعتبر است")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), script, sig) {
		return fmt.Errorf("امضای اسکریپت آپدیت معتبر نیست")
	}
	return nil
}

func fetchURL(hc *http.Client, url string) ([]byte, error) {
	resp, err := hc.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("دانلود ناموفق (%d)", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// downloadUpdateScript pulls the installer from the requested tag (immutable)
// and refuses to run it unless the signature matches the release key.
func downloadUpdateScript(tag string) ([]byte, error) {
	hc := &http.Client{Timeout: 30 * time.Second}
	script, err := fetchURL(hc, updateScriptURL(tag, "install-server.sh"))
	if err != nil {
		return nil, err
	}
	if os.Getenv("CG_ALLOW_UNSIGNED_UPDATE") == "1" {
		return script, nil
	}
	sig, err := fetchURL(hc, updateScriptURL(tag, "install-server.sh.sig"))
	if err != nil {
		return nil, fmt.Errorf("امضای آپدیت در دسترس نیست: %v", err)
	}
	if err := verifyScriptSig(script, string(sig), updatePubKeyHex); err != nil {
		return nil, err
	}
	return script, nil
}

func startSelfUpdate(tag string) error {
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	script, err := downloadUpdateScript(tag)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "cg-install-*.sh")
	if err != nil {
		return err
	}
	path := f.Name()
	if _, err := f.Write(script); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if err := os.Chmod(path, 0o700); err != nil {
		return err
	}
	logPath := installDir() + "/update.log"
	cmd := exec.Command("bash", "-lc", fmt.Sprintf(`REF=%q bash %q update >>%q 2>&1`, tag, path, logPath))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// CheckUpdate fetches the latest published tag and notifies admins once per
// new version. Safe to call periodically.
func (r *Router) CheckUpdate(bot interface {
	Send(int64, string, any) int64
}) {
	tag, err := latestTag()
	if err != nil || tag == "" {
		return
	}
	r.Store.Put("selfup_latest", tag, 0)
	r.Store.Put("selfup_latest_at", time.Now().UTC().Format(time.RFC3339), 0)
	if !verGreater(tag, r.curTag()) {
		return
	}
	if r.Store.Get("selfup_notified") == tag {
		return
	}
	r.Store.Put("selfup_notified", tag, 0)
	kb := [][]Btn{
		{btnS("⬆️ آپدیت به "+tag, "verdo:"+tag, "primary")},
		{btn("📋 فهرست نسخه‌ها", "verlist"), btn("⏸ بعداً", "updlater:"+tag)},
	}
	for _, a := range r.Admins() {
		bot.Send(a, fmt.Sprintf("🆕 نسخهٔ جدید <code>%s</code> منتشر شد (فعلی: <code>%s</code>).\nآپدیت کنم؟", Esc(tag), r.curTag()), kb)
	}
}
