package server

import (
	"bytes"
	"hash/fnv"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lessmess/internal/model"
)

func htmlGet(t *testing.T, h http.Handler, path string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("Accept", "text/html")
	if hx {
		r.Header.Set("HX-Request", "true")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAssetsVersionStableAndSensitive(t *testing.T) {
	v1 := assetsVersion()
	v2 := assetsVersion()
	if v1 == "" || v1 != v2 {
		t.Fatalf("unstable or empty version: %q vs %q", v1, v2)
	}
	// Content sensitivity: hash of different content differs.
	h1 := fnv.New32a()
	h1.Write([]byte("a"))
	h2 := fnv.New32a()
	h2.Write([]byte("b"))
	if h1.Sum32() == h2.Sum32() {
		t.Fatal("fnv not content-sensitive in test")
	}
}

func TestIndexHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"<html", "lessmess", "Fixture change", "/changes/2026-09-10-0", "htmx.min.js", `id="chat-lifecycle"`, `id="chat-revert-panel"`, `id="chat-compact-btn"`, "atomic revision guard"} {
		if !strings.Contains(body, want) {
			t.Errorf("index HTML missing %q", want)
		}
	}
	optionsAt := strings.Index(body, `id="chat-more-btn"`)
	compactAt := strings.Index(body, `id="chat-compact-btn"`)
	menuEnd := -1
	if optionsAt >= 0 {
		menuEnd = strings.Index(body[optionsAt:], `</div>`)
	}
	if optionsAt < 0 || compactAt < optionsAt || menuEnd < 0 || compactAt > optionsAt+menuEnd {
		t.Errorf("compact action is not inside the options menu")
	}
}

func TestIndexSortableMarkup(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	// Six sortable data-column headers, each a sort button with an indicator.
	for _, col := range []string{"id", "title", "prefix", "status", "tasks", "updated"} {
		if !strings.Contains(body, `data-sort-col="`+col+`"`) {
			t.Errorf("index HTML missing sort button for column %q", col)
		}
	}
	if got := strings.Count(body, `class="sort-btn"`); got != 6 {
		t.Errorf("sort button count = %d, want 6", got)
	}
	// Rows carry machine-readable sort keys.
	for _, attr := range []string{"data-tasks=", "data-status-rank=", "data-updated="} {
		if !strings.Contains(body, attr) {
			t.Errorf("index rows missing %q sort key", attr)
		}
	}
	// The Plan button column stays a plain header (not sortable).
	if strings.Contains(body, `data-sort-col="plan"`) {
		t.Error("plan column unexpectedly sortable")
	}
}

func TestStatusRankTemplateFunc(t *testing.T) {
	// Workflow order: Not started(0) … Cancelled(5); unknown sorts last.
	tmpl := template.Must(template.New("t").Funcs(templateFuncs).Parse(
		`{{range .}}{{. | statusRank }} {{end}}`))
	statuses := []string{"Not started", "In progress", "Blocked", "Test", "Done", "Cancelled", "Weird"}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, statuses); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "0 1 2 3 4 5 6 "; got != want {
		t.Fatalf("statusRank output = %q, want %q", got, want)
	}
}

