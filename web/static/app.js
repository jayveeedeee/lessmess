// lessmess board UI glue: SortableJS drag-and-drop, SSE live refresh,
// htmx form follow-ups, validation banner.
(function () {
  "use strict";

  var page = document.body.getAttribute("data-page");

  // --- effective settings (terminal auto-open gate) -------------------------

  // Cached once per page load; gates terminal auto-open after session
  // creation. Deliberate opens (clicking a session) are never gated.
  var autoOpenTerminal = true;
  fetch("/api/settings", { headers: { Accept: "application/json" } })
    .then(function (r) { return r.ok ? r.json() : null; })
    .then(function (j) {
      if (j && j.effective && j.effective.session) {
        autoOpenTerminal = j.effective.session.autoOpenTerminal !== false;
      }
    })
    .catch(function () {});

  function maybeOpenTerminal(sessionID, title) {
    if (autoOpenTerminal) openTerminal(sessionID, title);
  }

  function boardEl() { return document.getElementById("board"); }

  // --- drag and drop -----------------------------------------------------

  function initSortable() {
    var board = boardEl();
    if (!board || typeof Sortable === "undefined") return;
    var change = board.getAttribute("data-change");
    board.querySelectorAll(".cards").forEach(function (col) {
      if (col._sortable) return; // already initialized
      col._sortable = new Sortable(col, {
        group: "board",
        animation: 120,
        ghostClass: "sortable-ghost",
        dragClass: "sortable-drag",
        onEnd: function (evt) {
          var task = evt.item.getAttribute("data-task");
          var status = evt.to.getAttribute("data-status");
          fetch("/changes/" + encodeURIComponent(change) + "/move", {
            method: "POST",
            headers: { "Content-Type": "application/json", Accept: "application/json" },
            body: JSON.stringify({ task: task, status: status, index: evt.newIndex }),
          })
            .then(function (r) {
              if (!r.ok) return r.json().then(function (j) { throw new Error(j.error || r.statusText); });
            })
            .then(refreshBoard)
            .catch(function (e) {
              alert("Move failed: " + e.message);
              refreshBoard();
            });
        },
      });
    });
  }

  // --- board refresh ------------------------------------------------------

  function refreshBoard() {
    var board = boardEl();
    if (!board || typeof htmx === "undefined") return;
    htmx.ajax("GET", location.pathname, { target: "#board", swap: "innerHTML", headers: { Accept: "text/html" } });
  }

  document.addEventListener("htmx:afterSwap", function (e) {
    if (e.target && e.target.id === "board") {
      initSortable();
      if (boardEl()) syncTerminalTasks(boardEl(), boardEl().dataset.change);
      refreshSubs();
    }
  });

  // After a successful htmx form POST (add task), refresh the board.
  document.addEventListener("htmx:afterRequest", function (e) {
    if (!e.detail.successful) {
      var msg = "Request failed";
      try { msg = JSON.parse(e.detail.xhr.responseText).error || msg; } catch (_) {}
      alert(msg);
      return;
    }
    if (e.detail.elt.matches('form[hx-post*="/tasks"]')) {
      e.detail.elt.reset();
      refreshBoard();
    }
    if (e.detail.elt.matches('form[hx-post="/changes/session"]')) {
      try {
        var j = JSON.parse(e.detail.xhr.responseText);
        loadDiscussions();
        maybeOpenTerminal(j.session, j.title);
      } catch (_) {}
    }
  });

  // --- SSE live updates ----------------------------------------------------

  var es = typeof EventSource !== "undefined" ? new EventSource("/events") : null;
  var timer = null;
  if (es) {
    es.addEventListener("fs", scheduleRefresh);
    es.addEventListener("write", scheduleRefresh);
    es.addEventListener("docs", scheduleDocsRefresh);
  }

  function scheduleRefresh() {
    clearTimeout(timer);
    timer = setTimeout(function () {
      checkValidation();
      if (page === "board" && boardEl()) refreshBoard();
      // While the terminal is open on the index, a full reload would
      // destroy it — and the common cause of this event is the open
      // discussion scaffolding a change right now. Follow the session
      // to its new board instead of reloading.
      else if (page === "index" && terminalOpen() && tstate.session) followSession();
      else if (page === "index") location.reload();
      // Terminal task panel: re-mirror an open panel, and pick up a
      // fresh binding (a discussion that just scaffolded a change —
      // the scaffold writes changes/, which fired this event).
      if (terminalOpen()) {
        if (terminalPanelChange) loadTerminalTasks(terminalPanelChange);
        else if (!(page === "board" && boardEl()) && tstate.session) resolveTerminalPanel(tstate.session);
      }
    }, 250);
  }

  // followSession: the open terminal's session was likely just scaffolded
  // into a change. Once the mapping says so, navigate to that change's
  // board with ?session= — autoOpenSession reopens the same session there
  // with its task panel. Until it binds, skip the refresh entirely; the
  // stale list self-heals on navigation or when the terminal closes.
  function followSession() {
    fetch("/api/sessions/" + encodeURIComponent(tstate.session) + "/change", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        if (!j || !j.change) return;
        if (!terminalOpen() || !tstate.session) return; // user moved on
        location.assign("/changes/" + encodeURIComponent(j.change) + "?session=" + encodeURIComponent(tstate.session));
      })
      .catch(function () {});
  }

  // --- explorer: live tree refresh + directory chat -------------------------

  var docsTimer = null;
  function scheduleDocsRefresh() {
    clearTimeout(docsTimer);
    docsTimer = setTimeout(function () {
      document.dispatchEvent(new CustomEvent("tt:docs-event"));
      checkValidation();
      if (page === "explorer") refreshExplorerTree();
    }, 200);
  }

  // Selected directory for the master/detail explorer; root by default.
  var explorerSelected = ".";

  function refreshExplorerTree() {
    var box = document.getElementById("explorer-tree");
    if (!box) return;
    var open = [];
    box.querySelectorAll("details[open]").forEach(function (d) {
      if (d.dataset.rel) open.push(d.dataset.rel);
    });
    fetch("/explorer/tree", { headers: { Accept: "text/html" } })
      .then(function (r) { return r.text(); })
      .then(function (html) {
        box.innerHTML = html;
        if (window.htmx) htmx.process(box); // wire hx-get on the new summaries
        open.forEach(function (rel) {
          var d = box.querySelector('details[data-rel="' + CSS.escape(rel) + '"]');
          if (d) d.open = true;
        });
        // Restore the selection, falling back to root if the dir vanished.
        var sel = box.querySelector('details[data-rel="' + CSS.escape(explorerSelected) + '"] > summary');
        if (!sel) {
          explorerSelected = ".";
          sel = box.querySelector('details[data-rel="."] > summary');
        }
        box.querySelectorAll("summary.selected").forEach(function (s) { s.classList.remove("selected"); });
        if (sel) sel.classList.add("selected");
        refreshExplorerDetail();
      })
      .catch(function () {});
  }

  function refreshExplorerDetail() {
    var pane = document.getElementById("explorer-detail");
    if (!pane) return;
    fetch("/explorer/detail?dir=" + encodeURIComponent(explorerSelected), { headers: { Accept: "text/html" } })
      .then(function (r) { return r.text(); })
      .then(function (html) { pane.innerHTML = html; })
      .catch(function () {});
  }

  // Directory selection: htmx fetches the detail itself; we only track state.
  document.addEventListener("click", function (e) {
    var sum = e.target.closest(".xtree summary.xdir");
    if (!sum) return;
    var det = sum.closest("details");
    if (!det || !det.dataset.rel) return;
    explorerSelected = det.dataset.rel;
    var box = document.getElementById("explorer-tree");
    if (!box) return;
    box.querySelectorAll("summary.selected").forEach(function (s) { s.classList.remove("selected"); });
    sum.classList.add("selected");
  });

  // Chat lives only in the detail pane header (tree rows have no button).
  // The handler still runs in the capture phase: harmless, and it keeps any
  // future hx-get ancestor from also firing on chat clicks.
  document.addEventListener("click", function (e) {
    var btn = e.target.closest(".explorer-chat");
    if (!btn) return;
    e.preventDefault();
    e.stopPropagation();
    btn.disabled = true;
    fetch("/explorer/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ dir: btn.dataset.dir }),
    })
      .then(function (r) {
        return r.json().then(function (j) {
          if (!r.ok) throw new Error(j.error || "chat failed");
          return j;
        });
      })
      .then(function (j) { openTerminal(j.session, j.title); })
      .catch(function (err) { alert(err.message); })
      .finally(function () { btn.disabled = false; });
  }, true);

  // --- validation banner ---------------------------------------------------

  function checkValidation() {
    fetch("/api/validate", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) {
        var banner = document.getElementById("banner");
        if (banner) {
          if (j.violations && j.violations.length) {
            banner.innerHTML =
              "<strong>Validation violations:</strong><ul>" +
              j.violations.map(function (v) {
                var li = document.createElement("li");
                li.textContent = v.File + ": rule " + v.Rule + ": " + v.Msg;
                return li.outerHTML;
              }).join("") +
              "</ul>";
            banner.hidden = false;
          } else {
            banner.hidden = true;
          }
        }
        latestDocsFindings = j.docs || [];
        updateNotifBadge();
        renderNotifList();
        var seedBtn = document.getElementById("docs-seed-btn");
        if (seedBtn) {
          var pending = j.docsSeedPending;
          seedBtn.textContent = pending > 0
            ? "Run missing docs (" + pending + (pending === 1 ? " dir" : " dirs") + ")"
            : "Run docs seed";
        }
      })
      .catch(function () {});
  }

  // --- docs notifications (bell + modal) --------------------------------------

  var latestDocsFindings = [];
  var refreshRunning = false;

  function updateNotifBadge() {
    var bell = document.getElementById("notif-bell");
    var badge = document.getElementById("notif-badge");
    if (!bell || !badge) return;
    var n = latestDocsFindings.length;
    bell.hidden = n === 0;
    if (n === 0) return;
    badge.textContent = n;
    bell.classList.toggle(
      "err",
      latestDocsFindings.some(function (f) { return f.severity === "error"; })
    );
  }

  function renderNotifList() {
    var box = document.getElementById("notif-list");
    if (!box || notifModal().hidden) return;
    if (!latestDocsFindings.length) {
      box.innerHTML = '<p class="muted">No docs findings — everything is fresh.</p>';
      return;
    }
    var html = "";
    [["error", "Docs errors"], ["warning", "Docs warnings"]].forEach(function (g) {
      var items = latestDocsFindings.filter(function (f) { return f.severity === g[0]; });
      if (!items.length) return;
      html += '<h3 class="notif-group ' + g[0] + '">' + g[1] + "</h3><ul>" +
        items.map(function (f) {
          var li = document.createElement("li");
          li.textContent = f.file + ": " + f.msg;
          return li.outerHTML;
        }).join("") + "</ul>";
    });
    box.innerHTML = html;
  }

  function notifModal() { return document.getElementById("notif-modal"); }

  function openNotifModal() {
    notifModal().hidden = false;
    renderNotifList();
    checkValidation();
  }

  function closeNotifModal() { notifModal().hidden = true; }

  (function initNotifs() {
    var bell = document.getElementById("notif-bell");
    if (!bell) return;
    bell.addEventListener("click", openNotifModal);
    document.querySelector("[data-close-notif]").addEventListener("click", closeNotifModal);
    notifModal().addEventListener("click", function (e) {
      if (e.target === notifModal()) closeNotifModal();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && !notifModal().hidden) closeNotifModal();
    });

    var btn = document.getElementById("docs-refresh-btn");
    var status = document.getElementById("notif-refresh-status");
    btn.addEventListener("click", function () {
      btn.disabled = true;
      refreshRunning = true;
      status.textContent = "Refreshing…";
      fetch("/docs/refresh", { method: "POST", headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          if (j.enqueued) {
            status.textContent =
              "Refresh running for " + j.enqueued.length +
              (j.enqueued.length === 1 ? " directory" : " directories") +
              " — can take a few minutes; findings update live.";
          } else {
            status.textContent = j.status || j.error || "Nothing to do.";
            btn.disabled = false;
            refreshRunning = false;
          }
        })
        .catch(function () {
          status.textContent = "Refresh request failed.";
          btn.disabled = false;
          refreshRunning = false;
        });
    });
    // A running refresh likely finished when docs events arrive; re-enable.
    document.addEventListener("tt:docs-event", function () {
      if (refreshRunning) {
        refreshRunning = false;
        btn.disabled = false;
        status.textContent = "Docs updated — findings re-checked.";
      }
    });

    // Seed docs: without force, runs the covered dirs missing their doc
    // files; the force checkbox redoes everything regardless.
    var seedBtn = document.getElementById("docs-seed-btn");
    if (seedBtn) {
      var seedRunning = false;
      seedBtn.addEventListener("click", function () {
        var force = document.getElementById("docs-seed-force");
        if (force && force.checked &&
            !confirm("Force will re-run the docs seed for EVERY covered directory, ignoring what already exists. Continue?")) {
          return;
        }
        seedBtn.disabled = true;
        seedRunning = true;
        status.textContent = "Seeding…";
        fetch("/docs/seed", {
          method: "POST",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: JSON.stringify({ force: !!(force && force.checked) }),
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function (j) {
            status.textContent = j.status
              ? j.status
              : "Seed running — can take a few minutes; findings update live.";
            if (j.status) {
              seedBtn.disabled = false;
              seedRunning = false;
            }
          })
          .catch(function (err) {
            status.textContent = "Seed request failed: " + err.message;
            seedBtn.disabled = false;
            seedRunning = false;
          });
      });
      document.addEventListener("tt:docs-event", function () {
        if (seedRunning) {
          seedRunning = false;
          seedBtn.disabled = false;
        }
      });
    }
  })();

  // --- theme toggle ---------------------------------------------------------

  function applyTheme(theme) {
    document.documentElement.dataset.theme = theme;
    var btn = document.getElementById("theme-toggle");
    if (btn) btn.textContent = theme === "light" ? "☀" : "☾";
  }

  function initTheme() {
    applyTheme(localStorage.getItem("tt-theme") || "dark");
    var btn = document.getElementById("theme-toggle");
    if (btn) {
      btn.addEventListener("click", function () {
        var next = document.documentElement.dataset.theme === "light" ? "dark" : "light";
        localStorage.setItem("tt-theme", next);
        applyTheme(next);
      });
    }
  }

  // --- index discussions list -------------------------------------------------

  function loadDiscussions() {
    var ul = document.getElementById("discussions-list");
    if (!ul) return;
    fetch("/api/discussions", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) {
        ul.innerHTML = "";
        var sessions = j.sessions || [];
        if (!sessions.length) {
          var empty = document.createElement("li");
          empty.className = "session-empty";
          empty.textContent = "No open discussions — start one with “New change session”.";
          ul.appendChild(empty);
          return;
        }
        sessions.forEach(function (s) {
          var li = document.createElement("li");
          li.className = "session-item";
          var title = document.createElement("span");
          title.className = "session-title";
          title.textContent = s.title;
          var meta = document.createElement("span");
          meta.className = "session-meta";
          meta.textContent = (s.created || "").slice(0, 10);
          var actions = document.createElement("span");
          actions.className = "session-actions";
          var openBtn = document.createElement("button");
          openBtn.className = "btn-ghost";
          openBtn.textContent = "Open";
          openBtn.addEventListener("click", function () { openTerminal(s.session, s.title); });
          var unBtn = document.createElement("button");
          unBtn.className = "btn-ghost";
          unBtn.textContent = "✕";
          unBtn.title = "Unlink (session stays in opencode)";
          unBtn.addEventListener("click", function () {
            fetch("/api/discussions/" + s.session, { method: "DELETE" })
              .then(function (r) { if (!r.ok) throw 0; loadDiscussions(); })
              .catch(function () { alert("Unlink failed"); });
          });
          actions.appendChild(openBtn);
          actions.appendChild(unBtn);
          li.appendChild(title);
          li.appendChild(meta);
          li.appendChild(actions);
          ul.appendChild(li);
        });
      })
      .catch(function () {});
  }

  // --- sessions panel + terminal ----------------------------------------------

  function changeID() {
    var b = boardEl();
    return b ? b.getAttribute("data-change") : null;
  }

  // Last-opened session per change (client-side; this is a single-user tool).
  function lastSessionKey() { return "tt-last-session:" + changeID(); }
  function markOpened(sessionID) {
    try { localStorage.setItem(lastSessionKey(), sessionID); } catch (_) {}
  }

  // fetchSessions calls cb with the change's sessions, or null on error.
  function fetchSessions(cb) {
    fetch("/changes/" + changeID() + "/sessions", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) { cb(j.sessions || []); })
      .catch(function () { cb(null); });
  }

  function loadSessions() {
    fetchSessions(function (sessions) {
      var ul = document.getElementById("sessions-list");
      if (!ul) return;
      ul.innerHTML = "";
      if (!sessions || !sessions.length) {
        var empty = document.createElement("li");
        empty.className = "session-empty";
        empty.textContent = "No sessions yet — start one.";
        ul.appendChild(empty);
        return;
      }
      renderSubs(sessions);
      sessions.forEach(function (s) {
        var li = document.createElement("li");
        li.className = "session-item";
        var title = document.createElement("span");
        title.className = "session-title";
        title.textContent = s.title;
        var meta = document.createElement("span");
        meta.className = "session-meta";
        meta.textContent = (s.created || "").slice(0, 10);
        var actions = document.createElement("span");
        actions.className = "session-actions";
        var openBtn = document.createElement("button");
        openBtn.className = "btn-ghost";
        openBtn.textContent = "Open";
        openBtn.addEventListener("click", function () { markOpened(s.session); openTerminal(s.session, s.title); });
        var unBtn = document.createElement("button");
        unBtn.className = "btn-ghost";
        unBtn.textContent = "✕";
        unBtn.title = "Unlink from change (session stays in opencode)";
        unBtn.addEventListener("click", function () {
          fetch("/changes/" + changeID() + "/sessions/" + s.session, { method: "DELETE" })
            .then(function (r) { if (!r.ok) throw 0; loadSessions(); })
            .catch(function () { alert("Unlink failed"); });
        });
        actions.appendChild(openBtn);
        actions.appendChild(unBtn);
        li.appendChild(title);
        li.appendChild(meta);
        li.appendChild(actions);
        ul.appendChild(li);
      });
    });
  }

  // --- subagent session chips -------------------------------------------------

  // refreshSubs re-renders the subagent chips from the sessions endpoint;
  // called on board load and after every board fragment swap.
  function refreshSubs() {
    fetchSessions(renderSubs);
  }

  // renderSubs attaches subagent sessions to their task cards (via the
  // task-ID title prefix the server maps into `task`); bound sessions
  // without a task fall back to the header strip. Sessions without a
  // parent are plain change sessions and render in the Sessions panel only.
  function renderSubs(sessions) {
    var byTask = {};
    var stray = [];
    (sessions || []).forEach(function (s) {
      if (s.task) (byTask[s.task] = byTask[s.task] || []).push(s);
      else if (s.parent) stray.push(s);
    });
    document.querySelectorAll("#board .card[data-task]").forEach(function (card) {
      var old = card.querySelector(".card-subs");
      if (old) old.remove();
      var subs = byTask[card.dataset.task];
      if (!subs || !subs.length) return;
      var wrap = document.createElement("div");
      wrap.className = "card-subs";
      subs.forEach(function (s) { wrap.appendChild(subChip(s)); });
      card.appendChild(wrap);
    });
    var strip = document.getElementById("board-subs");
    if (!strip) return;
    strip.innerHTML = "";
    if (!stray.length) { strip.hidden = true; return; }
    strip.hidden = false;
    stray.forEach(function (s) { strip.appendChild(subChip(s)); });
  }

  // subChip is one subagent session: a live dot, its title, and a Talk
  // button opening the terminal overlay on that session id. Dead sessions
  // (gone from the opencode service) render dimmed but stay openable.
  function subChip(s) {
    var chip = document.createElement("span");
    chip.className = "sub-chip" + (s.live ? "" : " sub-dead");
    var dot = document.createElement("span");
    dot.className = "sub-dot";
    chip.appendChild(dot);
    var label = document.createElement("span");
    label.className = "sub-label";
    label.textContent = s.title || s.session;
    label.title = s.session;
    chip.appendChild(label);
    var talk = document.createElement("button");
    talk.type = "button";
    talk.className = "btn-ghost sub-talk";
    talk.textContent = "Talk";
    talk.title = s.live
      ? "Open a terminal chat on this subagent session"
      : "Session not found in the opencode service";
    talk.addEventListener("click", function () { markOpened(s.session); openTerminal(s.session, s.title); });
    chip.appendChild(talk);
    return chip;
  }

  function initSessions() {
    var btn = document.getElementById("sessions-btn");
    if (!btn) return;
    btn.addEventListener("click", function () {
      var p = document.getElementById("sessions-panel");
      p.hidden = !p.hidden;
      if (!p.hidden) loadSessions();
    });
    var nb = document.getElementById("new-session-btn");
    if (nb) {
      nb.addEventListener("click", function () {
        fetch("/changes/" + changeID() + "/sessions", {
          method: "POST",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: "{}",
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function (s) { markOpened(s.session); loadSessions(); maybeOpenTerminal(s.session, s.title); })
          .catch(function (e) { alert("Create session failed: " + e.message); });
      });
    }
  }

  // --- continue / start session button ---------------------------------------

  // One-click resume: opens the last-opened session (validated against the
  // live list, falling back to the newest created), or creates + opens a
  // session when the change has none.
  function initContinue() {
    var btn = document.getElementById("continue-session-btn");
    if (!btn) return;
    fetchSessions(function (sessions) {
      if (sessions) btn.textContent = sessions.length ? "Continue session" : "Start session";
      renderSubs(sessions);
    });
    btn.addEventListener("click", function () {
      if (btn.disabled) return;
      fetchSessions(function (sessions) {
        if (!sessions) { alert("Could not load sessions"); return; }
        if (!sessions.length) { createAndOpen(btn); return; }
        var stored = null;
        try { stored = localStorage.getItem(lastSessionKey()); } catch (_) {}
        var s = sessions.find(function (x) { return x.session === stored; }) || sessions[sessions.length - 1];
        markOpened(s.session);
        openTerminal(s.session, s.title);
      });
    });
  }

  function createAndOpen(btn) {
    btn.disabled = true;
    fetch("/changes/" + changeID() + "/sessions", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: "{}",
    })
      .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
      .then(function (s) {
        markOpened(s.session);
        btn.textContent = "Continue session";
        loadSessions();
        maybeOpenTerminal(s.session, s.title);
      })
      .catch(function (e) { alert("Create session failed: " + e.message); })
      .finally(function () { btn.disabled = false; });
  }

  var tstate = { term: null, ws: null, ro: null, session: null };
  // The change id the task panel currently shows for a non-board terminal
  // (null on board pages, where the panel mirrors the live board DOM).
  var terminalPanelChange = null;

  function openTerminal(sessionID, title) {
    closeTerminal();
    var overlay = document.getElementById("terminal-overlay");
    overlay.hidden = false;
    // The task panel serves any change-bound terminal: mirrored from the
    // live board DOM on board pages, resolved via the session→change
    // binding (and fed by a fetched board fragment) everywhere else.
    var panel = terminalTasksEl();
    var onBoard = page === "board" && boardEl() && boardEl().dataset.change;
    if (panel) {
      panel.hidden = true;
      panel.innerHTML = "";
      if (onBoard) {
        panel.hidden = false;
        syncTerminalTasks(boardEl(), boardEl().dataset.change);
      } else {
        resolveTerminalPanel(sessionID);
      }
    }
    document.getElementById("terminal-title").textContent = title || "";
    var status = document.getElementById("terminal-status");
    status.hidden = true;
    var container = document.getElementById("terminal-container");
    container.innerHTML = "";

    var term = new Terminal({
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
      fontSize: 13,
      cursorBlink: true,
      theme: {
        background: "#141414",
        foreground: "#e8e6e3",
        cursor: "#e8641f",
        selectionBackground: "#3a3a3a",
      },
    });
    var fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    term.open(container);
    fit.fit();

    var proto = location.protocol === "https:" ? "wss" : "ws";
    var ws = new WebSocket(
      proto + "://" + location.host + "/terminal/ws?session=" +
        encodeURIComponent(sessionID) + "&cols=" + term.cols + "&rows=" + term.rows
    );
    ws.binaryType = "arraybuffer";
    ws.onmessage = function (e) {
      term.write(typeof e.data === "string" ? e.data : new Uint8Array(e.data));
    };
    ws.onclose = function () {
      status.textContent = "disconnected — the opencode session persists; reopen to resume";
      status.hidden = false;
    };
    term.onData(function (d) {
      if (ws.readyState === 1) ws.send(d);
    });

    var ro = new ResizeObserver(function () {
      fit.fit();
      if (ws.readyState === 1) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    });
    ro.observe(container);

    tstate = { term: term, ws: ws, ro: ro, session: sessionID };
  }

  function closeTerminal() {
    if (tstate.ro) tstate.ro.disconnect();
    if (tstate.ws && tstate.ws.readyState <= 1) tstate.ws.close();
    if (tstate.term) tstate.term.dispose();
    tstate = { term: null, ws: null, ro: null, session: null };
    terminalPanelChange = null;
    var overlay = document.getElementById("terminal-overlay");
    if (overlay) overlay.hidden = true;
    var panel = terminalTasksEl();
    if (panel) { panel.hidden = true; panel.innerHTML = ""; }
    // On the index a refresh was deferred while the terminal was open
    // (see followSession); now that a reload is safe again, catch up.
    if (page === "index") scheduleRefresh();
  }

  function terminalOpen() {
    var ov = document.getElementById("terminal-overlay");
    return ov && !ov.hidden;
  }

  // --- terminal task panel -------------------------------------------------

  function terminalTasksEl() { return document.getElementById("terminal-tasks"); }

  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  // Mirrors the statusClass template func in internal/server/render.go.
  function statusClass(s) { return s.toLowerCase().replace(/ /g, "-"); }

  // The panel mirrors the board fragment DOM (.cards[data-status] /
  // .card[data-task]): on board pages src is the live #board (live updates
  // ride the existing SSE board refresh); elsewhere it is a fetched
  // fragment, re-loaded on SSE events by the terminal hooks above.
  function syncTerminalTasks(src, change) {
    var panel = terminalTasksEl();
    if (!panel || panel.hidden) return;
    var html = '<div class="ttp-scroll"><div class="ttp-head">Tasks</div>';
    var groups = 0;
    if (src) {
      src.querySelectorAll(".cards[data-status]").forEach(function (col) {
        var cards = col.querySelectorAll(".card");
        if (!cards.length) return;
        groups++;
        var status = col.getAttribute("data-status");
        html += '<div class="ttp-group"><div class="ttp-group-head status-' +
          statusClass(status) + '"><span>' + esc(status) + '</span><span class="count">' + cards.length +
          "</span></div>";
        cards.forEach(function (card) {
          var a = card.querySelector(".card-title");
          var href = a && a.getAttribute("hx-get");
          if (!href) return;
          html += '<a class="ttp-row" hx-get="' + esc(href) + '" hx-headers=\'{"Accept": "text/html"}\'' +
            ' hx-target="#detail" hx-swap="innerHTML"><span class="chip">' +
            esc(card.getAttribute("data-task") || "") + '</span><span class="ttp-row-title">' +
            esc(a.textContent) + "</span></a>";
        });
        html += "</div>";
      });
    }
    if (!groups) html += '<div class="ttp-empty">No tasks yet.</div>';
    html += "</div>"; // .ttp-scroll
    // Fixed footer: open the change plan modal, same request as the board's
    // Plan button (#detail stacks above the terminal).
    if (change) {
      html += '<div class="ttp-foot"><a class="ttp-plan" hx-get="/changes/' +
        encodeURIComponent(change) + '/plan"' +
        ' hx-target="#detail" hx-swap="innerHTML">Plan</a></div>';
    }
    panel.innerHTML = html;
    if (window.htmx) htmx.process(panel); // wire hx-get on the new rows
  }

  // Non-board terminals: resolve the session's change binding and, when
  // bound, fill the panel from the change's board fragment.
  function resolveTerminalPanel(sessionID) {
    fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/change", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        if (!j || !j.change) return;
        if (!terminalOpen() || tstate.session !== sessionID) return; // user moved on
        loadTerminalTasks(j.change);
      })
      .catch(function () {});
  }

  function loadTerminalTasks(changeID) {
    fetch("/changes/" + encodeURIComponent(changeID), { headers: { Accept: "text/html", "HX-Request": "true" } })
      .then(function (r) { return r.ok ? r.text() : null; })
      .then(function (html) {
        if (html == null || !terminalOpen()) return;
        var panel = terminalTasksEl();
        if (!panel) return;
        var src = document.createElement("div");
        src.innerHTML = html;
        terminalPanelChange = changeID;
        panel.hidden = false;
        syncTerminalTasks(src, changeID);
      })
      .catch(function () {});
  }

  // Auto-open the terminal when arriving from the new-change-session flow
  // (/changes/{id}?session={sid}).
  function autoOpenSession() {
    if (page !== "board") return;
    var sid = new URLSearchParams(location.search).get("session");
    if (!sid) return;
    history.replaceState(null, "", location.pathname);
    fetch("/changes/" + changeID() + "/sessions", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) {
        var s = (j.sessions || []).find(function (x) { return x.session === sid; });
        markOpened(sid);
        openTerminal(sid, s ? s.title : sid);
      })
      .catch(function () { openTerminal(sid, sid); });
  }

  document.addEventListener("click", function (e) {
    if (e.target.closest("[data-close-terminal]")) { closeTerminal(); return; }
    var ov = document.getElementById("terminal-overlay");
    if (ov && !ov.hidden && e.target === ov) closeTerminal();
  });

  // --- lifecycle buttons ---------------------------------------------------

  function countOpenTasks() {
    var n = 0;
    document.querySelectorAll("#board .cards").forEach(function (col) {
      var st = col.getAttribute("data-status");
      if (st !== "Done" && st !== "Cancelled") {
        n += col.querySelectorAll(".card").length;
      }
    });
    return n;
  }

  function postLifecycle(action) {
    fetch("/changes/" + changeID() + "/" + action, {
      method: "POST",
      headers: { Accept: "application/json" },
    })
      .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
      .then(function () { location.reload(); }) // status pill lives outside the board fragment
      .catch(function (e) { alert(action + " failed: " + e.message); });
  }

  function initLifecycle() {
    var closeBtn = document.getElementById("close-change-btn");
    if (closeBtn) {
      closeBtn.addEventListener("click", function () {
        var open = countOpenTasks();
        if (open > 0 && !confirm(open + " task(s) are not Done or Cancelled — close the change anyway?")) {
          return;
        }
        postLifecycle("close");
      });
    }
    var reopenBtn = document.getElementById("reopen-btn");
    if (reopenBtn) {
      reopenBtn.addEventListener("click", function () { postLifecycle("reopen"); });
    }
    var commitBtn = document.getElementById("commit-btn");
    if (commitBtn) {
      commitBtn.addEventListener("click", function () {
        if (commitBtn.disabled) return;
        setCommitBusy(true);
        fetch("/changes/" + changeID() + "/commit", {
          method: "POST",
          headers: { Accept: "application/json" },
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function (j) { pollCommitStatus(j.session, 0); })
          .catch(function (e) {
            setCommitBusy(false);
            alert("Commit failed: " + e.message);
          });
      });
    }

    function setCommitBusy(busy) {
      commitBtn.disabled = busy;
      if (busy) {
        commitBtn.dataset.label = commitBtn.textContent;
        commitBtn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Committing…';
      } else {
        commitBtn.textContent = commitBtn.dataset.label || "Commit";
      }
    }

    function pollCommitStatus(session, attempts) {
      if (attempts > 200) { // ~10 minutes max
        setCommitBusy(false);
        alert("Commit is taking unusually long — check the session in the Sessions panel.");
        return;
      }
      fetch("/changes/" + changeID() + "/commit-status?session=" + encodeURIComponent(session), {
        headers: { Accept: "application/json" },
      })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          if (j.done) {
            commitBtn.innerHTML = "✓ Committed";
            setTimeout(function () { setCommitBusy(false); }, 3000);
          } else {
            setTimeout(function () { pollCommitStatus(session, attempts + 1); }, 3000);
          }
        })
        .catch(function () {
          setCommitBusy(false);
          alert("Lost contact while waiting for the commit — check the Sessions panel.");
        });
    }
  }

  // --- detail modal -----------------------------------------------------------

  function closeDetail() {
    var d = document.getElementById("detail");
    if (d) d.hidden = true;
  }

  document.addEventListener("click", function (e) {
    if (e.target.closest("[data-close-detail]")) { closeDetail(); return; }
    var d = document.getElementById("detail");
    if (d && !d.hidden && e.target === d) closeDetail(); // backdrop click
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    var d = document.getElementById("detail");
    if (d && !d.hidden) { closeDetail(); return; } // the detail modal stacks above the terminal
    if (terminalOpen()) { closeTerminal(); return; }
    closeDetail();
  });

  document.addEventListener("htmx:afterSwap", function (e) {
    if (e.target && e.target.id === "detail") {
      e.target.hidden = false;
      buildDetailTOC();
    }
  });

  // --- detail modal TOC --------------------------------------------------------
  // The server renders headings with ids (goldmark auto heading IDs); we build
  // the left-rail contents from the swapped DOM so plan, task, and ledger
  // modals all share one code path.

  function currentChangeID() {
    var m = location.pathname.match(/^\/changes\/([^\/]+)/);
    return m ? decodeURIComponent(m[1]) : null;
  }

  // Offset of an element inside its scrolling pane, for click-to-scroll and
  // the scroll-spy (offsetTop is unreliable: .modal-body is not positioned).
  function paneOffset(body, el) {
    return el.getBoundingClientRect().top - body.getBoundingClientRect().top + body.scrollTop;
  }

  function buildDetailTOC() {
    var d = document.getElementById("detail");
    var modal = d && d.querySelector(".modal");
    var toc = d && d.querySelector(".modal-toc");
    var body = d && d.querySelector(".modal-body");
    if (!modal || !toc || !body) return;
    var heads = body.querySelectorAll("h2, h3");
    toc.innerHTML = "";
    if (heads.length < 2) {
      toc.hidden = true;
      modal.classList.remove("has-toc");
      return;
    }
    var seen = {};
    var label = document.createElement("div");
    label.className = "toc-title";
    label.textContent = "Contents";
    toc.appendChild(label);
    Array.prototype.forEach.call(heads, function (h) {
      if (!h.id) {
        var slug = h.textContent.toLowerCase().replace(/[^\w\s-]/g, "").trim().replace(/\s+/g, "-") || "section";
        var base = slug, n = 2;
        while (seen[slug]) slug = base + "-" + n++;
        h.id = slug;
      }
      seen[h.id] = true;
      var a = document.createElement("a");
      a.href = "#" + h.id;
      a.textContent = h.textContent;
      if (h.tagName === "H3") a.classList.add("toc-h3");
      a.addEventListener("click", function (e) {
        e.preventDefault();
        body.scrollTo({ top: paneOffset(body, h) - 12, behavior: "smooth" });
      });
      toc.appendChild(a);
    });
    toc.hidden = false;
    modal.classList.add("has-toc");

    var links = toc.querySelectorAll("a");
    function spy() {
      var top = body.getBoundingClientRect().top + 24;
      var cur = heads[0];
      Array.prototype.forEach.call(heads, function (h) {
        if (h.getBoundingClientRect().top <= top) cur = h;
      });
      var idx = Array.prototype.indexOf.call(heads, cur);
      links.forEach(function (l, i) { l.classList.toggle("active", i === idx); });
    }
    var raf = null;
    body.addEventListener("scroll", function () {
      if (raf) return;
      raf = requestAnimationFrame(function () { raf = null; spy(); });
    });
    spy();
  }

  // --- detail modal links ------------------------------------------------------
  // Relative .md links in the markdown (ledger, tasks, plan) open in this
  // modal instead of navigating to a nonexistent URL.

  function detailLinkTarget(href) {
    if (!href || !/\.md$/i.test(href.split("#")[0])) return null;
    if (/^[a-z][a-z0-9+.-]*:/i.test(href) || href.charAt(0) === "/") return null; // absolute: leave alone
    var hash = href.indexOf("#") >= 0 ? href.slice(href.indexOf("#") + 1) : "";
    var path = href.split("#")[0];
    var id = currentChangeID();
    var m;
    if ((m = path.match(/^tasks\/(.+\.md)$/i)) && id) {
      return { url: "/changes/" + id + "/tasks/" + m[1], frag: hash };
    }
    if ((m = path.match(/^(\d{4}-\d{2}-\d{2}-\d+)\/plan\.md$/i))) {
      return { url: "/changes/" + m[1] + "/plan", frag: hash };
    }
    if (/^(\.\.\/)?ledger\.md$/i.test(path) && id) {
      return { url: "/changes/" + id + "/ledger", frag: hash };
    }
    if (/^plan\.md$/i.test(path) && id) {
      return { url: "/changes/" + id + "/plan", frag: hash };
    }
    return null;
  }

  document.addEventListener("click", function (e) {
    var link = e.target.closest("#detail a[href]");
    if (!link) return;
    var d = document.getElementById("detail");
    if (!d || d.hidden) return;
    var t = detailLinkTarget(link.getAttribute("href"));
    if (!t) return;
    e.preventDefault();
    fetch(t.url, { headers: { Accept: "text/html" } })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.text();
      })
      .then(function (html) {
        d.innerHTML = html;
        buildDetailTOC();
        if (t.frag) {
          var body = d.querySelector(".modal-body");
          var el = body && body.querySelector('[id="' + CSS.escape(t.frag) + '"]');
          if (el) body.scrollTop = paneOffset(body, el) - 12;
        }
      })
      .catch(function (err) {
        alert("Could not open " + t.url + " (" + err.message + ")");
      });
  });

  // --- commit-all modal (index page) ------------------------------------------

  function initCommitAll() {
    var btn = document.getElementById("commit-all-btn");
    var modal = document.getElementById("commit-modal");
    if (!btn || !modal) return;
    var list = document.getElementById("commit-file-list");
    var stat = document.getElementById("commit-stat");
    var confirmBtn = document.getElementById("commit-confirm-btn");
    var status = document.getElementById("commit-modal-status");
    var busy = false;

    function closeModal() {
      if (busy) return; // commit session runs server-side; keep progress visible
      modal.hidden = true;
    }

    btn.addEventListener("click", function () {
      if (btn.disabled || busy) return;
      status.textContent = "";
      confirmBtn.disabled = true;
      confirmBtn.textContent = "Confirm commit";
      list.innerHTML = '<p class="muted">Checking…</p>';
      stat.textContent = "";
      modal.hidden = false;
      fetch("/api/git/status", { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          if (!j.repo || !j.changes || !j.changes.length) {
            list.innerHTML = '<p class="muted">Nothing to commit.</p>';
            return;
          }
          list.innerHTML = "";
          j.changes.forEach(function (c) {
            var row = document.createElement("div");
            row.className = "commit-file";
            var code = document.createElement("span");
            code.className = "commit-code";
            code.textContent = c.code;
            var path = document.createElement("span");
            path.className = "commit-path";
            path.textContent = c.path;
            row.appendChild(code);
            row.appendChild(path);
            if (c.added !== undefined && c.added !== "") {
              var counts = document.createElement("span");
              counts.className = "commit-counts";
              counts.textContent = "+" + c.added + " −" + c.deleted;
              row.appendChild(counts);
            }
            list.appendChild(row);
          });
          stat.textContent = j.summary || "";
          confirmBtn.disabled = false;
        })
        .catch(function () {
          list.innerHTML = '<p class="muted">Could not load git status.</p>';
        });
    });

    function setBusy(b) {
      busy = b;
      confirmBtn.disabled = b;
      if (b) {
        confirmBtn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Committing…';
        status.textContent = "";
      } else {
        confirmBtn.textContent = "Confirm commit";
      }
    }

    function poll(session, attempts) {
      if (attempts > 200) { // ~10 minutes max
        setBusy(false);
        status.textContent = "Commit is taking unusually long — check the session in Discussions.";
        return;
      }
      fetch("/api/git/commit-status?session=" + encodeURIComponent(session), {
        headers: { Accept: "application/json" },
      })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          if (j.done) {
            confirmBtn.innerHTML = "✓ Committed";
            btn.disabled = true; // tree is clean now
            setTimeout(function () {
              setBusy(false);
              modal.hidden = true;
              loadDiscussions(); // surface the commit session
            }, 2000);
          } else {
            setTimeout(function () { poll(session, attempts + 1); }, 3000);
          }
        })
        .catch(function () {
          setBusy(false);
          status.textContent = "Lost contact while waiting for the commit — check the session in Discussions.";
        });
    }

    confirmBtn.addEventListener("click", function () {
      if (confirmBtn.disabled || busy) return;
      setBusy(true);
      fetch("/api/git/commit", { method: "POST", headers: { Accept: "application/json" } })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function (j) { poll(j.session, 0); })
        .catch(function (e) {
          setBusy(false);
          status.textContent = e.message; // e.g. 422 "nothing to commit" race
        });
    });

    document.querySelector("[data-close-commit]").addEventListener("click", closeModal);
    modal.addEventListener("click", function (e) {
      if (e.target === modal) closeModal();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && !modal.hidden) closeModal();
    });
  }

  // --- settings page ---------------------------------------------------------

  function initSettings() {
    var root = document.getElementById("settings-page");
    if (!root) return;

    var view = null;
    var options = null;
    var scope = "project";

    var BOOL_DEFAULTS = {
      "session.autoOpenTerminal": true,
      "ui.showArchived": true,
      "docs.autoGardenerOnClose": true,
    };

    function getPath(obj, path) {
      return path.split(".").reduce(function (o, k) { return o == null ? undefined : o[k]; }, obj);
    }
    function isSet(v) { return v !== undefined && v !== null && v !== ""; }
    function boolLabel(v) { return v ? "On" : "Off"; }

    // What applies when the layer being edited leaves the field unset:
    // the other layer's value, else the built-in default.
    function fallbackFor(field) {
      var other = scope === "project" ? view.personal : view.project;
      var v = getPath(other, field);
      if (isSet(v)) return v;
      if (field === "docs.gardenerModel") return getPath(view.effective, "session.model");
      if (field in BOOL_DEFAULTS) return BOOL_DEFAULTS[field];
      return undefined;
    }

    function placeholderFor(field, fb) {
      if (field === "docs.gardenerModel") {
        return isSet(fb) ? "Session model (" + fb + ")" : "Service default";
      }
      if (isSet(fb)) return String(fb);
      if (field === "session.agent") return "Service default";
      if (field === "session.model") {
        return options && options.defaultModel ? "Service default (" + options.defaultModel + ")" : "Service default";
      }
      if (field === "git.defaultBranch") return "—";
      if (field.indexOf("prompts.") === 0) return "(no addendum)";
      return "";
    }

    function render() {
      var layer = scope === "project" ? view.project : view.personal;
      root.querySelectorAll(".settings-field").forEach(function (f) {
        var field = f.getAttribute("data-field");
        if (!field) return;
        var kind = f.getAttribute("data-kind");
        var input = f.querySelector("[data-input]");
        var badge = f.querySelector("[data-badge]");
        var lv = getPath(layer, field);
        var fb = fallbackFor(field);
        if (kind === "bool") {
          input.value = isSet(lv) ? String(lv) : "";
          var fbLabel = isSet(fb) ? boolLabel(fb) : boolLabel(BOOL_DEFAULTS[field]);
          input.options[0].textContent = "Inherit (" + fbLabel + ")";
        } else {
          input.value = isSet(lv) ? lv : "";
          input.placeholder = placeholderFor(field, fb);
        }
        var src = (view.sources && view.sources[field]) || "default";
        if (field === "docs.gardenerModel" && src === "default") {
          badge.textContent = "Session model";
        } else {
          badge.textContent = src.charAt(0).toUpperCase() + src.slice(1);
        }
        badge.classList.remove("src-default", "src-project", "src-personal");
        badge.classList.add("src-" + src);
      });
      var err = document.getElementById("settings-load-error");
      if (view.loadError) {
        err.textContent = "Settings file problem: " + view.loadError;
        err.hidden = false;
      } else {
        err.hidden = true;
      }
    }

    function renderOptions() {
      var hint = document.getElementById("settings-options-hint");
      if (!options || !options.available) {
        hint.hidden = false;
        return;
      }
      hint.hidden = true;
      var al = document.getElementById("settings-agent-list");
      (options.agents || []).forEach(function (a) {
        var o = document.createElement("option");
        o.value = a.id;
        o.label = a.name + (a.description ? " — " + a.description : "");
        al.appendChild(o);
      });
      var ml = document.getElementById("settings-model-list");
      (options.models || []).forEach(function (m) {
        var o = document.createElement("option");
        o.value = m.value;
        o.label = m.name;
        ml.appendChild(o);
      });
    }

    root.querySelectorAll('input[name="settings-scope"]').forEach(function (radio) {
      radio.addEventListener("change", function () {
        scope = radio.value;
        render();
      });
    });

    // Group navigation: one section visible at a time, tracked in the URL
    // hash so the active group survives a reload. Sections stay in the DOM
    // (hidden), so render/save logic above is unaffected.
    var nav = root.querySelector(".settings-nav");
    function showGroup(name) {
      root.querySelectorAll(".settings-section").forEach(function (sec) {
        sec.hidden = sec.getAttribute("data-section") !== name;
      });
      nav.querySelectorAll("button").forEach(function (b) {
        b.classList.toggle("active", b.getAttribute("data-group") === name);
      });
    }
    nav.querySelectorAll("button").forEach(function (b) {
      b.addEventListener("click", function () {
        var name = b.getAttribute("data-group");
        showGroup(name);
        if (history.replaceState) history.replaceState(null, "", "#" + name);
      });
    });
    var initial = (location.hash || "").slice(1);
    showGroup(nav.querySelector('button[data-group="' + initial + '"]') ? initial : "session");

    // Per-setting Change button: start (or reuse) the settings discussion
    // for exactly this setting. A deliberate click — always opens the
    // terminal, the auto-open gate does not apply.
    root.addEventListener("click", function (e) {
      var btn = e.target.closest(".settings-change");
      if (!btn || !root.contains(btn)) return;
      e.preventDefault();
      var field = btn.closest(".settings-field").getAttribute("data-field");
      btn.disabled = true;
      fetch("/api/settings/change", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ field: field, scope: scope }),
      })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function (j) { openTerminal(j.session, j.title); })
        .catch(function (err) { alert("Change request failed: " + err.message); })
        .finally(function () { btn.disabled = false; });
    });

    root.querySelectorAll("[data-save]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var sectionEl = btn.closest(".settings-section");
        var section = sectionEl.getAttribute("data-section");
        var status = sectionEl.querySelector(".settings-status");
        var payload = {};
        payload[section] = {};
        sectionEl.querySelectorAll(".settings-field").forEach(function (f) {
          var fieldAttr = f.getAttribute("data-field");
          if (!fieldAttr) return;
          var key = fieldAttr.split(".")[1];
          var kind = f.getAttribute("data-kind");
          var input = f.querySelector("[data-input]");
          if (kind === "bool") {
            payload[section][key] = input.value === "" ? null : input.value === "true";
          } else {
            payload[section][key] = input.value;
          }
        });
        btn.disabled = true;
        status.hidden = false;
        status.classList.remove("err");
        status.textContent = "Saving…";
        fetch("/api/settings?scope=" + scope, {
          method: "PUT",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: JSON.stringify(payload),
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function (j) {
            view = j;
            render();
            status.textContent = "Saved ✓";
            setTimeout(function () { status.hidden = true; }, 2500);
          })
          .catch(function (e) {
            status.textContent = e.message;
            status.classList.add("err");
          })
          .finally(function () { btn.disabled = false; });
      });
    });

    fetch("/api/settings", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) { view = j; render(); })
      .catch(function () {});
    fetch("/api/settings/options", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) { options = j; renderOptions(); render(); })
      .catch(function () {
        document.getElementById("settings-options-hint").hidden = false;
      });

    // Exclusions editor: a lazy folder tree like the onboarding wizard's
    // picker (same endpoint, same rows), persisted to agentsdocs.json.
    var exBox = document.getElementById("docs-exclusions");
    if (exBox) {
      var exStatus = document.getElementById("docs-exclusions-status");
      exBox.innerHTML = "";

      function exLoadRows(rel, afterRow, depth) {
        fetch("/api/setup/dirs?dir=" + encodeURIComponent(rel), { headers: { Accept: "application/json" } })
          .then(function (r) { return r.json(); })
          .then(function (j) {
            var dirs = j.dirs || [];
            if (!dirs.length && depth === 0) {
              exBox.innerHTML = '<span class="muted">No excludable directories.</span>';
            }
            dirs.forEach(function (d) {
              var row = exMakeRow(d, depth);
              if (afterRow) afterRow.parentNode.insertBefore(row, afterRow.nextSibling);
              else exBox.appendChild(row);
              afterRow = row;
            });
            if (j.hasConfig === false && depth === 0) {
              exStatus.textContent = "Docs coverage is disabled — run the onboarding wizard to enable it.";
            }
          })
          .catch(function () {
            if (depth === 0) exBox.innerHTML = '<span class="muted">Exclusions unavailable.</span>';
          });
      }

      function exMakeRow(d, depth) {
        var row = document.createElement("div");
        row.className = "setup-exclude";
        row.setAttribute("data-rel", d.rel);
        row.style.paddingLeft = (depth * 1.1 + 0.2) + "rem";
        var toggle = document.createElement("button");
        toggle.type = "button";
        toggle.className = "setup-exclude-toggle";
        if (d.hasChildren && !d.defaultExcluded) {
          toggle.textContent = "▸";
          toggle.title = "Expand";
          toggle.addEventListener("click", function () { exToggleRow(row, toggle, d, depth); });
        } else {
          toggle.classList.add("empty");
          toggle.disabled = true;
          toggle.tabIndex = -1;
        }
        row.appendChild(toggle);
        var label = document.createElement("label");
        label.className = "setup-exclude-name";
        var cb = document.createElement("input");
        cb.type = "checkbox";
        cb.value = d.rel;
        if (d.defaultExcluded) {
          cb.checked = true;
          cb.disabled = true;
          row.classList.add("default-excluded");
        } else if (d.excluded) {
          cb.checked = true;
        }
        cb.addEventListener("change", exImplied);
        label.appendChild(cb);
        label.appendChild(document.createTextNode(d.name + (d.defaultExcluded ? " (built-in)" : "")));
        row.appendChild(label);
        return row;
      }

      function exToggleRow(row, toggle, d, depth) {
        if (row.getAttribute("data-loaded") !== "1") {
          row.setAttribute("data-loaded", "1");
          toggle.textContent = "▾";
          exLoadRows(d.rel, row, depth + 1);
          return;
        }
        var collapse = row.getAttribute("data-collapsed") !== "1";
        row.setAttribute("data-collapsed", collapse ? "1" : "0");
        toggle.textContent = collapse ? "▸" : "▾";
        exDescendants(row).forEach(function (r) { r.style.display = collapse ? "none" : ""; });
      }

      function exDescendants(row) {
        var prefix = row.getAttribute("data-rel") + "/";
        var out = [];
        exBox.querySelectorAll('.setup-exclude[data-rel]').forEach(function (r) {
          if (r !== row && r.getAttribute("data-rel").indexOf(prefix) === 0) out.push(r);
        });
        return out;
      }

      // Descendants of a checked row are implied (the parent's pattern
      // prunes the subtree) and are not submitted separately.
      function exImplied() {
        exBox.querySelectorAll('.setup-exclude[data-rel]').forEach(function (r) { r.classList.remove("implied"); });
        exBox.querySelectorAll('.setup-exclude[data-rel]').forEach(function (r) {
          var cb = r.querySelector('input[type="checkbox"]');
          if (cb.checked && !cb.disabled) exDescendants(r).forEach(function (d) { d.classList.add("implied"); });
        });
      }

      function exSelected() {
        var out = [];
        exBox.querySelectorAll('.setup-exclude[data-rel]').forEach(function (r) {
          var cb = r.querySelector('input[type="checkbox"]');
          if (cb.checked && !cb.disabled && !r.classList.contains("implied")) out.push(cb.value);
        });
        return out;
      }

      exLoadRows("", null, 0);

      var exSave = document.getElementById("docs-exclusions-save");
      exSave.addEventListener("click", function () {
        exSave.disabled = true;
        exStatus.textContent = "Saving…";
        fetch("/docs/exclusions", {
          method: "POST",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: JSON.stringify({ excludeDirs: exSelected() }),
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function (j) {
            exStatus.textContent = j.saved ? "Saved ✓" : "No changes.";
            setTimeout(function () { exStatus.textContent = ""; }, 2500);
          })
          .catch(function (e) {
            exStatus.textContent = e.message;
          })
          .finally(function () { exSave.disabled = false; });
      });
    }
  }

  // --- onboarding banner (index) --------------------------------------------

  function initOnboardingBanner() {
    var b = document.getElementById("onboarding-banner");
    if (!b) return;
    var d = document.getElementById("onboarding-dismiss");
    d.addEventListener("click", function () {
      d.disabled = true;
      fetch("/api/setup/dismiss", { method: "POST", headers: { Accept: "application/json" } })
        .then(function (r) {
          if (r.ok) b.hidden = true;
          else d.disabled = false;
        })
        .catch(function () { d.disabled = false; });
    });
  }

  // --- index table sorting --------------------------------------------------

  // First-click direction per sortable column: text columns start ascending,
  // Updated and Tasks start descending (newest / most tasks first).
  var indexSortCols = {
    id: "asc", title: "asc", prefix: "asc", status: "asc",
    tasks: "desc", updated: "desc"
  };
  var INDEX_SORT_KEY = "tt-index-sort";

  function indexSortState() {
    // Restores {col, dir} from localStorage; anything corrupt or unknown
    // falls back to the server's default order (no override).
    try {
      var raw = JSON.parse(localStorage.getItem(INDEX_SORT_KEY) || "null");
      if (raw && indexSortCols[raw.col] && (raw.dir === "asc" || raw.dir === "desc")) {
        return raw;
      }
    } catch (_) {}
    return null;
  }

  function indexCellKey(tr, col) {
    if (col === "tasks") return parseInt(tr.getAttribute("data-tasks"), 10) || 0;
    if (col === "status") return parseInt(tr.getAttribute("data-status-rank"), 10) || 0;
    if (col === "updated") return tr.getAttribute("data-updated") || "";
    var cell = tr.querySelector('td[data-col="' + col + '"]');
    return cell ? cell.textContent.trim() : "";
  }

  function applyIndexSort(col, dir) {
    var table = document.querySelector(".change-table");
    if (!table) return;
    var tbody = table.tBodies[0];
    if (!tbody) return;
    var rows = Array.prototype.slice.call(tbody.rows);
    var mul = dir === "asc" ? 1 : -1;
    // Array#sort is stable, so equal keys keep the server's default order.
    rows.sort(function (a, b) {
      var ka = indexCellKey(a, col), kb = indexCellKey(b, col);
      if (ka < kb) return -mul;
      if (ka > kb) return mul;
      return 0;
    });
    rows.forEach(function (tr) { tbody.appendChild(tr); });

    table.querySelectorAll("thead th").forEach(function (th) {
      th.removeAttribute("aria-sort");
    });
    table.querySelectorAll(".sort-btn").forEach(function (btn) {
      btn.classList.remove("sort-asc", "sort-desc");
    });
    var th = table.querySelector('thead th[data-col="' + col + '"]');
    if (th) th.setAttribute("aria-sort", dir === "asc" ? "ascending" : "descending");
    var btn = table.querySelector('.sort-btn[data-sort-col="' + col + '"]');
    if (btn) btn.classList.add(dir === "asc" ? "sort-asc" : "sort-desc");
  }

  function initIndexSort() {
    var table = document.querySelector(".change-table");
    if (!table) return;
    table.querySelectorAll(".sort-btn").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var col = btn.getAttribute("data-sort-col");
        if (!indexSortCols[col]) return;
        var cur = indexSortState();
        var dir = (cur && cur.col === col)
          ? (cur.dir === "asc" ? "desc" : "asc")
          : indexSortCols[col];
        try {
          localStorage.setItem(INDEX_SORT_KEY, JSON.stringify({ col: col, dir: dir }));
        } catch (_) {}
        applyIndexSort(col, dir);
      });
    });
    var stored = indexSortState();
    if (stored) applyIndexSort(stored.col, stored.dir);
  }

  // --- setup wizard ---------------------------------------------------------

  function initSetup() {
    var root = document.getElementById("setup-page");
    if (!root) return;

    var state = {
      checks: [],
      ready: false,
      coverage: false,       // docs coverage enabled (checks or bootstrap choice)
      changesPresent: false, // repo already bootstrapped on load
      bootstrapped: false,   // bootstrap ran this session
      excludedCount: 0,      // dirs excluded from coverage at bootstrap
      agentSaved: false,
      agentSkipped: false,
      docsOutcome: "",       // "seeded" | "skipped" | "pending" (failed)
    };

    var steps = ["prereqs", "bootstrap", "agent", "docs", "finish"];

    function el(id) { return document.getElementById(id); }

    function showError(msg) {
      var e = el("setup-error");
      e.textContent = msg || "";
      e.hidden = !msg;
    }

    function visibleSteps() {
      return steps.filter(function (s) { return s !== "docs" || state.coverage; });
    }

    function showStep(name) {
      root.querySelectorAll(".setup-step").forEach(function (sec) {
        sec.hidden = sec.getAttribute("data-step") !== name;
      });
      var vis = visibleSteps();
      root.querySelectorAll("#setup-steps-nav li").forEach(function (li) {
        var n = li.getAttribute("data-step-nav");
        li.classList.toggle("active", n === name);
        li.classList.toggle("done", vis.indexOf(n) > -1 && vis.indexOf(n) < vis.indexOf(name));
      });
      el("setup-step-indicator").textContent = "Step " + (vis.indexOf(name) + 1) + " of " + vis.length;
      showError("");
      if (name === "bootstrap") enterBootstrap();
      if (name === "agent" && !optionsLoaded) loadAgentOptions();
      if (name === "finish") renderFinish();
    }

    // --- step 1: prerequisites ---

    function renderPrereqs() {
      var ul = el("setup-prereq-list");
      ul.innerHTML = "";
      state.checks.forEach(function (c) {
        var li = document.createElement("li");
        li.className = "setup-prereq";
        var pill = document.createElement("span");
        pill.className = "pill prereq-" + c.status;
        pill.textContent = c.status.toUpperCase();
        var body = document.createElement("div");
        body.className = "setup-prereq-body";
        var name = document.createElement("strong");
        name.textContent = c.name;
        body.appendChild(name);
        if (c.detail) {
          var d = document.createElement("div");
          d.className = "muted";
          d.textContent = c.detail;
          body.appendChild(d);
        }
        if (c.remedy) {
          var r = document.createElement("div");
          r.className = "setup-remedy";
          r.textContent = "→ " + c.remedy;
          body.appendChild(r);
        }
        li.appendChild(pill);
        li.appendChild(body);
        ul.appendChild(li);
      });
      el("setup-prereqs-next").disabled = !state.ready;
    }

    function loadPrereqs() {
      el("setup-prereqs-next").disabled = true;
      return fetch("/api/setup/prereqs", { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          state.checks = j.checks || [];
          state.ready = !!j.ready;
          state.checks.forEach(function (c) {
            if (c.id === "changes-present" && c.status === "ok") state.changesPresent = true;
            if (c.id === "docs-coverage" && c.status === "ok") state.coverage = true;
          });
          renderPrereqs();
        })
        .catch(function () { showError("Could not reach the setup API."); });
    }

    el("setup-recheck-btn").addEventListener("click", function () { loadPrereqs(); });
    el("setup-prereqs-next").addEventListener("click", function () { showStep("bootstrap"); });

    // --- step 2: bootstrap ---

    function enterBootstrap() {
      loadDirs();
      if (state.changesPresent && !state.bootstrapped) {
        el("setup-bootstrap-done").hidden = false;
        el("setup-bootstrap-next").hidden = false;
        // The Bootstrap button stays visible: init is idempotent, and a
        // submission updates the coverage exclusions of an existing config.
      }
    }

    // The exclusion picker is a lazy tree: each row loads its children on
    // expand. Checking a row marks its loaded descendants as implied (the
    // parent's pattern already prunes the subtree); implied rows are not
    // submitted, and the server also normalizes redundant nested patterns.
    var dirsLoaded = false;

    function loadDirs() {
      if (dirsLoaded) return;
      dirsLoaded = true;
      var box = el("setup-exclude-list");
      box.innerHTML = "";
      loadDirRows("", box, 0, null);
    }

    function loadDirRows(rel, box, depth, afterRow) {
      fetch("/api/setup/dirs?dir=" + encodeURIComponent(rel), { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          var dirs = j.dirs || [];
          if (!dirs.length && depth === 0) {
            box.innerHTML = '<p class="muted">No candidate directories.</p>';
            return;
          }
          var ref = afterRow;
          dirs.forEach(function (d) {
            var row = makeDirRow(d, depth);
            if (ref) {
              ref.parentNode.insertBefore(row, ref.nextSibling);
              ref = row;
            } else {
              box.appendChild(row);
            }
          });
          refreshImplied();
        })
        .catch(function () {});
    }

    function makeDirRow(d, depth) {
      var row = document.createElement("div");
      row.className = "setup-exclude";
      row.setAttribute("data-rel", d.rel);
      row.style.paddingLeft = (depth * 1.1 + 0.2) + "rem";

      var toggle = document.createElement("button");
      toggle.type = "button";
      toggle.className = "setup-exclude-toggle";
      if (d.hasChildren && !d.defaultExcluded) {
        toggle.textContent = "▸";
        toggle.title = "Expand";
        toggle.addEventListener("click", function () { toggleDirRow(row, toggle, d, depth); });
      } else {
        toggle.classList.add("empty");
        toggle.disabled = true;
        toggle.tabIndex = -1;
      }
      row.appendChild(toggle);

      var label = document.createElement("label");
      label.className = "setup-exclude-name";
      var cb = document.createElement("input");
      cb.type = "checkbox";
      cb.value = d.rel;
      if (d.defaultExcluded) {
        cb.checked = true;
        cb.disabled = true;
        row.classList.add("default-excluded");
      } else if (d.excluded) {
        cb.checked = true;
      }
      cb.addEventListener("change", refreshImplied);
      label.appendChild(cb);
      label.appendChild(document.createTextNode(d.name + (d.defaultExcluded ? " (built-in)" : "")));
      row.appendChild(label);
      return row;
    }

    function toggleDirRow(row, toggle, d, depth) {
      if (row.getAttribute("data-loaded") !== "1") {
        row.setAttribute("data-loaded", "1");
        toggle.textContent = "▾";
        loadDirRows(d.rel, row.parentNode, depth + 1, row);
        return;
      }
      var collapse = row.getAttribute("data-collapsed") !== "1";
      row.setAttribute("data-collapsed", collapse ? "1" : "0");
      toggle.textContent = collapse ? "▸" : "▾";
      descendantRows(row).forEach(function (r) { r.style.display = collapse ? "none" : ""; });
    }

    function descendantRows(row) {
      var prefix = row.getAttribute("data-rel") + "/";
      var out = [];
      row.parentNode.querySelectorAll('.setup-exclude[data-rel]').forEach(function (r) {
        if (r !== row && r.getAttribute("data-rel").indexOf(prefix) === 0) out.push(r);
      });
      return out;
    }

    // Descendants of a checked row are visually implied: the parent's
    // pattern prunes them whether or not they are checked themselves.
    function refreshImplied() {
      var rows = [];
      el("setup-exclude-list").querySelectorAll('.setup-exclude[data-rel]').forEach(function (r) { rows.push(r); });
      rows.forEach(function (r) { r.classList.remove("implied"); });
      rows.forEach(function (r) {
        var cb = r.querySelector('input[type="checkbox"]');
        if (cb.checked && !cb.disabled) {
          descendantRows(r).forEach(function (d) { d.classList.add("implied"); });
        }
      });
    }

    function selectedExcludes() {
      var out = [];
      el("setup-exclude-list").querySelectorAll('.setup-exclude[data-rel]').forEach(function (r) {
        var cb = r.querySelector('input[type="checkbox"]');
        if (cb.checked && !cb.disabled && !r.classList.contains("implied")) out.push(cb.value);
      });
      return out;
    }

    el("setup-coverage").addEventListener("change", function () {
      el("setup-excludes").hidden = !el("setup-coverage").checked;
    });

    el("setup-bootstrap-btn").addEventListener("click", function () {
      var btn = el("setup-bootstrap-btn");
      btn.disabled = true;
      showError("");
      var coverage = el("setup-coverage").checked;
      var excludeDirs = coverage ? selectedExcludes() : [];
      fetch("/api/setup/bootstrap", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ docsCoverage: coverage, excludeDirs: excludeDirs }),
      })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function (j) {
          state.bootstrapped = true;
          state.coverage = coverage;
          state.excludedCount = excludeDirs.length;
          var ul = el("setup-bootstrap-result");
          (j.actions || []).forEach(function (a) {
            var li = document.createElement("li");
            li.textContent = a.Action + "  " + a.Path;
            li.className = "artifact-" + a.Action;
            ul.appendChild(li);
          });
          btn.hidden = true;
          el("setup-bootstrap-done").hidden = false;
          el("setup-bootstrap-next").hidden = false;
        })
        .catch(function (e) { showError("Bootstrap failed: " + e.message); })
        .finally(function () { btn.disabled = false; });
    });
    el("setup-bootstrap-next").addEventListener("click", function () { showStep("agent"); });

    // --- step 3: default agent/model ---

    // The setup shell only holds an opencode client after the prereq check
    // discovers the service, so options load strictly after prereqs — and
    // are re-fetched when the agent step is entered if the first attempt
    // came back unavailable.
    var optionsLoaded = false;

    function loadAgentOptions() {
      return fetch("/api/settings/options", { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          if (!j.available) { el("setup-options-hint").hidden = false; return; }
          optionsLoaded = true;
          el("setup-options-hint").hidden = true;
          var al = el("setup-agent-list");
          al.innerHTML = "";
          (j.agents || []).forEach(function (a) {
            var o = document.createElement("option");
            o.value = a.id;
            o.label = a.name + (a.description ? " — " + a.description : "");
            al.appendChild(o);
          });
          var ml = el("setup-model-list");
          ml.innerHTML = "";
          (j.models || []).forEach(function (m) {
            var o = document.createElement("option");
            o.value = m.value;
            o.label = m.name;
            ml.appendChild(o);
          });
          if (j.defaultModel) {
            el("setup-model").placeholder = "Service default (" + j.defaultModel + ")";
          }
        })
        .catch(function () { el("setup-options-hint").hidden = false; });
    }

    function setupScope() {
      var r = root.querySelector('input[name="setup-scope"]:checked');
      return r ? r.value : "personal";
    }

    el("setup-agent-save").addEventListener("click", function () {
      var status = el("setup-agent-status");
      var payload = { session: {
        agent: el("setup-agent").value.trim(),
        model: el("setup-model").value.trim(),
      } };
      status.textContent = "Saving…";
      fetch("/api/settings?scope=" + setupScope(), {
        method: "PUT",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify(payload),
      })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function () {
          state.agentSaved = true;
          status.textContent = "Saved ✓";
          showStep(state.coverage ? "docs" : "finish");
        })
        .catch(function (e) { status.textContent = e.message; });
    });
    el("setup-agent-skip").addEventListener("click", function () {
      state.agentSkipped = true;
      showStep(state.coverage ? "docs" : "finish");
    });

    // --- step 4: docs seeding (opt-in) ---

    function pollSeed() {
      fetch("/api/setup/docs-seed-status", { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          var log = el("setup-seed-log");
          log.hidden = false;
          log.textContent = (j.lines || []).join("\n");
          log.scrollTop = log.scrollHeight;
          if (!j.done) {
            setTimeout(pollSeed, 1000);
            return;
          }
          el("setup-seed-btn").disabled = false;
          el("setup-seed-next").hidden = false;
          el("setup-seed-skip").hidden = true;
          if (j.error) {
            state.docsOutcome = "pending";
            showError("Docs generation finished with failures: " + j.error + " — you can re-run it later.");
          } else {
            state.docsOutcome = "seeded";
          }
        })
        .catch(function () { setTimeout(pollSeed, 2000); });
    }

    el("setup-seed-btn").addEventListener("click", function () {
      var budget = parseInt(el("setup-seed-budget").value, 10);
      if (isNaN(budget) || budget < 0) budget = 0;
      var btn = el("setup-seed-btn");
      btn.disabled = true;
      showError("");
      fetch("/api/setup/docs-seed", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ budget: budget }),
      })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function () { setTimeout(pollSeed, 500); })
        .catch(function (e) {
          btn.disabled = false;
          showError("Could not start docs generation: " + e.message);
        });
    });
    el("setup-seed-skip").addEventListener("click", function () {
      state.docsOutcome = "skipped";
      showStep("finish");
    });
    el("setup-seed-next").addEventListener("click", function () { showStep("finish"); });

    // --- step 5: finish ---

    function renderFinish() {
      var ul = el("setup-finish-summary");
      ul.innerHTML = "";
      var items = [];
      items.push(state.bootstrapped ? "Repository bootstrapped" : "Repository was already initialized");
      items.push(state.agentSaved ? "Default agent/model saved" : "Using the service's default agent/model");
      if (state.coverage) {
        var exNote = state.excludedCount ? " — " + state.excludedCount + " director" + (state.excludedCount === 1 ? "y" : "ies") + " excluded" : "";
        if (state.docsOutcome === "seeded") items.push("Agent-facing docs generated" + exNote);
        else if (state.docsOutcome === "skipped") items.push("Docs generation skipped — seed later anytime" + exNote);
        else items.push("Docs generation pending — seed later via Settings or the docs bell" + exNote);
      } else {
        items.push("Docs coverage disabled — enable later by re-running onboarding");
      }
      items.forEach(function (t) {
        var li = document.createElement("li");
        li.textContent = t;
        ul.appendChild(li);
      });
    }

    el("setup-finish-btn").addEventListener("click", function () {
      var btn = el("setup-finish-btn");
      btn.disabled = true;
      var stepMarks = {};
      if (state.agentSaved) stepMarks.agent = "set";
      else if (state.agentSkipped) stepMarks.agent = "skipped";
      if (state.docsOutcome === "skipped") stepMarks.docs = "skipped";
      fetch("/api/setup/complete", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({ steps: stepMarks }),
      })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function () { window.location.href = "/"; })
        .catch(function (e) {
          showError("Could not record completion: " + e.message);
          btn.disabled = false;
        });
    });

    loadPrereqs().then(loadAgentOptions);
  }

  // --- init -----------------------------------------------------------------

  document.addEventListener("DOMContentLoaded", function () {
    initTheme();
    initSortable();
    initSessions();
    initContinue();
    initLifecycle();
    initCommitAll();
    initIndexSort();
    initSettings();
    initSetup();
    initOnboardingBanner();
    checkValidation();
    autoOpenSession();
    loadDiscussions();
  });
})();