func TestIndexOnboardingBanner(t *testing.T) {
	st, dir := fixtureStore(t)
	h := New(st).Handler()

	// No onboarding state file: pending, banner shows.
	w := htmlGet(t, h, "/", false)
	if !strings.Contains(w.Body.String(), `id="onboarding-banner"`) {
		t.Error("banner missing while onboarding is pending")
	}

	// Dismissed: banner gone.
	if err := saveOnboarding(dir, onboardingState{Version: 1, Dismissed: true}); err != nil {
		t.Fatal(err)
	}
	w = htmlGet(t, h, "/", false)
	if strings.Contains(w.Body.String(), `id="onboarding-banner"`) {
		t.Error("banner present after dismiss")
	}

	// Completed: banner gone as well (and JSON is unaffected).
	if err := saveOnboarding(dir, onboardingState{Version: 1, CompletedAt: "2026-09-13T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	w = htmlGet(t, h, "/", false)
	if strings.Contains(w.Body.String(), `id="onboarding-banner"`) {
		t.Error("banner present after completion")
	}
	w = do(t, h, "GET", "/", "")
	if strings.Contains(w.Body.String(), "onboarding") {
		t.Error("JSON index must not carry onboarding state")
	}
}

func TestSettingsPageHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/settings", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`id="settings-page"`, `data-page="settings"`, "Prompt addenda",
		`class="settings-nav"`,
		`data-group="general"`, `data-group="session"`, `data-group="prompts"`, `data-group="git"`,
		`data-group="ui"`, `data-group="docs"`,
		`data-field="general.projectName"`,
		`data-field="session.agent"`, `data-field="session.model"`,
		`data-field="prompts.discussion"`, `data-field="prompts.change"`,
		`data-field="prompts.commit"`, `data-field="prompts.repoCommit"`,
		`data-field="prompts.gardener"`, `data-field="prompts.explorer"`,
		`data-field="git.defaultBranch"`, `data-field="ui.showArchived"`,
		`data-field="docs.autoGardenerOnClose"`, `class="settings-change"`,
		`name="settings-scope"`, "lessmess.json", ".lessmess/settings.json",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settings HTML missing %q", want)
		}
	}
	// The right-aligned top-menu link is present and active on the page.
	if !strings.Contains(body, `class="topnav topnav-right"`) {
		t.Error("topnav-right group missing")
	}
	if !strings.Contains(body, `<a href="/settings" class="active">Settings</a>`) {
		t.Error("active Settings nav link missing")
	}
	// The header Chat button sits beside Settings on app pages.
	if !strings.Contains(body, `id="chat-btn"`) {
		t.Error("header chat button missing")
	}
}

func TestSetupPageHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/setup", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`id="setup-page"`, `data-page="setup"`, "Set up lessmess",
		`data-step="prereqs"`, `data-step="name"`, `data-step="bootstrap"`, `data-step="agent"`,
		`data-step="docs"`, `data-step="finish"`,
		`data-step-nav="name"`,
		`id="setup-project-name"`, `id="setup-name-save"`, `id="setup-name-skip"`,
		`name="setup-name-scope"`,
		`id="setup-prereq-list"`, `id="setup-recheck-btn"`, `id="setup-prereqs-next"`,
		`id="setup-coverage"`, `id="setup-bootstrap-btn"`,
		`id="setup-exclude-list"`,
		`id="setup-agent"`, `id="setup-model"`, `name="setup-scope"`,
		`id="setup-seed-budget"`, `id="setup-seed-btn"`, `id="setup-seed-skip"`,
		`id="setup-seed-log"`, `id="setup-finish-btn"`, `id="setup-error"`,
		`id="setup-steps-nav"`, `id="setup-step-indicator"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("setup HTML missing %q", want)
		}
	}
	// Onboarding shows no other app chrome: no navs, docs bell, or banner.
	for _, absent := range []string{`class="topnav"`, `topnav-right`, `id="notif-bell"`, `id="banner"`, `>Changes</a>`, `>Explorer</a>`, `>Settings</a>`, `id="chat-btn"`} {
		if strings.Contains(body, absent) {
			t.Errorf("setup HTML must not contain app chrome %q", absent)
		}
	}
}

func TestBoardHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"Not started", "In progress", "Blocked", "Test", "Done", "Cancelled",
		"First", "Second", "data-task=\"FIX-00\"", `data-change="2026-09-10-0"`,
		`sessions-btn`, `sessions-panel`,
		`chat-overlay`, `chat-transcript`, `chat-composer`, `chat-interrupt-btn`,
		`chat-file-input`, `chat-draft-files`, `chat-reference-picker`, `chat-reference-list`,
		`chat-controls-btn`, `chat-controls-sheet`, `chat-controls-search`, `chat-agent-select`, `chat-model-select`, `chat-variant-select`,
		`chat-command-select`, `chat-command-args`, `chat-skill-list`, `chat-skill-chips`, `chat-usage`} {
		if !strings.Contains(body, want) {
			t.Errorf("board HTML missing %q", want)
		}
	}
	if strings.Count(body, `id="detail"`) != 1 || strings.Index(body, `id="detail"`) < strings.Index(body, `</main>`) {
		t.Errorf("detail host must appear once as a body-level overlay after main")
	}
}

func TestChatInboxUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{`id="chat-inbox"`, `id="chat-composer"`} {
		if !strings.Contains(page, want) {
			t.Errorf("chat page missing %q", want)
		}
	}
	for _, absent := range []string{`id="chat-delivery-controls"`, `id="chat-delivery-mode"`, `While OpenCode is working`, `Queue for next turn`, `Steer current turn`} {
		if strings.Contains(page, absent) {
			t.Errorf("chat page retained removed delivery control %q", absent)
		}
	}
	asset := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{"newChatMessageID", "promptDeliveryFiles", "promptDeliverySkills", `body.append("delivery", "queue")`, "inboxDelivery", "may have been delivered or cancelled", "Unsupported pending item"} {
		if !strings.Contains(asset, want) {
			t.Errorf("chat script missing %q", want)
		}
	}
	if strings.Contains(asset, "dataInboxEdit") {
		t.Fatal("pending inbox text editing must not be offered")
	}
}

func TestChatComposerUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{
		`id="chat-prompt" rows="2"`, `id="chat-send-btn" type="submit" class="btn-accent"`,
		`id="chat-more-btn"`, `aria-label="Chat options"`, `aria-haspopup="menu"`, `aria-expanded="false"`, `aria-controls="chat-more-menu">⋮</button>`,
		`id="chat-more-menu"`, `role="menu"`, `id="chat-more-backdrop"`, `aria-label="Close chat options"`,
		`id="chat-compact-btn" type="button" role="menuitem"`, `disabled>Compact</button>`,
		`id="chat-interrupt-btn" type="button" class="btn-ghost" hidden`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("chat composer missing %q", want)
		}
	}

	js := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`document.getElementById("chat-interrupt-btn").hidden = !busy`,
		`function closeChatMore(returnFocus)`, `moreBackdrop.addEventListener("click"`,
		`closeChatMessageActions(true) || closeChatMore(true) || closeChatReferences(true)`,
		`loadChatReferences().then`, `openChatControls(moreButton).then`,
		`var usage = cstate.usage || {}`, `setChatStatus("Compaction failed: " + err.message, true)`,
		`data-remove-chat-file`, `data-remove-chat-reference`, `data-remove-chat-skill`,
		`files.length + refs.length >= 10`, `20 * 1024 * 1024`, `promptDeliveryFiles`, `promptDeliverySkills`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("chat composer script missing %q", want)
		}
	}
	if strings.Contains(js, `!cstate.controls || !cstate.controls.usage.contextAvailable`) {
		t.Error("compact click still depends on opening Controls")
	}
	for _, removed := range []string{`chat-actions-btn`, `chat-actions-sheet`, `chat-actions-backdrop`, `closeChatActions`} {
		if strings.Contains(page, removed) || strings.Contains(js, removed) {
			t.Errorf("chat composer retained split action control %q", removed)
		}
	}
	for _, unrequested := range []string{`class="chat-add-file"`, `Attach files`, `id="chat-reference-btn"`, `id="chat-skill-btn"`, `Reference project file`, `Use skill`} {
		if strings.Contains(page, unrequested) {
			t.Errorf("chat options retained unrequested entry %q", unrequested)
		}
	}
}

func TestChatNavigationUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{`id="chat-agents-btn"`, `id="chat-family-bar"`, `id="chat-agents"`, `id="chat-agent-backdrop"`, `aria-label="Child agent activity"`} {
		if !strings.Contains(page, want) {
			t.Errorf("chat navigation missing %q", want)
		}
	}
	asset := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`cstate.drafts[cstate.session] = prompt.value`,
		`cstate.scrolls[cstate.session] = transcript.scrollTop`,
		`openChat(item.session, item.title || item.session)`,
		`"/navigation"`,
		`var ancestors = data.ancestors || []`,
		`bar.hidden = ancestors.length === 0`,
		`pending input`,
	} {
		if !strings.Contains(asset, want) {
			t.Errorf("chat navigation script missing %q", want)
		}
	}
	for _, redundant := range []string{`Return to change`, `Return to " + item.task`, `meta.className = "chat-family-meta"`, `current.textContent = data.current.title`} {
		if strings.Contains(asset, redundant) {
			t.Errorf("chat navigation retained redundant current-session chrome %q", redundant)
		}
	}
}

func TestChatCompactViewStackUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{
		`interactive-widget=resizes-content`,
		`data-chat-view="chat"`, `data-chat-view-panel="chat"`,
		`data-chat-view-panel="agents"`, `data-chat-view-panel="controls"`,
		`data-chat-view-panel="work"`, `data-close-chat`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("compact chat shell missing %q", want)
		}
	}

	asset := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`matchMedia("(max-width: 840px)")`, `function openChatView(name, opener)`,
		`function closeChatView()`, `panel.inert = !selected`,
		`panel.setAttribute("aria-hidden", String(!selected))`,
		`saveChatViewPosition(current)`, `restoreChatViewPosition(active)`,
		`cstate.viewStack.length < 2`, `function syncChatHeader()`,
		`if (!closeTopChatAuxiliary()) closeChat()`,
		`--chat-viewport-top`, `viewport.offsetTop`, `--chat-viewport-left`, `viewport.offsetLeft`,
		`--chat-viewport-width`, `viewport.width`, `--chat-viewport-height`, `viewport.height`,
	} {
		if !strings.Contains(asset, want) {
			t.Errorf("compact chat controller missing %q", want)
		}
	}

	css := do(t, h, "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`@media (max-width: 840px)`, `[data-chat-view-panel]`, `.chat-window[data-chat-view="work"]`,
		`top: var(--chat-viewport-top, 0)`, `left: var(--chat-viewport-left, 0)`,
		`width: var(--chat-viewport-width, 100%)`, `#chat-overlay { background: var(--bg); }`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("compact chat CSS missing %q", want)
		}
	}
}

func TestChatFormCompactFullscreenContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	css := do(t, h, "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`@media (max-width: 840px)`, `.chat-form {`, `position: fixed`, `inset: 0`,
		`height: var(--chat-viewport-height, 100dvh)`, `max-height: none`,
		`env(safe-area-inset-top)`, `env(safe-area-inset-right)`,
		`env(safe-area-inset-bottom)`, `env(safe-area-inset-left)`,
		`.chat-form-body`, `overflow-y: auto`, `.chat-form-nav`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("compact full-screen chat form CSS missing %q", want)
		}
	}
}

func TestChatDesktopAuxiliaryPanelUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	js := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`function requestChatAuxiliary(name)`, `if (compactChatUI()) return false`,
		`requestChatAuxiliary("work")`, `requestChatAuxiliary("agents")`, `requestChatAuxiliary("controls")`,
		`function openChatTasks(opener)`, `function openChatAgents(opener)`, `function openChatControls(opener)`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("desktop chat auxiliary arbiter missing %q", want)
		}
	}

	css := do(t, h, "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`@media (min-width: 841px)`, `.chat-main { min-width: 600px; }`,
		`.chat-tasks[hidden], .chat-tasks:not(.open)`, `.chat-agents[hidden], .chat-agents:not(.open)`,
		`.chat-more-wrap { position: relative; display: block; flex: none; }`,
		`clamp(220px, 24vw, 280px)`, `clamp(240px, 24vw, 300px)`, `clamp(240px, 25vw, 320px)`,
		`calc((100% - 860px) / 2)`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("desktop chat auxiliary CSS missing %q", want)
		}
	}
}

func TestChatComposerNavigationUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{
		`class="modal-close chat-back" data-close-chat aria-label="Back"><span class="back-chevron"`,
		`class="chat-heading"`, `id="chat-session-state"`, `role="status"`,
		`id="chat-more-btn"`, `aria-label="Chat options"`, `aria-haspopup="menu"`,
		`aria-expanded="false"`, `aria-controls="chat-more-menu"`,
		`id="chat-more-menu"`, `role="menu"`, `id="chat-more-backdrop"`,
		`id="chat-compact-btn"`,
		`id="chat-controls-btn"`, `id="chat-tasks-btn"`, `id="chat-agents-btn"`, `id="chat-variant-select"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("chat composer navigation missing %q", want)
		}
	}
	composerStart := strings.Index(page, `id="chat-composer"`)
	if composerStart < 0 {
		t.Fatal("chat composer not found")
	}
	composerEnd := strings.Index(page[composerStart:], `</form>`)
	if composerEnd < 0 {
		t.Fatal("chat composer end not found")
	}
	composer := page[composerStart : composerStart+composerEnd]
	for _, want := range []string{`id="chat-tasks-btn"`, `id="chat-agents-btn"`, `id="chat-controls-btn"`, `id="chat-more-btn"`, `id="chat-context-usage"`, `id="chat-send-btn"`} {
		if !strings.Contains(composer, want) {
			t.Errorf("chat composer missing navigation control %q", want)
		}
	}
	controlsEnd := strings.Index(composer, `class="chat-more-wrap"`)
	usageAt := strings.Index(composer, `id="chat-context-usage"`)
	spacerAt := strings.Index(composer, `class="chat-compose-spacer"`)
	if controlsEnd < 0 || usageAt < controlsEnd || spacerAt < usageAt {
		t.Errorf("context usage is not positioned after navigation controls and before the composer spacer")
	}

	js := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`function closeChatMore(returnFocus)`, `function openChatMore()`,
		`button.setAttribute("aria-expanded", "false")`, `button.setAttribute("aria-expanded", "true")`,
		`moreBackdrop.addEventListener("click"`, `closeChatMessageActions(true) || closeChatMore(true) || closeChatReferences(true)`,
		`closeChatMore(false);`, `openChatControls(moreButton)`, `e.target.closest('[role="menuitem"]')`,
		`setChatHeaderState(busy ? "Working" : "Idle"`, `setChatHeaderState("Disconnected", "error")`,
		`panel.hidden = !change`, `toggle.hidden = !change`, `toggle.hidden = descendants.length === 0`,
		`function renderChatVariantSelect(data)`, `fallback.textContent = "Default"`,
		`{ model: cstate.controls.model, variant: select.value }`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("chat composer navigation script missing %q", want)
		}
	}

	css := do(t, h, "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`.chat-back`, `.back-chevron`, `border-left: 2px solid currentColor`,
		`@media (max-width: 840px)`, `.chat-more-wrap { position: relative; display: block; flex: none; }`,
		`.chat-heading {`, `min-width: 5.5rem`, `.chat-session-state { display: block`,
		`.chat-more-menu button {`, `bottom: calc(100% + 0.5rem)`, `min-height: 44px`, `.chat-more-menu[hidden], .chat-more-backdrop[hidden]`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("chat composer navigation CSS missing %q", want)
		}
	}
}

func TestChatAccessibilityContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{
		`class="chat-window" role="dialog" aria-modal="true"`,
		`id="chat-agents"`, `aria-label="Child agent activity" aria-hidden="true" inert`,
		`id="chat-controls-sheet"`, `aria-label="Session controls" aria-hidden="true" inert`,
		`for="chat-controls-search">Filter session controls`,
		`aria-label="Attached skills" aria-live="polite"`,
		`aria-label="Attached files and references" aria-live="polite"`,
		`role="region" aria-label="Project references"`,
		`id="chat-reference-close"`, `for="chat-reference-search">Filter project references`,
		`id="chat-context-usage"`, `class="chat-context-usage unavailable"`, `aria-label="Active context usage unavailable"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("accessible Chat shell missing %q", want)
		}
	}

	js := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`function closeTopChatAuxiliary()`, `function restoreChatAuxiliaryFocus(name)`,
		`cstate.opener = opener`, `opener.focus({ preventScroll: true })`,
		`drafts: {}`, `files: {}`, `references: {}`, `skills: {}`, `scrolls: {}`, `viewScrolls: {}`,
		`cstate.drafts[sessionID] || ""`, `cstate.files[cstate.session]`, `cstate.references[cstate.session]`, `cstate.skills[cstate.session]`,
		`panel.setAttribute("aria-hidden", String(!selected))`, `panel.inert = !selected`,
		`var labels = { work: "Work", agents: "Child activity", controls: "Session controls" }`,
		`if (!closeTopChatAuxiliary()) closeChat()`, `state.hidden = view !== "chat"`,
		`function closeChatReferences(returnFocus)`, `function menuKeyboard(menu, close)`,
		`e.key !== "ArrowDown"`, `e.key !== "Tab" || !chatOpen()`,
		`window.visualViewport.addEventListener("resize", cstate.viewportHandler)`,
		`syncChatModalInert(true)`, `if (!closeTopChatAuxiliary()) closeChat()`,
		`data-chat-detail-inert-owned`, `while (path && path !== document.body)`, `sibling !== path`, `removeAttribute("data-chat-detail-inert-owned")`,
		`if (!(node instanceof HTMLElement) || node.inert) return`, `data-chat-inert-owned`,
		`/chat/usage`, `setTimeout(function () { loadChatUsage(true); }, 15000)`, `cstate.usageRequest.abort()`,
		`function settlePendingUserBoundary(transcript, sessionID)`, `running.classList.remove("is-running")`,
		`latestUser !== boundary.user`, `activityKey !== boundary.activity`,
		`header.style.setProperty("--usage"`, `"Active context " + percent + " percent`,
		`setChatStatus("");`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("accessible Chat script missing %q", want)
		}
	}

	css := do(t, h, "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`height: var(--chat-viewport-height, 100dvh)`,
		`background: conic-gradient(var(--usage-color) calc(var(--usage) * 1%)`, `.chat-context-usage::before`,
		`.chat-window :where(button, a[href], input, select, textarea, [tabindex]):focus-visible`,
		`outline: 2px solid var(--accent)`, `.chat-controls-sheet select`, `min-height: 44px`,
		`.chat-agents-head { display: none; }`, `.chat-controls-head { display: none; }`,
		`.chat-system-group.is-running > summary { color: var(--accent); }`,
		`#detail`, `z-index: 40`, `#chat-overlay`, `z-index: 30`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("accessible Chat CSS missing %q", want)
		}
	}
	if strings.Contains(js, `setChatStatus("Conversation updated")`) {
		t.Error("routine polling still exposes the chat update notice")
	}
	if strings.Contains(page, "Back to Chat") || strings.Contains(js, "Back to Chat") || strings.Contains(css, ".chat-view-back") {
		t.Error("compact Chat retained the redundant Back to Chat header control")
	}
	for _, removed := range []string{"terminal-overlay", "/terminal/ws", "xterm", "new Terminal(", "FitAddon", "autoOpenTerminal"} {
		if strings.Contains(page, removed) || strings.Contains(js, removed) || strings.Contains(css, removed) {
			t.Errorf("removed terminal integration reference remains: %q", removed)
		}
	}
	if w := do(t, h, "GET", "/terminal/ws?session=ses_test", ""); w.Code != http.StatusNotFound {
		t.Errorf("removed terminal route status = %d, want 404", w.Code)
	}
	if !strings.Contains(js, "function openPreferredSession(sessionID, title) {\n    openChat(sessionID, title);") {
		t.Error("session entry points do not converge directly on Chat")
	}
	settings := htmlGet(t, h, "/settings", false).Body.String()
	if strings.Contains(settings, "autoOpenTerminal") || strings.Contains(settings, "Auto-open terminal") {
		t.Error("settings retained the removed terminal preference")
	}
}

func TestChatCompactWorkUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{
		`id="chat-tasks-btn"`, `aria-controls="chat-tasks"`, `hidden>Work</button>`,
		`id="chat-tasks"`, `data-chat-view-panel="work"`, `aria-label="Work"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("compact Work shell missing %q", want)
		}
	}

	asset := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`if (!change) closeChatTasks()`, `panel.hidden = !change`, `toggle.hidden = !change`,
		`function openChatTasks(opener)`, `openChatView("work", opener)`, `var chatWork = panel.id === "chat-tasks"`,
		`ttp-plan ttp-plan-first`, `data-work-document="plan"`, `data-work-document="task"`,
		`src.querySelectorAll(".cards[data-status]")`, `cards.length`, `ttp-subtask-marker`,
		`data-open-subtasks`, `data-work-root`, `loadSessionTasks(subtasks.dataset.change, subtasks.dataset.task)`,
		`cstate.viewStack[cstate.viewStack.length - 1] === "work"`, `cstate.detailOpener = opener`,
	} {
		if !strings.Contains(asset, want) {
			t.Errorf("compact Work script missing %q", want)
		}
	}

	css := do(t, h, "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`.ttp-plan-first { display: none; }`, `.chat-tasks .ttp-plan-first`,
		`.chat-tasks .ttp-foot { display: none; }`, `#chat-task-backdrop:not([hidden]) { display: none; }`,
		`.ttp-subtask-marker`, `.task-detail-subtasks`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("compact Work CSS missing %q", want)
		}
	}
}

func TestChatManagementUIContract(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	page := htmlGet(t, h, "/changes/2026-09-10-0", false).Body.String()
	for _, want := range []string{`id="chat-rename-title"`, `id="chat-export-btn"`, `id="chat-unlink-btn"`, `id="chat-delete-preview-btn"`, "The OpenCode session and all conversation data remain"} {
		if !strings.Contains(page, want) {
			t.Errorf("chat management missing %q", want)
		}
	}
	asset := do(t, h, "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{"Sanitized export is unavailable on this OpenCode version", "Unlink this lessmess mapping only", "This is not unlink", `fingerprint: preview.fingerprint`, `link.download = safeSessionExportFilename(sessionID)`} {
		if !strings.Contains(asset, want) {
			t.Errorf("chat management script missing %q", want)
		}
	}
	if strings.Contains(asset, "new WebSocket(lifecycleURL") || strings.Contains(asset, "chat-shell-execute") {
		t.Fatal("session management must not add chat shell execution or PTY wiring")
	}
}

func TestBoardLifecycleButtons(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	w := htmlGet(t, h, "/changes/2026-09-10-0", false)
	body := w.Body.String()
	if !strings.Contains(body, `id="close-change-btn"`) || strings.Contains(body, `id="reopen-btn"`) {
		t.Errorf("In progress board should show Close change only")
	}
	if !strings.Contains(body, `id="commit-btn"`) {
		t.Errorf("missing Commit button")
	}
	// No manual status control anywhere: the pill is derived from tasks.
	if strings.Contains(body, `id="overall-status"`) {
		t.Errorf("board must not render a status select")
	}
	if strings.Contains(body, `pill progress`) {
		t.Errorf("board must not render the progress pill")
	}

	// After closing, the board offers Reopen instead and drops the select.
	if err := st.SetChangeStatus("2026-09-10-0", model.OverallDone); err != nil {
		t.Fatal(err)
	}
	w = htmlGet(t, h, "/changes/2026-09-10-0", false)
	body = w.Body.String()
	if !strings.Contains(body, `id="reopen-btn"`) || strings.Contains(body, `id="close-change-btn"`) {
		t.Errorf("Done board should show Reopen only")
	}
	if strings.Contains(body, `id="overall-status"`) {
		t.Errorf("Done board must not render a status select")
	}
}

func TestBoardFragmentHX(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0", true)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	// Fragment only: cards, but no full page chrome.
	if !strings.Contains(body, "data-task=\"FIX-00\"") {
		t.Error("fragment missing card")
	}
	if strings.Contains(body, "<html") {
		t.Error("fragment should not be a full page")
	}
}

func TestTaskDetailHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0/tasks/00-first.md", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	// goldmark rendered the skeleton headings to HTML.
	if !strings.Contains(body, "<h2 id=\"objective\">Objective</h2>") && !strings.Contains(body, "<h2") {
		t.Errorf("task detail did not render markdown headings: %.200s", body)
	}
	if !strings.Contains(body, "FIX-00") {
		t.Error("missing task id")
	}
	if !strings.Contains(body, `class="modal-close detail-back"`) || !strings.Contains(body, `aria-label="Back"`) {
		t.Error("task detail missing compact back control")
	}
	for _, want := range []string{`data-task-status-control`, `data-change="2026-09-10-0"`, `data-task="FIX-00"`, `data-task-status`, `task-detail-select`} {
		if !strings.Contains(body, want) {
			t.Errorf("task detail missing status control %q", want)
		}
	}
	if strings.Contains(body, `class="pill status-`) || strings.Contains(body, `type="submit"`) {
		t.Error("task detail retained duplicate status pill or update button")
	}
	asset := do(t, New(st).Handler(), "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{`select.closest("[data-task-status-control]")`, `"/tasks/" + encodeURIComponent(control.dataset.task) + "/status"`, `window.prompt("Verification evidence for Test`, `"X-Lessmess-UI": "1"`, `message.textContent = "Status updated."`} {
		if !strings.Contains(asset, want) {
			t.Errorf("task detail status script missing %q", want)
		}
	}
}

func TestLedgerDetailHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0/ledger", true)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `data-close-detail`) {
		t.Error("ledger detail missing modal chrome")
	}
	// The ledger's task table rendered as HTML, with the leading H1 dropped.
	if !strings.Contains(body, "<table") {
		t.Error("ledger detail did not render markdown table")
	}
	if strings.Contains(body, "<h1") {
		t.Error("ledger detail should drop the leading H1")
	}
}

func TestDetailReadingViewContract(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	cases := []struct {
		name string
		path string
	}{
		{name: "task", path: "/changes/2026-09-10-0/tasks/00-first.md"},
		{name: "plan", path: "/changes/2026-09-10-0/plan"},
		{name: "ledger", path: "/changes/2026-09-10-0/ledger"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := htmlGet(t, s.Handler(), tc.path, true).Body.String()
			for _, want := range []string{`role="dialog"`, `aria-modal="true"`, `aria-labelledby="detail-title"`, `id="detail-title"`, `class="modal-close detail-back"`, `aria-label="Back"`, `class="modal-panes"`, `class="modal-toc" hidden`, `class="modal-body prose"`} {
				if !strings.Contains(body, want) {
					t.Errorf("%s detail missing %q", tc.name, want)
				}
			}
		})
	}

	t.Run("review", func(t *testing.T) {
		w := httptest.NewRecorder()
		s.rend.render(w, s.rend.partial, "reviewDetail", planView{ID: "2026-09-10-0", Body: "## Verdict\n\n## Evidence"})
		body := w.Body.String()
		for _, want := range []string{`role="dialog"`, `aria-labelledby="detail-title"`, `id="detail-title"`, `class="modal-close detail-back"`, `aria-label="Back"`, `class="modal-toc" hidden`, `class="modal-body prose"`} {
			if !strings.Contains(body, want) {
				t.Errorf("review detail missing %q", want)
			}
		}
	})

	js := do(t, s.Handler(), "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`body.querySelectorAll("h2, h3")`, `if (heads.length < 2)`,
		`toggle.id = "detail-contents-toggle"`, `list.id = "detail-contents-list"`,
		`toggle.setAttribute("aria-expanded"`, `list.hidden = true`,
		`window.visualViewport`, `h.focus({ preventScroll: true })`,
		`opener.matches('[hx-target="#detail"]')`, `opener.focus({ preventScroll: true })`,
		`function closeDetailContents(returnFocus)`, `if (!closeDetailContents(true)) closeDetail()`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("detail script missing %q", want)
		}
	}

	css := do(t, s.Handler(), "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`@media (max-width: 840px)`, `#detail .modal.has-toc`,
		`#detail .modal-panes { flex-direction: column`, `#detail .modal-toc`,
		`position: sticky`, `min-height: 44px`, `env(safe-area-inset-bottom)`,
		`height: var(--detail-viewport-height, 100dvh)`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("compact detail CSS missing %q", want)
		}
	}
}

func TestMarkdownHeadingIDs(t *testing.T) {
	html := string(renderMarkdown("## Objective and context\n\ntext"))
	if !strings.Contains(html, `<h2 id="objective-and-context">`) {
		t.Errorf("heading id missing: %s", html)
	}
}

func TestStaticAssets(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	for _, p := range []string{"/static/htmx.min.js", "/static/Sortable.min.js", "/static/app.js", "/static/app.css"} {
		r := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Errorf("%s: code = %d", p, w.Code)
		}
		if cc := w.Header().Get("Cache-Control"); cc == "" {
			t.Errorf("%s: no cache header", p)
		}
	}
}

func TestFormEncodedCreateTask(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	r := httptest.NewRequest("POST", "/changes/2026-09-10-0/tasks", strings.NewReader("title=Via+form"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
}

func TestFormEncodedCreateChangeRedirect(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	r := httptest.NewRequest("POST", "/changes/", strings.NewReader("title=New+thing&prefix=NT"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	redir := w.Header().Get("HX-Redirect")
	if !strings.HasPrefix(redir, "/changes/") {
		t.Fatalf("HX-Redirect = %q", redir)
	}
}
