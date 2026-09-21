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

  function narrowSessionUI() {
    return window.matchMedia && window.matchMedia("(max-width: 840px)").matches;
  }

  function openPreferredSession(sessionID, title) {
    if (narrowSessionUI()) openChat(sessionID, title);
    else openTerminal(sessionID, title);
  }

  function maybeOpenSession(sessionID, title) {
    if (autoOpenTerminal) openPreferredSession(sessionID, title);
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
            headers: { "Content-Type": "application/json", Accept: "application/json", "X-Lessmess-UI": "1" },
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
    // Keep the drill-down task in the URL so SSE refreshes stay scoped.
    htmx.ajax("GET", location.pathname + location.search, { target: "#board", swap: "innerHTML", headers: { Accept: "text/html" } });
  }

  document.addEventListener("htmx:afterSwap", function (e) {
    if (e.target && e.target.id === "board") {
      initSortable();
      if (boardEl()) syncTerminalTasks(boardEl(), boardEl().dataset.change);
      if (chatOpen() && boardEl()) setChatTasks(boardEl(), boardEl().dataset.change);
      refreshSubs();
    }
  });

  // After a successful htmx form POST, run form-specific follow-ups.
  document.addEventListener("htmx:afterRequest", function (e) {
    if (!e.detail.successful) {
      var msg = "Request failed";
      try { msg = JSON.parse(e.detail.xhr.responseText).error || msg; } catch (_) {}
      alert(msg);
      return;
    }
    if (e.detail.elt.matches('form[hx-post="/changes/session"]')) {
      try {
        var j = JSON.parse(e.detail.xhr.responseText);
        loadDiscussions();
        maybeOpenSession(j.session, j.title);
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
      else if (page === "index" && sessionOverlayOpen() && activeSessionID()) followSession();
      else if (page === "index") location.reload();
      // Terminal task panel: re-mirror an open panel, and pick up a
      // fresh binding (a discussion that just scaffolded a change —
      // the scaffold writes changes/, which fired this event).
      if (sessionOverlayOpen()) {
        if (terminalPanelChange) loadTerminalTasks(terminalPanelChange);
        else if (!(page === "board" && boardEl()) && activeSessionID()) resolveTerminalPanel(activeSessionID());
      }
    }, 250);
  }

  // followSession: the open terminal's session was likely just scaffolded
  // into a change. Once the mapping says so, navigate to that change's
  // board with ?session= — autoOpenSession reopens the same session there
  // with its task panel. Until it binds, skip the refresh entirely; the
  // stale list self-heals on navigation or when the terminal closes.
  function followSession() {
    var sessionID = activeSessionID();
    fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/change", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        if (!j || !j.change) return;
        if (!sessionOverlayOpen() || activeSessionID() !== sessionID) return;
        location.assign("/changes/" + encodeURIComponent(j.change) + "?session=" + encodeURIComponent(sessionID));
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
      .then(function (j) { openChat(j.session, j.title); })
      .catch(function (err) { alert(err.message); })
      .finally(function () { btn.disabled = false; });
  }, true);

  // --- header chat button ---------------------------------------------------

  // The header Chat button (every page) spawns a general codebase chat —
  // a free agent, not bound to any change — and opens the Chat overlay.
  // A deliberate open, so autoOpenTerminal never gates it.
  document.addEventListener("click", function (e) {
    var btn = e.target.closest("#chat-btn");
    if (!btn) return;
    btn.disabled = true;
    fetch("/chat/session", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: "{}",
    })
      .then(function (r) {
        return r.json().then(function (j) {
          if (!r.ok) throw new Error(j.error || "chat failed");
          return j;
        });
      })
      .then(function (j) { openChat(j.session, j.title); })
      .catch(function (err) { alert(err.message); })
      .finally(function () { btn.disabled = false; });
  });

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
          var chatBtn = document.createElement("button");
          chatBtn.className = "btn-ghost";
          chatBtn.textContent = "Chat";
          chatBtn.addEventListener("click", function () { openChat(s.session, s.title); });
          var openBtn = document.createElement("button");
          openBtn.className = "btn-ghost";
          openBtn.textContent = "Terminal";
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
          actions.appendChild(chatBtn);
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

  // The board's drill-down task ("" on the change's root board).
  function boardTask() {
    var b = boardEl();
    return b ? b.getAttribute("data-task") || "" : "";
  }

  // Last-opened session per board scope: change root or drilled task.
  function lastSessionKey() {
    var t = boardTask();
    return "tt-last-session:" + changeID() + (t ? "/" + t : "");
  }
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
      renderSubs(sessions);
      var scoped = boardSessions(sessions);
      if (!scoped || !scoped.length) {
        var empty = document.createElement("li");
        empty.className = "session-empty";
        empty.textContent = boardTask()
          ? "No sessions bound to this task yet — start one."
          : "No sessions yet — start one.";
        ul.appendChild(empty);
        return;
      }
      scoped.forEach(function (s) {
        var li = document.createElement("li");
        li.className = "session-item";
        var title = document.createElement("span");
        title.className = "session-title";
        title.textContent = s.title;
        var meta = document.createElement("span");
        meta.className = "session-meta";
        meta.textContent = (s.created || "").slice(0, 10);
        if (s.spawnedFrom) {
          var badge = document.createElement("span");
          badge.className = "spawn-badge";
          badge.textContent = "from " + s.spawnedFrom;
          badge.title = "Spawned by a handoff from change " + s.spawnedFrom;
          meta.appendChild(document.createTextNode(" "));
          meta.appendChild(badge);
        }
        var actions = document.createElement("span");
        actions.className = "session-actions";
        var chatBtn = document.createElement("button");
        chatBtn.className = "btn-ghost";
        chatBtn.textContent = "Chat";
        chatBtn.addEventListener("click", function () { markOpened(s.session); openChat(s.session, s.title); });
        var openBtn = document.createElement("button");
        openBtn.className = "btn-ghost";
        openBtn.textContent = "Terminal";
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
        actions.appendChild(chatBtn);
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

  // subChip is one subagent session with explicit Chat and Terminal actions.
  // Dead sessions
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
    talk.textContent = "Chat";
    talk.title = s.live
      ? "Open Chat on this subagent session"
      : "Session not found in the opencode service";
    talk.addEventListener("click", function () { markOpened(s.session); openChat(s.session, s.title); });
    chip.appendChild(talk);
    var terminal = document.createElement("button");
    terminal.type = "button";
    terminal.className = "btn-ghost sub-talk";
    terminal.textContent = "Terminal";
    terminal.addEventListener("click", function () { markOpened(s.session); openTerminal(s.session, s.title); });
    chip.appendChild(terminal);
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
        createSessionAndOpen(nb, maybeOpenSession);
      });
    }
    initSpawnChange();
  }

  // --- spawn change (handoff) -------------------------------------------------

  // initSpawnChange wires the "Spawn change" action: pick a handoff*.md
  // artifact authored in this change, and spawn a new change whose fresh
  // session is seeded from it. The picker reloads every time the form
  // opens (artifacts are written by sessions or by hand while the board
  // is open).
  function initSpawnChange() {
    var btn = document.getElementById("spawn-change-btn");
    var form = document.getElementById("spawn-change-form");
    if (!btn || !form) return;
    var sel = document.getElementById("spawn-artifact");
    var errEl = document.getElementById("spawn-error");

    function spawnError(msg) {
      errEl.textContent = msg || "";
      errEl.hidden = !msg;
    }

    function loadHandoffs() {
      spawnError("");
      sel.innerHTML = "";
      fetch("/changes/" + changeID() + "/handoffs", { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function (j) {
          var files = j.handoffs || [];
          if (!files.length) {
            var opt = document.createElement("option");
            opt.value = "";
            opt.textContent = "no handoff-*.md artifacts in this change";
            sel.appendChild(opt);
            sel.disabled = true;
            return;
          }
          sel.disabled = false;
          files.forEach(function (f) {
            var opt = document.createElement("option");
            opt.value = f;
            opt.textContent = f;
            sel.appendChild(opt);
          });
        })
        .catch(function (e) { spawnError("Load handoffs failed: " + e.message); });
    }

    btn.addEventListener("click", function () {
      form.hidden = !form.hidden;
      if (!form.hidden) loadHandoffs();
    });

    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var title = document.getElementById("spawn-title").value.trim();
      var prefix = document.getElementById("spawn-prefix").value.trim();
      var artifact = sel.value;
      if (!title || !artifact) return;
      var body = { title: title, artifact: artifact };
      if (prefix) body.prefix = prefix;
      var submit = form.querySelector("button[type=submit]");
      submit.disabled = true;
      fetch("/changes/" + changeID() + "/spawn-change", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify(body),
      })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function (j) {
          form.hidden = true;
          form.reset();
          spawnError("");
          openPreferredSession(j.session, changeID() + " — " + j.title);
        })
        .catch(function (e) { spawnError("Spawn failed: " + e.message); })
        .finally(function () { submit.disabled = false; });
    });
  }

  // createSessionAndOpen POSTs a session for the board scope (task-bound
  // on sub-boards) and hands it to onCreated.
  function createSessionAndOpen(btn, onCreated) {
    var t = boardTask();
    fetch("/changes/" + changeID() + "/sessions", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify(t ? { task: t } : {}),
    })
      .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
      .then(function (s) { markOpened(s.session); loadSessions(); onCreated(s.session, s.title); })
      .catch(function (e) { alert("Create session failed: " + e.message); });
  }

  // boardSessions filters a session list to the board scope: on a
  // sub-board only sessions bound to that exact task (plus its auto-spawn
  // and delegated children share the task annotation).
  function boardSessions(sessions) {
    var t = boardTask();
    if (!t) return sessions;
    return (sessions || []).filter(function (s) { return s.task === t; });
  }

  // --- continue / start session button ---------------------------------------

  // One-click resume: opens the last-opened session (validated against the
  // live list, falling back to the newest created), or creates + opens a
  // session when the change has none.
  function initContinue() {
    var btn = document.getElementById("continue-session-btn");
    if (!btn) return;
    fetchSessions(function (sessions) {
      if (sessions) {
        var scoped = boardSessions(sessions);
        btn.textContent = scoped.length ? "Continue session" : "Start session";
      }
      renderSubs(sessions);
    });
    btn.addEventListener("click", function () {
      if (btn.disabled) return;
      fetchSessions(function (sessions) {
        if (!sessions) { alert("Could not load sessions"); return; }
        var scoped = boardSessions(sessions);
        if (!scoped.length) { createSessionAndOpen(btn, function (sid, title) { btn.textContent = "Continue session"; maybeOpenSession(sid, title); }); return; }
        var stored = null;
        try { stored = localStorage.getItem(lastSessionKey()); } catch (_) {}
        var s = scoped.find(function (x) { return x.session === stored; }) || scoped[scoped.length - 1];
        markOpened(s.session);
        openPreferredSession(s.session, s.title);
      });
    });
  }

  // --- chat overlay ---------------------------------------------------------

  var cstate = {
    session: null,
    title: "",
    timer: null,
    request: null,
    mutation: false,
    snapshot: null,
    drafts: {},
    files: {},
    references: {},
    referenceCatalog: [],
    controls: null,
    skills: {},
    scrolls: {},
    restoreScroll: null,
    lifecycle: null,
    lifecycleAction: false,
    lifecycleError: "",
    revertTarget: null,
    revertPreviewHTML: "",
    revertPreviewValid: false,
    compactPending: {},
    inbox: [],
    inboxRequest: null,
    deliveryIDs: {},
    navigation: null,
    navigationRequest: null,
    usageTimer: null,
    usageRequest: null,
    deletePreview: null,
    viewStack: ["chat"],
    viewScrolls: {},
    viewFocus: [],
    detailOpener: null,
    detailViewportHandler: null,
    detailTOCMedia: null,
    detailTOCHandler: null,
    opener: null,
    auxiliaryFocus: {},
    viewportHandler: null,
  };

  function compactChatUI() {
    return window.matchMedia && window.matchMedia("(max-width: 840px)").matches;
  }

  function closeChatMessageActions(returnFocus) {
    var menu = document.querySelector("#chat-transcript .chat-message-menu:not([hidden])");
    if (!menu) return false;
    var message = menu.closest("[data-chat-message-actions]");
    menu.hidden = true;
    if (message) {
      message.setAttribute("aria-expanded", "false");
      if (returnFocus) message.focus({ preventScroll: true });
    }
    return true;
  }

  function openChatMessageActions(message, focusMenu) {
    var menu = message && message.querySelector(":scope > .chat-message-menu");
    if (!menu) return;
    closeChatMessageActions(false);
    menu.hidden = false;
    message.setAttribute("aria-expanded", "true");
    if (focusMenu) {
      var first = menu.querySelector("button:not([disabled])");
      if (first) first.focus({ preventScroll: true });
    }
  }

  function chatViewPanel(name) {
    return document.querySelector('.chat-window [data-chat-view-panel="' + name + '"]');
  }

  function chatViewScroller(name) {
    var panel = chatViewPanel(name);
    if (!panel) return null;
    if (name === "chat") return document.getElementById("chat-transcript");
    return panel.querySelector("[data-chat-view-scroll], .ttp-scroll") || panel;
  }

  function saveChatViewPosition(name) {
    if (!cstate.session) return;
    var scroller = chatViewScroller(name);
    if (!scroller) return;
    if (!cstate.viewScrolls[cstate.session]) cstate.viewScrolls[cstate.session] = {};
    cstate.viewScrolls[cstate.session][name] = scroller.scrollTop;
    if (name === "chat") cstate.scrolls[cstate.session] = scroller.scrollTop;
  }

  function restoreChatViewPosition(name) {
    if (!cstate.session) return;
    var positions = cstate.viewScrolls[cstate.session] || {};
    var top = Object.prototype.hasOwnProperty.call(positions, name) ? positions[name] : null;
    if (top === null && name === "chat" && Object.prototype.hasOwnProperty.call(cstate.scrolls, cstate.session)) top = cstate.scrolls[cstate.session];
    if (top === null) return;
    requestAnimationFrame(function () {
      var scroller = chatViewScroller(name);
      if (scroller) scroller.scrollTop = top;
    });
  }

  function syncChatView(focusView) {
    var win = document.querySelector(".chat-window");
    if (!win) return;
    if (!compactChatUI()) {
      win.dataset.chatView = "chat";
      win.querySelectorAll("[data-chat-view-panel]").forEach(function (panel) {
        var name = panel.dataset.chatViewPanel;
        var selected = name === "chat" || (name === "work" && !panel.hidden && panel.classList.contains("open")) ||
          (name === "agents" && !panel.hidden && panel.classList.contains("open")) || (name === "controls" && !panel.hidden);
        panel.setAttribute("aria-hidden", String(!selected));
        panel.inert = !selected;
      });
      document.getElementById("chat-tasks-btn").setAttribute("aria-expanded", String(document.getElementById("chat-tasks").classList.contains("open")));
      document.getElementById("chat-agents-btn").setAttribute("aria-expanded", String(document.getElementById("chat-agents").classList.contains("open")));
      document.getElementById("chat-controls-btn").setAttribute("aria-expanded", String(!document.getElementById("chat-controls-sheet").hidden));
      return;
    }
    var active = cstate.viewStack[cstate.viewStack.length - 1] || "chat";
    win.dataset.chatView = active;
    win.querySelectorAll("[data-chat-view-panel]").forEach(function (panel) {
      var selected = panel.dataset.chatViewPanel === active;
      panel.setAttribute("aria-hidden", String(!selected));
      panel.inert = !selected;
    });
    restoreChatViewPosition(active);
    if (focusView) {
      requestAnimationFrame(function () {
        var panel = chatViewPanel(active);
        var target = panel && panel.querySelector("[data-chat-view-back], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [href]");
        if (target) target.focus({ preventScroll: true });
      });
    }
  }

  function resetChatViews() {
    cstate.viewStack = ["chat"];
    cstate.viewFocus = [];
    syncChatView(false);
  }

  function focusChatPanel(name) {
    requestAnimationFrame(function () {
      var panel = chatViewPanel(name);
      var target = panel && panel.querySelector("button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [href]");
      if (target) target.focus({ preventScroll: true });
    });
  }

  function restoreChatAuxiliaryFocus(name) {
    var target = cstate.auxiliaryFocus[name];
    delete cstate.auxiliaryFocus[name];
    if (target && target.isConnected) requestAnimationFrame(function () { target.focus({ preventScroll: true }); });
  }

  function openChatView(name, opener) {
    if (!compactChatUI() || !chatViewPanel(name)) return false;
    closeChatMore(false);
    var current = cstate.viewStack[cstate.viewStack.length - 1] || "chat";
    if (current === name) return true;
    saveChatViewPosition(current);
    cstate.viewStack.push(name);
    cstate.viewFocus.push(opener || document.activeElement);
    syncChatView(true);
    return true;
  }

  function closeChatView() {
    if (!compactChatUI() || cstate.viewStack.length < 2) return false;
    var current = cstate.viewStack.pop();
    var returnFocus = cstate.viewFocus.pop();
    saveChatViewPosition(current);
    var toggle = document.getElementById(current === "agents" ? "chat-agents-btn" : current === "work" ? "chat-tasks-btn" : current === "controls" ? "chat-controls-btn" : "");
    if (toggle) toggle.setAttribute("aria-expanded", "false");
    if (current === "controls") document.getElementById("chat-controls-sheet").hidden = true;
    if (current === "agents") {
      document.getElementById("chat-agents").classList.remove("open");
      document.getElementById("chat-agent-backdrop").hidden = true;
    }
    if (current === "work") {
      document.getElementById("chat-tasks").classList.remove("open");
      document.getElementById("chat-task-backdrop").hidden = true;
    }
    syncChatView(false);
    if (returnFocus && returnFocus.isConnected) requestAnimationFrame(function () { returnFocus.focus({ preventScroll: true }); });
    return true;
  }

  function closeTopChatAuxiliary() {
    if (compactChatUI()) return closeChatView();
    var controls = document.getElementById("chat-controls-sheet");
    var agents = document.getElementById("chat-agents");
    var work = document.getElementById("chat-tasks");
    if (controls && !controls.hidden) { closeChatControls(); return true; }
    if (agents && !agents.hidden && agents.classList.contains("open")) { closeChatAgents(); return true; }
    if (work && !work.hidden && work.classList.contains("open")) { closeChatTasks(); return true; }
    return false;
  }

  function chatOpen() {
    var ov = document.getElementById("chat-overlay");
    return ov && !ov.hidden;
  }

  function activeSessionID() {
    if (chatOpen()) return cstate.session;
    return terminalOpen() ? tstate.session : null;
  }

  function sessionOverlayOpen() { return chatOpen() || terminalOpen(); }

  function syncChatModalInert(detailOpen) {
    document.querySelectorAll("[data-chat-detail-inert-owned]").forEach(function (node) {
      node.inert = false;
      node.removeAttribute("data-chat-detail-inert-owned");
    });
    document.querySelectorAll("[data-chat-inert-owned]").forEach(function (node) {
      node.inert = false;
      node.removeAttribute("data-chat-inert-owned");
    });
    function ownInert(node, marker) {
      if (!(node instanceof HTMLElement) || node.inert) return;
      node.inert = true;
      node.setAttribute(marker, "");
    }
    if (detailOpen) {
      var detail = document.getElementById("detail");
      var path = detail;
      while (path && path !== document.body) {
        var parent = path.parentElement;
        if (!parent) break;
        Array.prototype.forEach.call(parent.children, function (sibling) {
          if (sibling !== path) ownInert(sibling, "data-chat-detail-inert-owned");
        });
        path = parent;
      }
      return;
    }
    if (chatOpen()) {
      Array.prototype.forEach.call(document.body.children, function (child) {
        if (child.id !== "chat-overlay") ownInert(child, "data-chat-inert-owned");
      });
    }
  }

  function openChat(sessionID, title) {
    var opener = chatOpen() ? cstate.opener : document.activeElement;
    closeTerminal(true);
    closeChat(true, true);
    cstate.opener = opener;
    cstate.session = sessionID;
    cstate.title = title || sessionID;
    cstate.snapshot = null;
    cstate.controls = null;
    cstate.inbox = [];
    cstate.lifecycle = null;
    cstate.navigation = null;
    cstate.deletePreview = null;
    cstate.lifecycleError = "";
    cstate.revertTarget = null;
    cstate.revertPreviewHTML = "";
    cstate.revertPreviewValid = false;
    cstate.restoreScroll = Object.prototype.hasOwnProperty.call(cstate.scrolls, sessionID) ? cstate.scrolls[sessionID] : null;
    var overlay = document.getElementById("chat-overlay");
    overlay.hidden = false;
    syncChatModalInert(false);
    syncChatViewport();
    resetChatViews();
    document.getElementById("chat-title").textContent = cstate.title;
    renderChatUsage(null);
    setChatHeaderState("Connecting", "connecting");
    var prompt = document.getElementById("chat-prompt");
    prompt.value = cstate.drafts[sessionID] || "";
    document.getElementById("chat-reference-picker").hidden = true;
    document.getElementById("chat-reference-btn").setAttribute("aria-expanded", "false");
    document.getElementById("chat-actions-sheet").hidden = true;
    document.getElementById("chat-actions-backdrop").hidden = true;
    document.getElementById("chat-actions-btn").setAttribute("aria-expanded", "false");
    renderChatDraftFiles();
    renderChatSkillChips();
    closeChatControls();
    resizeChatPrompt();
    document.getElementById("chat-transcript").innerHTML = '<p class="chat-empty">Loading conversation…</p>';
    renderChatInbox();
    renderChatNavigation();
    updateChatDeliveryControls();
    setChatStatus("Connecting…");
    setChatTasks(null, null);
    if (page === "board" && boardEl() && boardEl().dataset.change) {
      setChatTasks(boardEl(), boardEl().dataset.change);
    } else {
      resolveTerminalPanel(sessionID);
    }
    pollChat(true);
    loadChatUsage(true);
    loadChatLifecycle();
    requestAnimationFrame(function () { prompt.focus({ preventScroll: true }); });
  }

  function closeChat(suppressRefresh, suppressFocus) {
    if (!cstate.session && !chatOpen()) return;
    var prompt = document.getElementById("chat-prompt");
    if (prompt && cstate.session) cstate.drafts[cstate.session] = prompt.value;
    var transcript = document.getElementById("chat-transcript");
    if (transcript && cstate.session) cstate.scrolls[cstate.session] = transcript.scrollTop;
    clearTimeout(cstate.timer);
    if (cstate.request) cstate.request.abort();
    if (cstate.inboxRequest) cstate.inboxRequest.abort();
    if (cstate.navigationRequest) cstate.navigationRequest.abort();
    clearTimeout(cstate.usageTimer);
    if (cstate.usageRequest) cstate.usageRequest.abort();
    closeChatMore(false);
    cstate.timer = null;
    cstate.request = null;
    cstate.inboxRequest = null;
    cstate.navigationRequest = null;
    cstate.usageTimer = null;
    cstate.usageRequest = null;
    cstate.mutation = false;
    cstate.lifecycleAction = false;
    cstate.session = null;
    var send = document.getElementById("chat-send-btn");
    var interrupt = document.getElementById("chat-interrupt-btn");
    if (send) send.disabled = false;
    if (interrupt) interrupt.disabled = false;
    terminalPanelChange = null;
    var overlay = document.getElementById("chat-overlay");
    if (overlay) overlay.hidden = true;
    syncChatModalInert(false);
    resetChatViews();
    closeChatTasks(false);
    closeChatAgents(false);
    closeChatControls(false);
    cstate.auxiliaryFocus = {};
    if (!suppressFocus) {
      var opener = cstate.opener;
      cstate.opener = null;
      if (opener && opener.isConnected) requestAnimationFrame(function () { opener.focus({ preventScroll: true }); });
    }
    if (!suppressRefresh && page === "index") scheduleRefresh();
  }

  function syncChatViewport() {
    var win = document.querySelector(".chat-window");
    if (!win || !window.visualViewport || !chatOpen()) return;
    win.style.setProperty("--chat-viewport-height", window.visualViewport.height + "px");
  }

  function scheduleChatPoll(delay) {
    clearTimeout(cstate.timer);
    if (!chatOpen() || !cstate.session || document.hidden) return;
    cstate.timer = setTimeout(function () { pollChat(false); }, delay);
  }

  function pollChat(initial) {
    if (!chatOpen() || !cstate.session || document.hidden || cstate.request) return;
    var sessionID = cstate.session;
    var transcript = document.getElementById("chat-transcript");
    var controller = new AbortController();
    cstate.request = controller;
    transcript.setAttribute("aria-busy", "true");
    fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/chat", {
      headers: { Accept: "text/html" },
      signal: controller.signal,
    })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.text();
      })
      .then(function (html) {
        if (!chatOpen() || cstate.session !== sessionID) return;
        var nearBottom = transcript.scrollHeight - transcript.scrollTop - transcript.clientHeight < 96;
        var oldTop = transcript.scrollTop;
        var changed = cstate.snapshot !== html;
        if (changed) {
          var previousRoot = transcript.querySelector(".chat-snapshot");
          var historyPages = previousRoot ? Array.prototype.slice.call(previousRoot.querySelectorAll(":scope > .chat-history-page")) : [];
          var historyLoaded = previousRoot && previousRoot.dataset.historyLoaded === "true";
          var historyCursor = historyLoaded ? (previousRoot.dataset.historyCursor || "") : null;
          var openDetails = {};
          transcript.querySelectorAll("details[data-chat-expand-key][open]").forEach(function (detail) {
            var body = detail.hasAttribute("data-chat-detail-url") ? detail.querySelector(".chat-lazy-detail") : null;
            openDetails[detail.dataset.chatExpandKey] = body ? body.innerHTML : null;
          });
          transcript.innerHTML = html;
          var root = transcript.querySelector(".chat-snapshot");
          if (root && cstate.lifecycle) {
            var snapshotBusy = root.dataset.busy === "true";
           if (cstate.lifecycle.busy !== snapshotBusy) {
              cstate.lifecycle.busy = snapshotBusy;
              renderChatLifecycle();
               if (!snapshotBusy) loadChatLifecycle();
             }
           }
           updateChatDeliveryControls();
          var insertionPoint = root && root.querySelector("[data-chat-block], .chat-interaction");
          historyPages.forEach(function (historyPage) {
            if (root) root.insertBefore(historyPage, insertionPoint);
          });
          if (root && historyLoaded) {
            root.dataset.historyLoaded = "true";
            root.dataset.historyCursor = historyCursor;
            var load = root.querySelector("[data-chat-load-older]");
            if (historyCursor && load) load.dataset.cursor = historyCursor;
            else if (load) load.closest(".chat-history-control").remove();
          }
          Object.keys(openDetails).forEach(function (key) {
            var detail = Array.prototype.find.call(transcript.querySelectorAll("details[data-chat-expand-key]"), function (item) {
              return item.dataset.chatExpandKey === key;
            });
            if (!detail) return;
            detail.open = true;
            if (detail.hasAttribute("data-chat-detail-url")) {
              var body = detail.querySelector(".chat-lazy-detail");
              if (body && openDetails[key]) body.innerHTML = openDetails[key];
              loadChatDetail(detail, true);
            }
          });
          cstate.snapshot = html;
          if (window.htmx) htmx.process(transcript);
          var compactions = transcript.querySelectorAll('[data-message-type="compaction"]');
          var compact = compactions.length ? compactions[compactions.length - 1] : null;
          if (compact && (compact.dataset.messageStatus === "completed" || compact.dataset.messageStatus === "failed")) {
            cstate.compactPending[sessionID] = false;
            renderChatLifecycle();
          }
          if (initial && cstate.restoreScroll != null) {
            transcript.scrollTop = cstate.restoreScroll;
            cstate.restoreScroll = null;
          } else if (nearBottom || initial) transcript.scrollTop = transcript.scrollHeight;
          else transcript.scrollTop = oldTop;
          setChatStatus("Conversation updated");
        } else {
          setChatStatus("");
        }
      })
      .catch(function (err) {
        if (err.name !== "AbortError") {
          setChatStatus("Disconnected from OpenCode. Retrying…", true, true);
          setChatHeaderState("Disconnected", "error");
        }
      })
      .finally(function () {
        if (cstate.request === controller) {
          cstate.request = null;
          transcript.setAttribute("aria-busy", "false");
           scheduleChatPoll(1200);
           loadChatInbox(false);
        }
      });
  }

  function refreshChat() {
    if (cstate.request) cstate.request.abort();
    cstate.request = null;
    pollChat(true);
  }

  function loadOlderChat(control) {
    if (!cstate.session || cstate.request || !control.dataset.cursor) return;
    var sessionID = cstate.session;
    var transcript = document.getElementById("chat-transcript");
    var controller = new AbortController();
    var oldHeight = transcript.scrollHeight;
    var oldTop = transcript.scrollTop;
    cstate.request = controller;
    control.disabled = true;
    transcript.setAttribute("aria-busy", "true");
    setChatStatus("Loading older messages…");
    fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/chat?cursor=" + encodeURIComponent(control.dataset.cursor), {
      headers: { Accept: "text/html" },
      signal: controller.signal,
    })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.text();
      })
      .then(function (html) {
        if (!chatOpen() || cstate.session !== sessionID) return;
        var holder = document.createElement("template");
        holder.innerHTML = html.trim();
        var page = holder.content.querySelector(".chat-snapshot");
        var root = transcript.querySelector(".chat-snapshot");
        if (!page || !root) throw new Error("Invalid history response");
        var existing = {};
        root.querySelectorAll("[data-message]").forEach(function (message) { existing[message.dataset.message] = true; });
        var insertionPoint = root.querySelector("[data-chat-block], .chat-interaction");
        var historyPage = document.createElement("div");
        historyPage.className = "chat-history-page";
        page.querySelectorAll(":scope > [data-chat-block]").forEach(function (block) {
          var source = (block.dataset.chatSource || "").trim().split(/\s+/).filter(Boolean);
          if (source.length && source.every(function (id) { return existing[id]; })) return;
          block.querySelectorAll("[data-message]").forEach(function (message) {
            if (existing[message.dataset.message]) message.remove();
            else existing[message.dataset.message] = true;
          });
          historyPage.appendChild(block);
        });
        if (historyPage.children.length) root.insertBefore(historyPage, insertionPoint);
        var next = page.querySelector("[data-chat-load-older]");
        root.dataset.historyLoaded = "true";
        root.dataset.historyCursor = next ? next.dataset.cursor : "";
        if (next) control.dataset.cursor = next.dataset.cursor;
        else control.closest(".chat-history-control").remove();
        if (window.htmx) htmx.process(transcript);
        transcript.scrollTop = oldTop + transcript.scrollHeight - oldHeight;
        setChatStatus("");
      })
      .catch(function (err) {
        if (err.name !== "AbortError") setChatStatus("Could not load older messages. " + err.message, true);
      })
      .finally(function () {
        if (cstate.request === controller) {
          cstate.request = null;
          transcript.setAttribute("aria-busy", "false");
          if (document.contains(control)) control.disabled = false;
          scheduleChatPoll(1200);
        }
      });
  }

  function setChatStatus(message, error, serviceLink) {
    var el = document.getElementById("chat-status");
    if (!el) return;
    el.textContent = message || "";
    el.classList.toggle("error", !!error);
    if (serviceLink) {
      var link = document.createElement("a");
      link.href = "/settings/opencode";
      link.textContent = "View service status";
      link.className = "chat-service-link";
      el.appendChild(link);
    }
  }

  function chatBusy() {
    var root = document.querySelector("#chat-transcript .chat-snapshot");
    return root ? root.dataset.busy === "true" : !!(cstate.lifecycle && cstate.lifecycle.busy);
  }

  function setChatHeaderState(label, state) {
    var el = document.getElementById("chat-session-state");
    if (!el) return;
    el.textContent = label;
    el.dataset.state = state || "";
  }

  function updateChatDeliveryControls() {
    var controls = document.getElementById("chat-delivery-controls");
    if (!controls) return;
    var busy = chatBusy();
    if (cstate.lifecycle || document.querySelector("#chat-transcript .chat-snapshot")) {
      setChatHeaderState(busy ? "Working" : "Idle", busy ? "busy" : "idle");
    }
    var cap = cstate.lifecycle && cstate.lifecycle.capabilities || {};
    var files = !!cap.promptDeliveryFiles;
    var skills = !!cap.promptDeliverySkills;
    var available = !!cap.promptDelivery && !!cap.promptDeliveryID;
    controls.hidden = !busy;
    document.getElementById("chat-delivery-mode").disabled = !available;
    document.getElementById("chat-delivery-note").textContent = !available
      ? "Durable follow-ups are unavailable on this OpenCode service."
      : files && skills ? "Choose queue for the next turn or steer the current turn."
      : "Unsupported attachments, references, or skills remain in your draft until the session is idle.";
    var addFile = document.querySelector(".chat-add-file");
    if (addFile) addFile.classList.toggle("is-disabled", busy && !files);
    if (addFile) addFile.disabled = busy && !files;
    document.getElementById("chat-file-input").disabled = busy && !files;
    document.getElementById("chat-reference-btn").disabled = busy && !files;
    document.getElementById("chat-interrupt-btn").hidden = !busy;
    renderChatControlsAvailability();
  }

  function renderChatControlsAvailability() {
    var skills = !!(cstate.lifecycle && cstate.lifecycle.capabilities && cstate.lifecycle.capabilities.promptDeliverySkills);
    document.querySelectorAll("#chat-skill-list [data-attach-skill]").forEach(function (button) {
      button.disabled = chatBusy() && !skills;
    });
  }

  function inboxTypeLabel(item) {
    if (!item.known) return "Unsupported pending item (" + (item.type || "unknown") + ")";
    return ({ user: "Follow-up", synthetic: "Synthetic message", compaction: "Compaction", move: "Session move" })[item.type] || "Pending item";
  }

  function renderChatInbox() {
    var root = document.getElementById("chat-inbox");
    if (!root) return;
    root.innerHTML = "";
    root.hidden = !cstate.inbox.length;
    if (!cstate.inbox.length) return;
    var heading = document.createElement("strong");
    heading.textContent = "Pending in OpenCode (" + cstate.inbox.length + ")";
    root.appendChild(heading);
    var cap = cstate.lifecycle && cstate.lifecycle.capabilities || {};
    cstate.inbox.forEach(function (item) {
      var row = document.createElement("div");
      row.className = "chat-inbox-item";
      row.dataset.inboxID = item.id;
      var copy = document.createElement("div");
      copy.className = "chat-inbox-copy";
      var label = document.createElement("small");
      label.textContent = inboxTypeLabel(item);
      copy.appendChild(label);
      if (item.known && (item.text || item.description)) {
        var text = document.createElement("p");
        text.textContent = item.text || item.description;
        copy.appendChild(text);
      }
      row.appendChild(copy);
      var mode = document.createElement("select");
      mode.setAttribute("aria-label", "Delivery mode for " + inboxTypeLabel(item));
      ["queue", "steer"].forEach(function (value) {
        var option = document.createElement("option");
        option.value = value;
        option.textContent = value === "queue" ? "Queued" : "Steering";
        option.selected = item.delivery === value;
        mode.appendChild(option);
      });
      mode.disabled = !cap.inboxDelivery;
      mode.dataset.inboxDelivery = item.id;
      row.appendChild(mode);
      var cancel = document.createElement("button");
      cancel.type = "button";
      cancel.className = "btn-ghost";
      cancel.textContent = "Cancel";
      cancel.disabled = !cap.inboxCancel;
      cancel.dataset.inboxCancel = item.id;
      row.appendChild(cancel);
      root.appendChild(row);
    });
  }

  function loadChatInbox(reportError, propagateError) {
    if (!cstate.session || cstate.inboxRequest) return Promise.resolve();
    var cap = cstate.lifecycle && cstate.lifecycle.capabilities;
    if (!cap || !cap.inboxList) {
      cstate.inbox = [];
      renderChatInbox();
      return Promise.resolve();
    }
    var sessionID = cstate.session;
    var controller = new AbortController();
    cstate.inboxRequest = controller;
    return fetch(lifecycleURL("/inbox"), { headers: { Accept: "application/json" }, signal: controller.signal })
      .then(lifecycleResponse)
      .then(function (data) {
        if (cstate.session !== sessionID) return;
        cstate.inbox = data.items || [];
        renderChatInbox();
        return data;
      })
      .catch(function (err) {
        if (err.name !== "AbortError" && reportError && cstate.session === sessionID) setChatStatus("Pending follow-ups unavailable: " + err.message, true);
        if (propagateError && err.name !== "AbortError") throw err;
      })
      .finally(function () { if (cstate.inboxRequest === controller) cstate.inboxRequest = null; });
  }

  function mutateChatInbox(itemID, method, body, control) {
    if (!cstate.session || cstate.mutation) return Promise.reject(new Error("A request is already in progress"));
    var sessionID = cstate.session;
    cstate.mutation = true;
    control.disabled = true;
    var options = { method: method, headers: { Accept: "application/json" } };
    if (body) {
      options.headers["Content-Type"] = "application/json";
      options.body = JSON.stringify(body);
    }
    return fetch(lifecycleURL("/inbox/" + encodeURIComponent(itemID)), options)
      .then(lifecycleResponse)
      .then(function () {
        if (cstate.inboxRequest) {
          cstate.inboxRequest.abort();
          cstate.inboxRequest = null;
        }
        return loadChatInbox(true, true);
      })
      .then(function (data) {
        if (cstate.session !== sessionID) return;
        var pending = (data.items || []).some(function (item) { return item.id === itemID; });
        if (method === "DELETE") {
          setChatStatus(pending ? "Cancellation was requested, but the item is still pending." : "Item is no longer pending. It may have been delivered or cancelled.", pending);
        } else {
          setChatStatus(pending ? "Pending delivery mode refreshed." : "The item is no longer pending and may already have been delivered.");
        }
        refreshChat();
      })
      .catch(function (err) {
        if (cstate.session === sessionID) setChatStatus(err.message, true);
        throw err;
      })
      .finally(function () {
        if (cstate.session === sessionID) {
          cstate.mutation = false;
          if (document.contains(control)) control.disabled = false;
        }
      });
  }

  function loadChatDetail(detail, refresh) {
    if (!detail || !detail.open || !detail.dataset.chatDetailUrl || detail.dataset.loading === "true") return;
    if (!refresh && detail.dataset.loaded === "true") return;
    var body = detail.querySelector(".chat-lazy-detail");
    if (!body) return;
    detail.dataset.loading = "true";
    if (!refresh) body.textContent = "Loading…";
    fetch(detail.dataset.chatDetailUrl, { headers: { Accept: "text/html" } })
      .then(function (r) {
        if (!r.ok) return r.json().catch(function () { return {}; }).then(function (j) { throw new Error(j.error || "HTTP " + r.status); });
        return r.text();
      })
      .then(function (html) {
        if (!document.contains(detail) || !detail.open) return;
        body.innerHTML = html;
        detail.dataset.loaded = "true";
      })
      .catch(function (err) {
        if (document.contains(detail) && detail.open) body.textContent = "Could not load detail: " + err.message;
      })
      .finally(function () { detail.dataset.loading = "false"; });
  }

  document.addEventListener("toggle", function (e) {
    var detail = e.target.closest && e.target.closest("#chat-transcript details[data-chat-detail-url]");
    if (detail && detail.open) loadChatDetail(detail, false);
  }, true);

  function chatMutation(path, body, control) {
    if (!cstate.session || cstate.mutation) return Promise.reject(new Error("A request is already in progress"));
    var sessionID = cstate.session;
    cstate.mutation = true;
    if (control) control.disabled = true;
    setChatStatus("Sending…");
    var options = { method: "POST", headers: { Accept: "application/json" } };
    if (body instanceof FormData) {
      options.body = body;
    } else if (body !== undefined) {
      options.headers["Content-Type"] = "application/json";
      options.body = JSON.stringify(body);
    }
    return fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/chat/" + path, options)
      .then(function (r) {
        if (r.ok) return;
        return r.json().catch(function () { return {}; }).then(function (j) {
          throw new Error(j.error || "Request failed (" + r.status + ")");
        });
      })
      .then(function () {
        if (cstate.session === sessionID) { setChatStatus(""); refreshChat(); }
      })
      .catch(function (err) {
        if (cstate.session === sessionID) setChatStatus(err.message, true);
        throw err;
      })
      .finally(function () {
        if (cstate.session === sessionID) {
          cstate.mutation = false;
          if (control && document.contains(control)) control.disabled = false;
        }
      });
  }

  function newChatMessageID() {
    var value = window.crypto && window.crypto.randomUUID ? window.crypto.randomUUID() : Date.now().toString(36) + Math.random().toString(36).slice(2);
    return "msg_" + value.replace(/-/g, "");
  }

  function deliverBusyPrompt(body, control) {
    if (!cstate.session || cstate.mutation) return Promise.reject(new Error("A request is already in progress"));
    var sessionID = cstate.session;
    cstate.mutation = true;
    control.disabled = true;
    setChatStatus("Submitting durable follow-up…");
    return fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/deliver", {
      method: "POST",
      headers: { Accept: "application/json" },
      body: body,
    })
      .then(lifecycleResponse)
      .then(function () {
        if (cstate.session !== sessionID) return;
        delete cstate.deliveryIDs[sessionID];
        if (cstate.inboxRequest) {
          cstate.inboxRequest.abort();
          cstate.inboxRequest = null;
        }
        return loadChatInbox(true).then(function () { refreshChat(); });
      })
      .catch(function (err) {
        if (cstate.session === sessionID) setChatStatus(err.message, true);
        throw err;
      })
      .finally(function () {
        if (cstate.session === sessionID) {
          cstate.mutation = false;
          if (document.contains(control)) control.disabled = false;
        }
      });
  }

  function resizeChatPrompt() {
    var input = document.getElementById("chat-prompt");
    if (!input) return;
    input.style.height = "auto";
    input.style.height = Math.min(input.scrollHeight, 160) + "px";
  }

  function chatDraftFiles() {
    if (!cstate.session) return [];
    if (!cstate.files[cstate.session]) cstate.files[cstate.session] = [];
    return cstate.files[cstate.session];
  }

  function chatDraftReferences() {
    if (!cstate.session) return [];
    if (!cstate.references[cstate.session]) cstate.references[cstate.session] = [];
    return cstate.references[cstate.session];
  }

  function chatDraftSkills() {
    if (!cstate.session) return [];
    if (!cstate.skills[cstate.session]) cstate.skills[cstate.session] = [];
    return cstate.skills[cstate.session];
  }

  function revokeChatFile(file) {
    if (file.preview) URL.revokeObjectURL(file.preview);
  }

  function clearChatDraftFiles(sessionID) {
    (cstate.files[sessionID] || []).forEach(revokeChatFile);
    cstate.files[sessionID] = [];
    cstate.references[sessionID] = [];
  }

  function renderChatDraftFiles() {
    var tray = document.getElementById("chat-draft-files");
    if (!tray) return;
    tray.innerHTML = "";
    chatDraftFiles().forEach(function (item, index) {
      var chip = document.createElement("div");
      chip.className = "chat-draft-file";
      if (item.preview) {
        var img = document.createElement("img");
        img.src = item.preview;
        img.alt = "";
        chip.appendChild(img);
      }
      var name = document.createElement("span");
      name.textContent = item.file.name;
      chip.appendChild(name);
      var remove = document.createElement("button");
      remove.type = "button";
      remove.dataset.removeChatFile = String(index);
      remove.setAttribute("aria-label", "Remove " + item.file.name);
      remove.textContent = "×";
      chip.appendChild(remove);
      tray.appendChild(chip);
    });
    chatDraftReferences().forEach(function (ref) {
      var chip = document.createElement("div");
      chip.className = "chat-draft-file is-reference";
      var name = document.createElement("span");
      name.textContent = ref.name;
      chip.appendChild(name);
      var remove = document.createElement("button");
      remove.type = "button";
      remove.dataset.removeChatReference = ref.alias;
      remove.setAttribute("aria-label", "Remove reference " + ref.name);
      remove.textContent = "×";
      chip.appendChild(remove);
      tray.appendChild(chip);
    });
    tray.hidden = !tray.children.length;
  }

  function renderChatSkillChips() {
    var tray = document.getElementById("chat-skill-chips");
    if (!tray) return;
    tray.innerHTML = "";
    chatDraftSkills().forEach(function (skill) {
      var chip = document.createElement("div");
      chip.className = "chat-draft-file is-skill";
      var name = document.createElement("span");
      name.textContent = "Skill: " + (skill.name || skill.id);
      chip.appendChild(name);
      var remove = document.createElement("button");
      remove.type = "button";
      remove.dataset.removeChatSkill = skill.id;
      remove.setAttribute("aria-label", "Remove skill " + (skill.name || skill.id));
      remove.textContent = "×";
      chip.appendChild(remove);
      tray.appendChild(chip);
    });
    tray.hidden = !tray.children.length;
  }

  function closeChatControls(returnFocus) {
    var sheet = document.getElementById("chat-controls-sheet");
    var button = document.getElementById("chat-controls-btn");
    if (compactChatUI() && cstate.viewStack[cstate.viewStack.length - 1] === "controls") closeChatView();
    if (sheet) sheet.hidden = true;
    if (button) button.setAttribute("aria-expanded", "false");
    syncChatView(false);
    if (!compactChatUI() && returnFocus !== false) restoreChatAuxiliaryFocus("controls");
  }

  function requestChatAuxiliary(name) {
    if (compactChatUI()) return false;
    if (name !== "work") closeChatTasks(false);
    if (name !== "agents") closeChatAgents(false);
    if (name !== "controls") closeChatControls(false);
    return true;
  }

  function openChatControls(opener) {
    var sheet = document.getElementById("chat-controls-sheet");
    if (!compactChatUI()) requestChatAuxiliary("controls");
    if (!compactChatUI()) cstate.auxiliaryFocus.controls = opener || document.activeElement;
    sheet.hidden = false;
    document.getElementById("chat-controls-btn").setAttribute("aria-expanded", "true");
    if (compactChatUI()) openChatView("controls", opener);
    else { syncChatView(false); focusChatPanel("controls"); }
    return loadChatControls();
  }

  function optionLabel(item) { return item.name ? item.name + " · " + (item.value || item.id) : (item.value || item.id); }

  function fillChatSelect(select, items, current, placeholder) {
    select.innerHTML = "";
    var empty = document.createElement("option");
    empty.value = "";
    empty.textContent = placeholder;
    empty.disabled = true;
    select.appendChild(empty);
    items.forEach(function (item) {
      var option = document.createElement("option");
      option.value = item.value || item.id || item.name;
      option.textContent = optionLabel(item);
      option.dataset.search = ((item.name || "") + " " + (item.description || "") + " " + option.value).toLowerCase();
      option.selected = option.value === current;
      select.appendChild(option);
    });
    if (!current || !items.some(function (item) { return (item.value || item.id || item.name) === current; })) select.selectedIndex = 0;
  }

  function conciseTokens(value) {
    value = Number(value) || 0;
    if (value >= 1000000) return (value / 1000000).toFixed(value >= 10000000 ? 0 : 1).replace(/\.0$/, "") + "m";
    if (value >= 1000) return Math.round(value / 1000) + "k";
    return String(value);
  }

  function renderChatUsage(usage) {
    usage = usage || {};
    var header = document.getElementById("chat-context-usage");
    var available = !!usage.contextAvailable && !!usage.contextLimit;
    header.textContent = available ? "Context " + usage.percent + "%" : "Context unavailable";
    header.title = available ? conciseTokens(usage.estimatedContext) + " / " + conciseTokens(usage.contextLimit) : "Active context usage unavailable";
    header.classList.toggle("warning", available && !!usage.warning);

    var usageEl = document.getElementById("chat-usage");
    if (!usageEl) return;
    var usageText = "Session totals: " + conciseTokens(usage.input) + " input, " + conciseTokens(usage.output) + " output, " + conciseTokens(usage.reasoning) + " reasoning, " + conciseTokens(usage.cacheRead) + " cache read, " + conciseTokens(usage.cacheWrite) + " cache write.";
    if (available) usageText += " Active context: " + conciseTokens(usage.estimatedContext) + " / " + conciseTokens(usage.contextLimit) + " (" + usage.percent + "%).";
    else usageText += " Active context usage is unavailable.";
    usageEl.textContent = usageText;
    usageEl.classList.toggle("warning", available && !!usage.warning);
    if (available && usage.warning) usageEl.textContent += " Context is at least 80% full; compaction may happen soon.";
  }

  function loadChatUsage(immediate) {
    clearTimeout(cstate.usageTimer);
    if (!chatOpen() || !cstate.session) return;
    if (!immediate) {
      cstate.usageTimer = setTimeout(function () { loadChatUsage(true); }, 15000);
      return;
    }
    var sessionID = cstate.session;
    if (cstate.usageRequest) cstate.usageRequest.abort();
    cstate.usageRequest = new AbortController();
    fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/chat/usage", {
      headers: { Accept: "application/json" }, signal: cstate.usageRequest.signal
    }).then(function (r) {
      if (!r.ok) throw new Error("Context usage unavailable");
      return r.json();
    }).then(function (usage) {
      if (cstate.session !== sessionID) return;
      if (cstate.controls) cstate.controls.usage = usage;
      renderChatUsage(usage);
      renderChatLifecycle();
    }).catch(function (err) {
      if (err.name === "AbortError" || cstate.session !== sessionID) return;
      var usage = cstate.controls && cstate.controls.usage || {};
      usage.contextAvailable = false;
      if (cstate.controls) cstate.controls.usage = usage;
      renderChatUsage(usage);
      renderChatLifecycle();
    }).finally(function () {
      if (cstate.session === sessionID) {
        cstate.usageRequest = null;
        loadChatUsage(false);
      }
    });
  }

  function renderChatControls(data) {
    cstate.controls = data;
    fillChatSelect(document.getElementById("chat-agent-select"), data.agents || [], data.agent, "Choose agent");
    fillChatSelect(document.getElementById("chat-model-select"), data.models || [], data.model, "Choose model");
    fillChatSelect(document.getElementById("chat-command-select"), data.commands || [], "", "Choose command");
    renderChatUsage(data.usage);
    var list = document.getElementById("chat-skill-list");
    list.innerHTML = "";
    (data.skills || []).forEach(function (skill) {
      var row = document.createElement("div");
      row.className = "chat-skill-row";
      row.dataset.search = ((skill.name || "") + " " + skill.id + " " + (skill.description || "")).toLowerCase();
      var text = document.createElement("span");
      text.textContent = skill.name || skill.id;
      if (skill.description) text.title = skill.description;
      row.appendChild(text);
      var attach = document.createElement("button");
      attach.type = "button";
      attach.className = "btn-ghost";
      attach.dataset.attachSkill = skill.id;
      attach.textContent = "Attach";
      row.appendChild(attach);
      var activate = document.createElement("button");
      activate.type = "button";
      activate.className = "btn-ghost";
      activate.dataset.activateSkill = skill.id;
      activate.textContent = "Activate";
      activate.disabled = !data.standaloneSkill;
      row.appendChild(activate);
      list.appendChild(row);
    });
    document.getElementById("chat-controls-note").textContent = data.standaloneSkill ? "Attach adds a skill to your next prompt. Activate runs it immediately." : "Standalone activation is unavailable on this OpenCode service. Prompt attachment remains available.";
    renderChatLifecycle();
    renderChatManagement();
    renderChatControlsAvailability();
  }

  function lifecycleURL(path) {
    return "/api/sessions/" + encodeURIComponent(cstate.session) + (path === undefined ? "/lifecycle" : path);
  }

  function lifecycleResponse(r) {
    if (r.ok) return r.status === 204 ? null : r.json();
    return r.json().catch(function () { return {}; }).then(function (body) {
      var err = new Error(body.error || "Request failed (" + r.status + ")");
      err.status = r.status;
      throw err;
    });
  }

  function navigationMeta(item) {
    var parts = [];
    if (item.task) parts.push(item.task);
    parts.push(item.busy ? "working" : "idle");
    if (item.pendingInput) parts.push(item.pendingInput + " pending input" + (item.pendingInput === 1 ? "" : "s"));
    return parts.join(" · ");
  }

  function switchChatSession(item) {
    if (!item || !item.session || item.session === cstate.session) return;
    openChat(item.session, item.title || item.session);
  }

  function appendNavigationButton(parent, item, className) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = className || "btn-ghost";
    var label = document.createElement("span");
    label.textContent = item.title || item.session;
    button.appendChild(label);
    button.title = navigationMeta(item);
    button.addEventListener("click", function () { switchChatSession(item); });
    parent.appendChild(button);
    return button;
  }

  function owningSessionLink(item) {
    if (!item || !item.change) return null;
    var link = document.createElement("a");
    link.href = "/changes/" + encodeURIComponent(item.change) + (item.task ? "?task=" + encodeURIComponent(item.task) : "");
    link.textContent = item.task ? "Return to " + item.task : "Return to change";
    return link;
  }

  function renderChatNavigation() {
    var bar = document.getElementById("chat-family-bar");
    var drawer = document.getElementById("chat-agents");
    var toggle = document.getElementById("chat-agents-btn");
    if (!bar || !drawer || !toggle) return;
    bar.innerHTML = "";
    drawer.innerHTML = "";
    var data = cstate.navigation;
    if (!data || !data.current) {
      bar.hidden = true;
      drawer.hidden = true;
      toggle.hidden = true;
      closeChatAgents();
      return;
    }
    (data.ancestors || []).forEach(function (item) {
      appendNavigationButton(bar, item);
      bar.appendChild(document.createTextNode("/"));
    });
    var current = document.createElement("strong");
    current.textContent = data.current.title || data.current.session;
    bar.appendChild(current);
    var meta = document.createElement("span");
    meta.className = "chat-family-meta";
    meta.textContent = navigationMeta(data.current);
    bar.appendChild(meta);
    var owner = owningSessionLink(data.current);
    if (owner) bar.appendChild(owner);
    bar.hidden = false;

    var descendants = data.descendants || [];
    toggle.hidden = descendants.length === 0;
    drawer.hidden = descendants.length === 0;
    if (!descendants.length) {
      closeChatAgents();
      return;
    }
    var heading = document.createElement("div");
    heading.className = "chat-agents-head";
    var back = document.createElement("button");
    back.type = "button";
    back.className = "chat-view-back btn-ghost";
    back.dataset.chatViewBack = "";
    back.textContent = "Back to Chat";
    var strong = document.createElement("strong");
    strong.textContent = "Child activity";
    var close = document.createElement("button");
    close.type = "button";
    close.className = "modal-close";
    close.setAttribute("aria-label", "Close agent drawer");
    close.textContent = "✕";
    close.addEventListener("click", closeChatAgents);
    heading.appendChild(back);
    heading.appendChild(strong);
    heading.appendChild(close);
    drawer.appendChild(heading);
    descendants.forEach(function (item) {
      var row = appendNavigationButton(drawer, item, "chat-agent-row");
      row.style.marginLeft = Math.min(Math.max((item.depth || 1) - 1, 0) * 12, 48) + "px";
      var state = document.createElement("span");
      state.className = "chat-agent-state";
      state.textContent = item.pendingInput ? "Needs input" : (item.busy ? "Working" : "Idle");
      row.appendChild(state);
      var detail = document.createElement("small");
      detail.textContent = (item.task ? item.task + " · " : "") + (item.change ? item.change : "Unmapped");
      row.appendChild(detail);
    });
  }

  function loadChatNavigation() {
    if (!cstate.session) return Promise.resolve(null);
    if (cstate.navigationRequest) cstate.navigationRequest.abort();
    var sessionID = cstate.session;
    var controller = new AbortController();
    cstate.navigationRequest = controller;
    return fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/navigation", { headers: { Accept: "application/json" }, signal: controller.signal })
      .then(lifecycleResponse)
      .then(function (data) {
        if (cstate.session !== sessionID) return null;
        cstate.navigation = data;
        if (data.current && data.current.title) {
          cstate.title = data.current.title;
          document.getElementById("chat-title").textContent = cstate.title;
        }
        renderChatNavigation();
        return data;
      })
      .catch(function (err) {
        if (err.name !== "AbortError" && cstate.session === sessionID) setChatStatus("Agent activity unavailable: " + err.message, true);
        return null;
      })
      .finally(function () { if (cstate.navigationRequest === controller) cstate.navigationRequest = null; });
  }

  function closeChatAgents(returnFocus) {
    var drawer = document.getElementById("chat-agents");
    var backdrop = document.getElementById("chat-agent-backdrop");
    var button = document.getElementById("chat-agents-btn");
    if (compactChatUI() && cstate.viewStack[cstate.viewStack.length - 1] === "agents") closeChatView();
    if (drawer) drawer.classList.remove("open");
    if (backdrop) backdrop.hidden = true;
    if (button) button.setAttribute("aria-expanded", "false");
    syncChatView(false);
    if (!compactChatUI() && returnFocus !== false) restoreChatAuxiliaryFocus("agents");
  }

  function openChatAgents(opener) {
    var drawer = document.getElementById("chat-agents");
    var button = document.getElementById("chat-agents-btn");
    if (compactChatUI()) {
      button.setAttribute("aria-expanded", "true");
      openChatView("agents", opener);
      return;
    }
    requestChatAuxiliary("agents");
    cstate.auxiliaryFocus.agents = opener || document.activeElement;
    drawer.classList.add("open");
    button.setAttribute("aria-expanded", "true");
    document.getElementById("chat-agent-backdrop").hidden = false;
    syncChatView(false);
    focusChatPanel("agents");
  }

  function loadChatLifecycle() {
    if (!cstate.session) return Promise.resolve(null);
    var sessionID = cstate.session;
    return fetch(lifecycleURL(), { headers: { Accept: "application/json" } })
      .then(lifecycleResponse)
      .then(function (data) {
        if (cstate.session !== sessionID) return null;
        cstate.lifecycle = data;
        cstate.lifecycleError = "";
        renderChatLifecycle();
        renderChatManagement();
        updateChatDeliveryControls();
        loadChatInbox(true);
        loadChatNavigation();
        return data;
      })
      .catch(function (err) {
        if (cstate.session === sessionID) {
          cstate.lifecycleError = err.message;
          renderChatLifecycle();
          renderChatManagement();
        }
        return null;
      });
  }

  function lifecycleFingerprint(data) {
    var session = data && data.session;
    var revert = session && session.revert;
    return revert ? session.id + ":" + revert.messageID + ":" + session.updated : "";
  }

  function appendRevertFiles(parent, files) {
    var list = document.createElement("div");
    list.className = "chat-revert-files";
    if (!files || !files.length) {
      list.textContent = "No file restoration is reported for this staged revert.";
    } else {
      files.forEach(function (file) {
        var detail = document.createElement("details");
        detail.className = "chat-revert-file";
        var summary = document.createElement("summary");
        var name = document.createElement("code");
        name.textContent = file.file || "unnamed file";
        var stats = document.createElement("span");
        stats.textContent = "+" + (file.additions || 0) + " / -" + (file.deletions || 0);
        summary.appendChild(name);
        summary.appendChild(stats);
        detail.appendChild(summary);
        if (file.patch) {
          var patch = document.createElement("pre");
          patch.textContent = file.patch;
          detail.appendChild(patch);
        }
        list.appendChild(detail);
      });
    }
    parent.appendChild(list);
  }

  function renderChatLifecycle() {
    var root = document.getElementById("chat-lifecycle");
    if (!root) return;
    var state = document.getElementById("chat-lifecycle-state");
    var panel = document.getElementById("chat-revert-panel");
    var compact = document.getElementById("chat-compact-btn");
    var compactNote = document.getElementById("chat-compact-note");
    panel.innerHTML = "";
    if (cstate.lifecycleError) state.textContent = "Session state unavailable: " + cstate.lifecycleError;
    else if (!cstate.lifecycle) state.textContent = "Loading session state...";
    else state.textContent = cstate.lifecycle.busy ? "OpenCode is working. History changes are paused." : "Session is idle.";

    var data = cstate.lifecycle;
    var cap = data && data.capabilities || {};
    var session = data && data.session;
    var staged = session && session.revert;
    if (staged) {
      var box = document.createElement("div");
      box.className = "chat-revert-staged";
      var heading = document.createElement("strong");
      heading.textContent = "Staged revert";
      box.appendChild(heading);
      var range = document.createElement("p");
      range.textContent = "Transcript range: " + staged.messageID + " through the current end of the conversation.";
      box.appendChild(range);
      appendRevertFiles(box, staged.files || []);
      var fingerprint = lifecycleFingerprint(data);
      var label = document.createElement("label");
      label.className = "chat-revert-confirm";
      label.textContent = cstate.revertPreviewValid ? "Type the fingerprint to confirm" : "Refresh this preview before committing";
      var code = document.createElement("code");
      code.textContent = fingerprint;
      label.appendChild(code);
      var input = document.createElement("input");
      input.id = "chat-revert-fingerprint";
      input.type = "text";
      input.autocomplete = "off";
      input.spellcheck = false;
      label.appendChild(input);
      box.appendChild(label);
      var actions = document.createElement("div");
      actions.className = "chat-lifecycle-actions";
      var refresh = document.createElement("button");
      refresh.type = "button";
      refresh.dataset.chatRevertRefresh = staged.messageID;
      refresh.textContent = "Refresh preview";
      refresh.hidden = cstate.revertPreviewValid;
      var commit = document.createElement("button");
      commit.type = "button";
      commit.dataset.chatRevertCommit = "1";
      commit.textContent = "Commit revert";
      commit.disabled = !cstate.revertPreviewValid || !!(data && data.busy) || cstate.lifecycleAction;
      var cancel = document.createElement("button");
      cancel.type = "button";
      cancel.className = "btn-ghost";
      cancel.dataset.chatRevertCancel = "1";
      cancel.textContent = "Cancel staged revert";
      cancel.disabled = cstate.lifecycleAction;
      actions.appendChild(refresh);
      actions.appendChild(commit);
      actions.appendChild(cancel);
      box.appendChild(actions);
      input.addEventListener("input", function () {
        commit.disabled = input.value !== fingerprint || !!data.busy || cstate.lifecycleAction;
        cancel.disabled = input.value !== fingerprint || cstate.lifecycleAction;
      });
      cancel.disabled = true;
      panel.appendChild(box);
    } else if (cstate.revertTarget) {
      var preview = document.createElement("div");
      preview.className = "chat-revert-preview";
      var title = document.createElement("strong");
      title.textContent = "Read-only revert preview";
      preview.appendChild(title);
      var transcript = document.getElementById("chat-transcript");
      var messages = Array.prototype.slice.call(transcript.querySelectorAll("[data-message]"));
      var index = messages.findIndex(function (message) { return message.dataset.message === cstate.revertTarget; });
      var affected = index < 0 ? [] : messages.slice(index);
      var rangeText = document.createElement("p");
      rangeText.textContent = "Transcript range: " + cstate.revertTarget + " through " + (affected.length ? affected[affected.length - 1].dataset.message : "the current end") + " (" + affected.length + " loaded messages).";
      preview.appendChild(rangeText);
      var diff = document.createElement("div");
      diff.innerHTML = cstate.revertPreviewHTML || '<p class="muted">Loading affected files...</p>';
      preview.appendChild(diff);
      var files = document.createElement("label");
      var check = document.createElement("input");
      check.id = "chat-revert-files";
      check.type = "checkbox";
      files.appendChild(check);
      files.appendChild(document.createTextNode(" Restore the displayed file changes as well as conversation history"));
      preview.appendChild(files);
      var stage = document.createElement("button");
      stage.type = "button";
      stage.dataset.chatRevertStage = cstate.revertTarget;
      stage.textContent = "Stage this revert";
      stage.disabled = !cstate.revertPreviewHTML || !data || data.busy || !cap.revertStage || cstate.lifecycleAction;
      preview.appendChild(stage);
      panel.appendChild(preview);
    } else {
      var hint = document.createElement("p");
      hint.className = "muted";
      hint.textContent = cap.revertStage === false ? "Revert is unavailable on this OpenCode service." : "Choose Preview revert on a transcript message to inspect the affected range and files.";
      panel.appendChild(hint);
    }

    var usage = cstate.controls && cstate.controls.usage || {};
    var pending = !!(cstate.session && cstate.compactPending[cstate.session]);
    var canCompact = !!(data && cap.compact && !data.busy && usage.contextAvailable && usage.contextLimit && !pending && !cstate.lifecycleAction);
    compact.disabled = !canCompact;
    compact.textContent = pending ? "Compaction in progress..." : "Compact context";
    if (pending) compactNote.textContent = "Progress and any failure appear asynchronously in the transcript.";
    else if (!cap.compact && data) compactNote.textContent = "Manual compaction is unavailable on this OpenCode service.";
    else if (data && data.busy) compactNote.textContent = "Wait for the active response to finish before compacting.";
    else if (!usage.contextAvailable || !usage.contextLimit) compactNote.textContent = "Active context usage is required before manual compaction.";
    else compactNote.textContent = "Active context: " + conciseTokens(usage.estimatedContext) + " / " + conciseTokens(usage.contextLimit) + " (" + (usage.percent || 0) + "%). Progress appears in the transcript.";
  }

  function renderChatManagement() {
    var data = cstate.lifecycle;
    var cap = data && data.capabilities || {};
    var session = data && data.session;
    var title = document.getElementById("chat-rename-title");
    var rename = document.getElementById("chat-rename-btn");
    var exportButton = document.getElementById("chat-export-btn");
    var exportNote = document.getElementById("chat-export-note");
    var unlink = document.getElementById("chat-unlink-btn");
    var deletion = document.getElementById("chat-delete-preview-btn");
    if (!title || !rename) return;
    if (session && document.activeElement !== title) title.value = session.title || "";
    title.disabled = !session || !cap.rename;
    rename.disabled = !session || !cap.rename || cstate.lifecycleAction;
    exportButton.disabled = !session || !cap.export || cstate.lifecycleAction;
    exportNote.textContent = data && !cap.export ? "Sanitized export is unavailable on this OpenCode version." : "Export always requests OpenCode sanitization.";
    unlink.disabled = !session || !data.mapping || cstate.lifecycleAction;
    deletion.disabled = !session || !cap.delete || cstate.lifecycleAction;
    deletion.textContent = cap.delete || !data ? "Review OpenCode deletion" : "OpenCode deletion unavailable";
    renderChatDeletePreview();
  }

  function renderChatDeletePreview() {
    var root = document.getElementById("chat-delete-preview");
    if (!root) return;
    root.innerHTML = "";
    var preview = cstate.deletePreview;
    root.hidden = !preview;
    if (!preview) return;
    var warning = document.createElement("p");
    warning.textContent = "Delete this OpenCode session and " + (preview.count - 1) + " descendant(s). All " + preview.mappings.length + " affected lessmess mapping(s) will also be removed.";
    root.appendChild(warning);
    var fingerprint = document.createElement("p");
    fingerprint.appendChild(document.createTextNode("Current tree fingerprint: "));
    var code = document.createElement("code");
    code.textContent = preview.fingerprint;
    fingerprint.appendChild(code);
    root.appendChild(fingerprint);
    var list = document.createElement("ul");
    (preview.sessions || []).forEach(function (item) {
      var row = document.createElement("li");
      row.textContent = (item.depth ? "Child: " : "") + (item.title || item.sessionID) + " (" + item.sessionID + ")";
      list.appendChild(row);
    });
    (preview.mappings || []).forEach(function (item) {
      var row = document.createElement("li");
      row.textContent = "Mapping: " + item.session + " -> " + (item.change || "Unassigned") + (item.task ? " / " + item.task : "");
      list.appendChild(row);
    });
    root.appendChild(list);
    var label = document.createElement("label");
    label.textContent = "Type the current title exactly to confirm";
    var input = document.createElement("input");
    input.id = "chat-delete-title";
    input.type = "text";
    input.autocomplete = "off";
    label.appendChild(input);
    root.appendChild(label);
    var confirm = document.createElement("button");
    confirm.type = "button";
    confirm.className = "chat-delete-confirm";
    confirm.dataset.chatDeleteConfirm = "1";
    confirm.textContent = "Delete OpenCode tree and mappings";
    confirm.disabled = true;
    input.addEventListener("input", function () { confirm.disabled = input.value !== preview.title || cstate.lifecycleAction; });
    root.appendChild(confirm);
  }

  function sessionManagementRequest(path, method, body) {
    var options = { method: method, headers: { Accept: "application/json" } };
    if (body !== undefined) {
      options.headers["Content-Type"] = "application/json";
      options.body = JSON.stringify(body);
    }
    return fetch(lifecycleURL(path), options).then(lifecycleResponse);
  }

  function safeSessionExportFilename(sessionID) {
    return "session-" + sessionID.replace(/[^A-Za-z0-9_-]/g, "_").slice(0, 128) + ".json";
  }

  function previewChatRevert(messageID) {
    if (!cstate.session || cstate.lifecycleAction) return;
    var sessionID = cstate.session;
    cstate.lifecycleAction = true;
    cstate.revertTarget = messageID;
    cstate.revertPreviewHTML = "";
    cstate.revertPreviewValid = false;
    openChatControls();
    renderChatLifecycle();
    Promise.all([
      loadChatLifecycle(),
      fetch(lifecycleURL("/chat/diff?from=" + encodeURIComponent(messageID)), { headers: { Accept: "text/html" } }).then(function (r) {
        if (!r.ok) throw new Error("Affected files unavailable (HTTP " + r.status + ")");
        return r.text();
      })
    ]).then(function (values) {
      if (cstate.session !== sessionID) return;
      cstate.revertPreviewHTML = values[1];
      cstate.revertPreviewValid = true;
    }).catch(function (err) {
      if (cstate.session === sessionID) cstate.lifecycleError = err.message;
    }).finally(function () {
      if (cstate.session === sessionID) {
        cstate.lifecycleAction = false;
        renderChatLifecycle();
      }
    });
  }

  function runLifecycleAction(path, body, onSuccess) {
    if (!cstate.session || cstate.lifecycleAction) return Promise.reject(new Error("A history action is already in progress"));
    var sessionID = cstate.session;
    cstate.lifecycleAction = true;
    renderChatLifecycle();
    return fetch(lifecycleURL(path), {
      method: "POST",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify(body || {})
    }).then(lifecycleResponse).then(function (result) {
      if (cstate.session === sessionID && onSuccess) onSuccess(result);
      return result;
    }).catch(function (err) {
      if (cstate.session === sessionID) {
        cstate.lifecycleError = err.message;
        if (err.status === 409) {
          cstate.revertPreviewValid = false;
          cstate.revertPreviewHTML = "";
        }
      }
      throw err;
    }).finally(function () {
      if (cstate.session === sessionID) {
        cstate.lifecycleAction = false;
        loadChatLifecycle();
        renderChatLifecycle();
      }
    });
  }

  function loadChatControls() {
    if (!cstate.session) return Promise.resolve();
    var sessionID = cstate.session;
    document.getElementById("chat-controls-note").textContent = "Loading controls…";
    return fetch("/api/sessions/" + encodeURIComponent(sessionID) + "/chat/controls", { headers: { Accept: "application/json" } })
      .then(function (r) { if (!r.ok) return r.json().catch(function () { return {}; }).then(function (j) { throw new Error(j.error || "Controls unavailable"); }); return r.json(); })
      .then(function (data) { if (cstate.session === sessionID) renderChatControls(data); })
      .catch(function (err) { if (cstate.session === sessionID) document.getElementById("chat-controls-note").textContent = err.message; });
  }

  function inferredChatMIME(file) {
    if (file.type) return file.type.toLowerCase();
    var ext = file.name.toLowerCase().split(".").pop();
    return ({ txt: "text/plain", md: "text/markdown", json: "application/json", xml: "application/xml", js: "application/javascript", css: "text/css", html: "text/html", yaml: "application/yaml", yml: "application/yaml", toml: "application/toml", svg: "image/svg+xml", png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", gif: "image/gif", webp: "image/webp" })[ext] || "text/plain";
  }

  function addChatFiles(list) {
    if (cstate.session) delete cstate.deliveryIDs[cstate.session];
    var files = chatDraftFiles();
    var refs = chatDraftReferences();
    var total = files.reduce(function (sum, item) { return sum + item.file.size; }, 0);
    Array.prototype.forEach.call(list, function (file) {
      var mime = inferredChatMIME(file);
      if (files.length + refs.length >= 10) { setChatStatus("At most 10 files and references are allowed.", true); return; }
      if (file.size > 20 * 1024 * 1024 || total + file.size > 20 * 1024 * 1024) { setChatStatus("Attachments may be at most 20 MiB each and total.", true); return; }
      if (!mime) { setChatStatus("Unsupported file type: " + file.name, true); return; }
      var preview = /^(image\/(png|jpeg|gif|webp))$/.test(mime) ? URL.createObjectURL(file) : "";
      files.push({ file: file, mime: mime, preview: preview });
      total += file.size;
    });
    renderChatDraftFiles();
  }

  function loadChatReferences() {
    var list = document.getElementById("chat-reference-list");
    list.textContent = "Loading…";
    return fetch("/api/sessions/" + encodeURIComponent(cstate.session) + "/chat/references", { headers: { Accept: "application/json" } })
      .then(function (r) { if (!r.ok) throw new Error("HTTP " + r.status); return r.json(); })
      .then(function (data) { cstate.referenceCatalog = data.references || []; renderChatReferenceList(); })
      .catch(function () { list.textContent = "Could not load project references."; });
  }

  function renderChatReferenceList() {
    var list = document.getElementById("chat-reference-list");
    var query = document.getElementById("chat-reference-search").value.toLowerCase();
    var selected = {};
    chatDraftReferences().forEach(function (ref) { selected[ref.alias] = true; });
    list.innerHTML = "";
    cstate.referenceCatalog.forEach(function (ref) {
      if (query && (ref.name + " " + (ref.description || "")).toLowerCase().indexOf(query) < 0) return;
      var label = document.createElement("label");
      var check = document.createElement("input");
      check.type = "checkbox";
      check.dataset.chatReference = ref.alias;
      check.checked = !!selected[ref.alias];
      label.appendChild(check);
      var text = document.createElement("span");
      text.textContent = ref.name;
      label.appendChild(text);
      list.appendChild(label);
    });
    if (!list.children.length) list.textContent = "No matching references.";
  }

  function closeChatTasks(returnFocus) {
    var panel = document.getElementById("chat-tasks");
    var toggle = document.getElementById("chat-tasks-btn");
    var backdrop = document.getElementById("chat-task-backdrop");
    if (compactChatUI() && cstate.viewStack[cstate.viewStack.length - 1] === "work") closeChatView();
    if (panel) panel.classList.remove("open");
    if (toggle) toggle.setAttribute("aria-expanded", "false");
    if (backdrop) backdrop.hidden = true;
    syncChatView(false);
    if (!compactChatUI() && returnFocus !== false) restoreChatAuxiliaryFocus("work");
  }

  function openChatTasks(opener) {
    var panel = document.getElementById("chat-tasks");
    var button = document.getElementById("chat-tasks-btn");
    if (compactChatUI()) {
      button.setAttribute("aria-expanded", "true");
      openChatView("work", opener);
      return;
    }
    requestChatAuxiliary("work");
    cstate.auxiliaryFocus.work = opener || document.activeElement;
    panel.classList.add("open");
    button.setAttribute("aria-expanded", "true");
    document.getElementById("chat-task-backdrop").hidden = false;
    syncChatView(false);
    focusChatPanel("work");
  }

  function setChatTasks(src, change) {
    var panel = document.getElementById("chat-tasks");
    var toggle = document.getElementById("chat-tasks-btn");
    if (!panel || !toggle) return;
    if (!change) closeChatTasks();
    panel.innerHTML = "";
    panel.hidden = !change;
    toggle.hidden = !change;
    if (change) renderTaskPanel(panel, src, change);
  }

  function closeChatMore(returnFocus) {
    var button = document.getElementById("chat-more-btn");
    var menu = document.getElementById("chat-more-menu");
    var backdrop = document.getElementById("chat-more-backdrop");
    if (!button || !menu || menu.hidden) return false;
    menu.hidden = true;
    backdrop.hidden = true;
    button.setAttribute("aria-expanded", "false");
    if (returnFocus !== false && button.isConnected) button.focus({ preventScroll: true });
    return true;
  }

  function openChatMore() {
    var button = document.getElementById("chat-more-btn");
    var menu = document.getElementById("chat-more-menu");
    var backdrop = document.getElementById("chat-more-backdrop");
    menu.hidden = false;
    backdrop.hidden = false;
    button.setAttribute("aria-expanded", "true");
    var first = menu.querySelector('[role="menuitem"]:not([disabled])');
    if (first) first.focus({ preventScroll: true });
  }

  (function initChat() {
    var composer = document.getElementById("chat-composer");
    if (!composer) return;
    var prompt = document.getElementById("chat-prompt");
    var send = document.getElementById("chat-send-btn");
    var fileInput = document.getElementById("chat-file-input");
    var actionsButton = document.getElementById("chat-actions-btn");
    var actionsSheet = document.getElementById("chat-actions-sheet");
    var actionsBackdrop = document.getElementById("chat-actions-backdrop");
    var moreButton = document.getElementById("chat-more-btn");
    var moreMenu = document.getElementById("chat-more-menu");
    var moreBackdrop = document.getElementById("chat-more-backdrop");
    var referencePicker = document.getElementById("chat-reference-picker");
    var referenceButton = document.getElementById("chat-reference-btn");

    function closeChatReferences(returnFocus) {
      if (referencePicker.hidden) return false;
      referencePicker.hidden = true;
      referenceButton.setAttribute("aria-expanded", "false");
      if (returnFocus !== false) actionsButton.focus({ preventScroll: true });
      return true;
    }

    function closeChatActions(returnFocus) {
      if (actionsSheet.hidden) return false;
      actionsSheet.hidden = true;
      actionsBackdrop.hidden = true;
      actionsButton.setAttribute("aria-expanded", "false");
      if (returnFocus !== false) actionsButton.focus({ preventScroll: true });
      return true;
    }

    function openChatActions() {
      closeChatMore(false);
      closeChatReferences(false);
      actionsSheet.hidden = false;
      actionsBackdrop.hidden = false;
      actionsButton.setAttribute("aria-expanded", "true");
      var first = actionsSheet.querySelector("button:not([disabled])");
      if (first) first.focus({ preventScroll: true });
    }

    actionsButton.addEventListener("click", function () {
      if (!closeChatActions(true)) openChatActions();
    });
    actionsBackdrop.addEventListener("click", function () { closeChatActions(true); });
    document.getElementById("chat-reference-close").addEventListener("click", function () { closeChatReferences(true); });
    moreButton.addEventListener("click", function () {
      if (!closeChatMore(true)) {
        closeChatActions(false);
        closeChatReferences(false);
        openChatMore();
      }
    });
    moreBackdrop.addEventListener("click", function () { closeChatMore(true); });
    moreMenu.addEventListener("click", function (e) {
      var item = e.target.closest("[data-chat-more-target]");
      if (!item) return;
      var target = document.getElementById(item.dataset.chatMoreTarget);
      closeChatMore(false);
      if (target && target.id === "chat-controls-btn") openChatControls(moreButton);
      else if (target) target.click();
    });
    document.querySelector(".chat-add-file").addEventListener("click", function () {
      closeChatActions(true);
      fileInput.click();
    });
    prompt.addEventListener("input", function () {
      if (cstate.session) {
        cstate.drafts[cstate.session] = prompt.value;
        delete cstate.deliveryIDs[cstate.session];
      }
      resizeChatPrompt();
    });
    prompt.addEventListener("keydown", function (e) {
      if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
        e.preventDefault();
        composer.requestSubmit();
      }
    });
    composer.addEventListener("submit", function (e) {
      e.preventDefault();
      var text = prompt.value.trim();
      var files = chatDraftFiles(), refs = chatDraftReferences(), skills = chatDraftSkills();
      if ((!text && !files.length && !refs.length && !skills.length) || cstate.mutation) return;
      var sessionID = cstate.session;
      var busy = chatBusy();
      var cap = cstate.lifecycle && cstate.lifecycle.capabilities || {};
      if (busy && (!cap.promptDelivery || !cap.promptDeliveryID)) {
        setChatStatus("Durable follow-ups are unavailable on this OpenCode service.", true);
        return;
      }
      if (busy && !cap.promptDeliveryFiles && (files.length || refs.length)) {
        setChatStatus("This service cannot deliver attachments or references while busy. Remove them or wait until the session is idle.", true);
        return;
      }
      if (busy && !cap.promptDeliverySkills && skills.length) {
        setChatStatus("This service cannot deliver skills while busy. Remove them or wait until the session is idle.", true);
        return;
      }
      var body = new FormData();
      body.append("text", text);
      files.forEach(function (item) {
        var upload = item.file.type === item.mime ? item.file : new File([item.file], item.file.name, { type: item.mime });
        body.append("files", upload, item.file.name);
      });
      refs.forEach(function (ref) { body.append("references", ref.alias); });
      skills.forEach(function (skill) { body.append("skills", skill.id); });
      var request;
      if (busy) {
        if (!cstate.deliveryIDs[sessionID]) cstate.deliveryIDs[sessionID] = newChatMessageID();
        body.append("id", cstate.deliveryIDs[sessionID]);
        body.append("delivery", document.getElementById("chat-delivery-mode").value);
        request = deliverBusyPrompt(body, send);
      } else {
        request = chatMutation("prompt", body, send);
      }
      request.then(function () {
        cstate.drafts[sessionID] = "";
        clearChatDraftFiles(sessionID);
        cstate.skills[sessionID] = [];
        if (cstate.session === sessionID) {
          prompt.value = "";
          fileInput.value = "";
          renderChatDraftFiles();
          renderChatSkillChips();
          resizeChatPrompt();
        }
      }).catch(function () {});
    });
    fileInput.addEventListener("change", function () { addChatFiles(fileInput.files); fileInput.value = ""; });
    document.getElementById("chat-draft-files").addEventListener("click", function (e) {
      var fileButton = e.target.closest("[data-remove-chat-file]");
      if (fileButton) {
        delete cstate.deliveryIDs[cstate.session];
        var removed = chatDraftFiles().splice(Number(fileButton.dataset.removeChatFile), 1)[0];
        if (removed) revokeChatFile(removed);
        renderChatDraftFiles();
      }
      var refButton = e.target.closest("[data-remove-chat-reference]");
      if (refButton) {
        delete cstate.deliveryIDs[cstate.session];
        cstate.references[cstate.session] = chatDraftReferences().filter(function (ref) { return ref.alias !== refButton.dataset.removeChatReference; });
        renderChatDraftFiles();
        renderChatReferenceList();
      }
    });
    document.getElementById("chat-skill-chips").addEventListener("click", function (e) {
      var button = e.target.closest("[data-remove-chat-skill]");
      if (!button) return;
      delete cstate.deliveryIDs[cstate.session];
      cstate.skills[cstate.session] = chatDraftSkills().filter(function (skill) { return skill.id !== button.dataset.removeChatSkill; });
      renderChatSkillChips();
    });
    document.getElementById("chat-inbox").addEventListener("change", function (e) {
      var select = e.target.closest("[data-inbox-delivery]");
      if (!select) return;
      mutateChatInbox(select.dataset.inboxDelivery, "PATCH", { delivery: select.value }, select).catch(function () { loadChatInbox(false); });
    });
    document.getElementById("chat-inbox").addEventListener("click", function (e) {
      var button = e.target.closest("[data-inbox-cancel]");
      if (!button) return;
      mutateChatInbox(button.dataset.inboxCancel, "DELETE", null, button).catch(function () { loadChatInbox(false); });
    });
    document.getElementById("chat-reference-btn").addEventListener("click", function (e) {
      closeChatActions(false);
      referencePicker.hidden = !referencePicker.hidden;
      e.currentTarget.setAttribute("aria-expanded", String(!referencePicker.hidden));
      if (!referencePicker.hidden) loadChatReferences().then(function () { document.getElementById("chat-reference-search").focus({ preventScroll: true }); });
      else actionsButton.focus({ preventScroll: true });
    });
    document.getElementById("chat-skill-btn").addEventListener("click", function () {
      closeChatActions(false);
      openChatControls(actionsButton).then(function () {
        var skill = document.querySelector("#chat-skill-list [data-attach-skill]");
        (skill || document.getElementById("chat-controls-search")).focus({ preventScroll: true });
      });
    });
    document.getElementById("chat-reference-search").addEventListener("input", renderChatReferenceList);
    document.getElementById("chat-reference-list").addEventListener("change", function (e) {
      var input = e.target.closest("[data-chat-reference]");
      if (!input) return;
      var ref = cstate.referenceCatalog.find(function (item) { return item.alias === input.dataset.chatReference; });
      var selected = chatDraftReferences();
      if (input.checked && ref && !selected.some(function (item) { return item.alias === ref.alias; })) {
        if (selected.length + chatDraftFiles().length >= 10) { input.checked = false; setChatStatus("At most 10 files and references are allowed.", true); return; }
        selected.push(ref);
      } else if (!input.checked) {
        cstate.references[cstate.session] = selected.filter(function (item) { return item.alias !== input.dataset.chatReference; });
      }
      delete cstate.deliveryIDs[cstate.session];
      renderChatDraftFiles();
    });
    document.getElementById("chat-interrupt-btn").addEventListener("click", function (e) {
      chatMutation("interrupt", undefined, e.currentTarget).catch(function () {});
    });
    document.getElementById("chat-controls-btn").addEventListener("click", function (e) {
      var sheet = document.getElementById("chat-controls-sheet");
      if (sheet.hidden || compactChatUI()) openChatControls(e.currentTarget);
      else closeChatControls();
    });
    document.getElementById("chat-controls-close").addEventListener("click", closeChatControls);
    document.getElementById("chat-lifecycle-refresh").addEventListener("click", function (e) {
      if (cstate.lifecycleAction) return;
      e.currentTarget.disabled = true;
      loadChatLifecycle().finally(function () { e.currentTarget.disabled = false; });
    });
    document.getElementById("chat-compact-btn").addEventListener("click", function () {
      var data = cstate.lifecycle;
      if (!data || data.busy || !data.capabilities.compact || !cstate.controls || !cstate.controls.usage.contextAvailable || !cstate.controls.usage.contextLimit) return;
      if (!window.confirm("Compact this session's context now? The transcript remains available.")) return;
      runLifecycleAction("/compact", {
        delivery: "queue",
        confirmation: { sessionID: data.session.id, updated: data.session.updated }
      }, function () {
        cstate.compactPending[cstate.session] = true;
        setChatStatus("Compaction queued. Progress will appear in the transcript.");
        refreshChat();
      }).catch(function () {});
    });
    document.getElementById("chat-rename-btn").addEventListener("click", function (e) {
      var value = document.getElementById("chat-rename-title").value.trim();
      if (!value || !cstate.lifecycle || cstate.lifecycleAction) return;
      var button = e.currentTarget;
      cstate.lifecycleAction = true;
      button.disabled = true;
      sessionManagementRequest("", "PATCH", { title: value }).then(function () {
        if (!cstate.lifecycle) return;
        cstate.lifecycle.session.title = value;
        cstate.title = value;
        document.getElementById("chat-title").textContent = value;
        setChatStatus("Session renamed. The persisted fallback title was updated.");
        return loadChatNavigation();
      }).catch(function (err) {
        setChatStatus("Rename failed: " + err.message, true);
      }).finally(function () {
        cstate.lifecycleAction = false;
        renderChatManagement();
      });
    });
    document.getElementById("chat-export-btn").addEventListener("click", function (e) {
      if (!cstate.session || cstate.lifecycleAction) return;
      var sessionID = cstate.session, button = e.currentTarget;
      cstate.lifecycleAction = true;
      button.disabled = true;
      fetch(lifecycleURL("/export"), { headers: { Accept: "application/json" } }).then(function (response) {
        if (!response.ok) return lifecycleResponse(response);
        return response.blob().then(function (blob) {
          var href = URL.createObjectURL(blob);
          var link = document.createElement("a");
          link.href = href;
          link.download = safeSessionExportFilename(sessionID);
          document.body.appendChild(link);
          link.click();
          link.remove();
          URL.revokeObjectURL(href);
          setChatStatus("Sanitized export downloaded.");
        });
      }).catch(function (err) {
        setChatStatus("Export failed: " + err.message, true);
      }).finally(function () {
        cstate.lifecycleAction = false;
        renderChatManagement();
      });
    });
    document.getElementById("chat-unlink-btn").addEventListener("click", function () {
      if (!cstate.lifecycle || !cstate.lifecycle.mapping || cstate.lifecycleAction) return;
      if (!window.confirm("Unlink this lessmess mapping only? The OpenCode session and its conversation will remain.")) return;
      cstate.lifecycleAction = true;
      sessionManagementRequest("/mapping", "DELETE").then(function () {
        if (cstate.lifecycle) cstate.lifecycle.mapping = null;
        setChatStatus("Mapping unlinked. The OpenCode session remains available.");
        loadChatNavigation();
      }).catch(function (err) {
        setChatStatus("Unlink failed: " + err.message, true);
      }).finally(function () {
        cstate.lifecycleAction = false;
        renderChatManagement();
      });
    });
    document.getElementById("chat-delete-preview-btn").addEventListener("click", function () {
      if (!cstate.session || cstate.lifecycleAction) return;
      var sessionID = cstate.session;
      cstate.lifecycleAction = true;
      cstate.deletePreview = null;
      renderChatManagement();
      sessionManagementRequest("/delete-preview", "GET").then(function (preview) {
        if (cstate.session === sessionID) cstate.deletePreview = preview;
      }).catch(function (err) {
        setChatStatus("Deletion preview failed: " + err.message, true);
      }).finally(function () {
        if (cstate.session === sessionID) {
          cstate.lifecycleAction = false;
          renderChatManagement();
        }
      });
    });
    document.getElementById("chat-delete-preview").addEventListener("click", function (e) {
      var button = e.target.closest("[data-chat-delete-confirm]");
      var preview = cstate.deletePreview;
      var input = document.getElementById("chat-delete-title");
      if (!button || !preview || !input || input.value !== preview.title || cstate.lifecycleAction) return;
      if (!window.confirm("Permanently delete this OpenCode session, all displayed descendants, and every displayed lessmess mapping? This is not unlink.")) return;
      cstate.lifecycleAction = true;
      button.disabled = true;
      sessionManagementRequest("", "DELETE", { confirmation: {
        sessionID: preview.sessionID, fingerprint: preview.fingerprint, count: preview.count, title: preview.title
      }}).then(function () {
        setChatStatus("OpenCode session tree and affected mappings deleted.");
        closeChat();
      }).catch(function (err) {
        if (err.status === 409) cstate.deletePreview = null;
        setChatStatus("Delete failed: " + err.message, true);
      }).finally(function () {
        cstate.lifecycleAction = false;
        if (chatOpen()) renderChatManagement();
      });
    });
    document.getElementById("chat-agent-select").addEventListener("change", function (e) {
      var select = e.currentTarget, prior = cstate.controls && cstate.controls.agent;
      chatMutation("agent", { agent: select.value }, select).then(loadChatControls).catch(function () { select.value = prior || ""; });
    });
    document.getElementById("chat-model-select").addEventListener("change", function (e) {
      var select = e.currentTarget, prior = cstate.controls && cstate.controls.model;
      chatMutation("model", { model: select.value }, select).then(loadChatControls).catch(function () { select.value = prior || ""; });
    });
    document.getElementById("chat-command-run").addEventListener("click", function (e) {
      var command = document.getElementById("chat-command-select").value;
      if (!command) { setChatStatus("Choose a command first.", true); return; }
      chatMutation("command", { command: command, arguments: document.getElementById("chat-command-args").value }, e.currentTarget).then(function () {
        document.getElementById("chat-command-args").value = "";
        closeChatControls();
      }).catch(function () {});
    });
    document.getElementById("chat-skill-list").addEventListener("click", function (e) {
      var attach = e.target.closest("[data-attach-skill]");
      var activate = e.target.closest("[data-activate-skill]");
      var id = attach ? attach.dataset.attachSkill : activate && activate.dataset.activateSkill;
      if (!id || !cstate.controls) return;
      var skill = (cstate.controls.skills || []).find(function (item) { return item.id === id; });
      if (attach && skill) {
        delete cstate.deliveryIDs[cstate.session];
        if (!chatDraftSkills().some(function (item) { return item.id === id; })) chatDraftSkills().push(skill);
        renderChatSkillChips();
        closeChatControls();
      } else if (activate) {
        chatMutation("skill", { skill: id }, activate).then(closeChatControls).catch(function () {});
      }
    });
    document.getElementById("chat-controls-search").addEventListener("input", function (e) {
      var query = e.currentTarget.value.toLowerCase();
      ["chat-agent-select", "chat-model-select", "chat-command-select"].forEach(function (id) {
        document.getElementById(id).querySelectorAll("option[data-search]").forEach(function (option) { option.hidden = !!query && option.dataset.search.indexOf(query) < 0; });
      });
      document.querySelectorAll("#chat-skill-list .chat-skill-row").forEach(function (row) { row.hidden = !!query && row.dataset.search.indexOf(query) < 0; });
    });
    document.getElementById("chat-terminal-btn").addEventListener("click", function () {
      var sid = cstate.session, title = cstate.title;
      openTerminal(sid, title);
    });
    document.getElementById("terminal-chat-btn").addEventListener("click", function () {
      if (tstate.session) openChat(tstate.session, document.getElementById("terminal-title").textContent);
    });
    document.getElementById("chat-agents-btn").addEventListener("click", function (e) {
      var drawer = document.getElementById("chat-agents");
      if (drawer.classList.contains("open") && !compactChatUI()) closeChatAgents();
      else openChatAgents(e.currentTarget);
    });
    document.getElementById("chat-agent-backdrop").addEventListener("click", closeChatAgents);
    document.getElementById("chat-tasks-btn").addEventListener("click", function (e) {
      var panel = document.getElementById("chat-tasks");
      if (panel.classList.contains("open") && !compactChatUI()) closeChatTasks();
      else openChatTasks(e.currentTarget);
    });
    document.getElementById("chat-task-backdrop").addEventListener("click", closeChatTasks);
    document.querySelector(".chat-window").addEventListener("click", function (e) {
      if (e.target.closest("[data-chat-view-back]")) closeChatView();
    });
    function menuKeyboard(menu, close) {
      menu.addEventListener("keydown", function (e) {
        var items = Array.prototype.filter.call(menu.querySelectorAll('[role="menuitem"]'), function (item) { return !item.disabled && !item.hidden; });
        var index = items.indexOf(document.activeElement);
        if (e.key === "Escape") { e.preventDefault(); close(true); return; }
        if (e.key !== "ArrowDown" && e.key !== "ArrowUp" && e.key !== "Home" && e.key !== "End") return;
        e.preventDefault();
        if (e.key === "Home") index = 0;
        else if (e.key === "End") index = items.length - 1;
        else index = (index + (e.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
        if (items[index]) items[index].focus({ preventScroll: true });
      });
    }
    menuKeyboard(actionsSheet, closeChatActions);
    menuKeyboard(moreMenu, closeChatMore);

    var compactQuery = window.matchMedia && window.matchMedia("(max-width: 840px)");
    if (compactQuery) compactQuery.addEventListener("change", function () {
      closeChatActions(false);
      closeChatMore(false);
      closeChatReferences(false);
      closeChatTasks(false);
      closeChatAgents(false);
      closeChatControls(false);
      resetChatViews();
    });
    if (window.visualViewport) {
      cstate.viewportHandler = syncChatViewport;
      window.visualViewport.addEventListener("resize", cstate.viewportHandler);
      window.visualViewport.addEventListener("scroll", cstate.viewportHandler);
    }
    document.addEventListener("visibilitychange", function () {
      if (document.hidden) {
        clearTimeout(cstate.timer);
        if (cstate.request) cstate.request.abort();
        cstate.request = null;
        document.getElementById("chat-transcript").setAttribute("aria-busy", "false");
      } else if (chatOpen()) {
        refreshChat();
        loadChatLifecycle();
      }
    });
    window.addEventListener("beforeunload", function () {
      Object.keys(cstate.files).forEach(function (sessionID) { (cstate.files[sessionID] || []).forEach(revokeChatFile); });
    });
    document.addEventListener("keydown", function (e) {
      if (e.key !== "Escape") return;
      var detail = document.getElementById("detail");
      if (detail && !detail.hidden) return;
      if (closeChatMessageActions(true) || closeChatActions(true) || closeChatMore(true) || closeChatReferences(true)) {
        e.preventDefault();
        e.stopImmediatePropagation();
      }
    }, true);
    document.addEventListener("keydown", function (e) {
      if (e.key !== "Tab" || !chatOpen()) return;
      var detail = document.getElementById("detail");
      if (detail && !detail.hidden) return;
      var win = document.querySelector(".chat-window");
      var focusable = Array.prototype.filter.call(win.querySelectorAll('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])'), function (item) {
        return !item.hidden && !item.closest("[hidden], [inert]") && item.getClientRects().length > 0;
      });
      if (!focusable.length) return;
      var first = focusable[0], last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    });
  })();

  document.addEventListener("click", function (e) {
    var menu = e.target.closest("#chat-transcript .chat-message-menu");
    if (menu) {
      if (e.target.closest("button")) closeChatMessageActions(false);
      return;
    }
    var message = e.target.closest("#chat-transcript [data-chat-message-actions]");
    if (message && !e.target.closest("a, button, input, select, textarea, details, summary")) {
      openChatMessageActions(message, false);
      return;
    }
    closeChatMessageActions(false);
  }, true);

  document.addEventListener("keydown", function (e) {
    var message = e.target.closest && e.target.closest("#chat-transcript [data-chat-message-actions]");
    if (!message || (e.key !== "Enter" && e.key !== " ")) return;
    if (e.target.closest(".chat-message-menu")) return;
    e.preventDefault();
    openChatMessageActions(message, true);
  });

  document.addEventListener("click", function (e) {
    var fork = e.target.closest("#chat-transcript [data-chat-fork]");
    if (fork) {
      e.preventDefault();
      var parentID = cstate.session;
      runLifecycleAction("/fork", { before: fork.dataset.chatFork }, function (result) {
        if (!result || !result.session) throw new Error("Fork response did not include the child session");
        cstate.scrolls[parentID] = document.getElementById("chat-transcript").scrollTop;
        markOpened(result.session.id);
        openChat(result.session.id, result.session.title || "Forked session");
      }).catch(function (err) { setChatStatus("Fork failed: " + err.message, true); });
      return;
    }
    var revert = e.target.closest("#chat-transcript [data-chat-revert]");
    if (revert) {
      e.preventDefault();
      previewChatRevert(revert.dataset.chatRevert);
      return;
    }
    var refreshRevert = e.target.closest("[data-chat-revert-refresh]");
    if (refreshRevert) {
      e.preventDefault();
      previewChatRevert(refreshRevert.dataset.chatRevertRefresh);
      return;
    }
    var stageRevert = e.target.closest("[data-chat-revert-stage]");
    if (stageRevert) {
      e.preventDefault();
      var stageData = cstate.lifecycle;
      if (!stageData || !cstate.revertPreviewValid) return;
      runLifecycleAction("/revert/stage", {
        messageID: stageRevert.dataset.chatRevertStage,
        files: document.getElementById("chat-revert-files").checked,
        confirmation: { sessionID: stageData.session.id, updated: stageData.session.updated }
      }, function () {
        cstate.revertPreviewValid = true;
        setChatStatus("Revert staged. Review the authoritative staged state before committing or cancelling.");
      }).catch(function () {});
      return;
    }
    var finalRevert = e.target.closest("[data-chat-revert-commit], [data-chat-revert-cancel]");
    if (finalRevert) {
      e.preventDefault();
      var finalData = cstate.lifecycle;
      var fingerprint = lifecycleFingerprint(finalData);
      var fingerprintInput = document.getElementById("chat-revert-fingerprint");
      if (!finalData || !finalData.session.revert || !fingerprintInput || fingerprintInput.value !== fingerprint) return;
      var committing = finalRevert.hasAttribute("data-chat-revert-commit");
      if (committing && !cstate.revertPreviewValid) return;
      var prompt = committing
        ? "Commit this staged revert? This applies exactly the displayed staged transcript and file operation."
        : "Cancel this staged revert? OpenCode may not restore files after clearing a files-enabled stage.";
      if (!window.confirm(prompt)) return;
      runLifecycleAction(committing ? "/revert/commit" : "/revert/clear", {
        confirmation: {
          sessionID: finalData.session.id,
          updated: finalData.session.updated,
          messageID: finalData.session.revert.messageID
        }
      }, function () {
        cstate.revertTarget = null;
        cstate.revertPreviewHTML = "";
        cstate.revertPreviewValid = false;
        setChatStatus(committing ? "Revert committed." : "Staged revert cancelled.");
        refreshChat();
      }).catch(function () {});
      return;
    }
    var copy = e.target.closest("#chat-transcript [data-chat-copy]");
    if (copy) {
      e.preventDefault();
      var section = copy.closest("section, .chat-diff-file");
      var source = section && section.querySelector(".chat-copy-source");
      if (source && navigator.clipboard) navigator.clipboard.writeText(source.textContent).then(function () {
        var old = copy.textContent;
        copy.textContent = "Copied";
        setTimeout(function () { if (document.contains(copy)) copy.textContent = old; }, 1200);
      }).catch(function () { setChatStatus("Could not copy text.", true); });
      return;
    }
    var copyMessage = e.target.closest("#chat-transcript [data-chat-copy-message]");
    if (copyMessage) {
      e.preventDefault();
      var message = copyMessage.closest("[data-chat-message-actions]");
      var text = Array.prototype.map.call(message ? message.querySelectorAll(".chat-markdown") : [], function (part) {
        return part.textContent.trim();
      }).filter(Boolean).join("\n\n");
      if (!text || !navigator.clipboard) {
        setChatStatus("Could not copy message.", true);
        return;
      }
      navigator.clipboard.writeText(text).then(function () {
        setChatStatus("Message copied.");
      }).catch(function () { setChatStatus("Could not copy message.", true); });
      return;
    }
    var more = e.target.closest("#chat-transcript [data-chat-load-more]");
    if (more) {
      e.preventDefault();
      more.disabled = true;
      fetch(more.dataset.chatLoadMore, { headers: { Accept: "text/html" } })
        .then(function (r) { if (!r.ok) throw new Error("HTTP " + r.status); return r.text(); })
        .then(function (html) {
          var holder = document.createElement("template");
          holder.innerHTML = html.trim();
          var incoming = holder.content.querySelector(".chat-tool-detail");
          var current = more.closest(".chat-tool-detail");
          var output = incoming && incoming.querySelector(".chat-tool-output");
          var target = current && current.querySelector(".chat-tool-output");
          if (output && target) target.textContent += output.textContent;
          else if (output && current) current.insertBefore(output.closest("section"), more);
          var next = incoming && incoming.querySelector("[data-chat-load-more]");
          if (next) { more.dataset.chatLoadMore = next.dataset.chatLoadMore; more.disabled = false; }
          else {
            var capped = incoming && incoming.querySelector(".chat-truncated");
            if (capped && current) current.appendChild(capped);
            more.remove();
          }
        })
        .catch(function (err) { more.disabled = false; setChatStatus("Could not load more output. " + err.message, true); });
      return;
    }
    var loadOlder = e.target.closest("#chat-transcript [data-chat-load-older]");
    if (loadOlder) {
      e.preventDefault();
      loadOlderChat(loadOlder);
      return;
    }
    var decision = e.target.closest('#chat-transcript [data-permission-decision], #chat-transcript [data-decision], #chat-transcript button[name="decision"]');
    if (!decision) return;
    var request = decision.closest("[data-permission], [data-request-id], [data-permission-id]");
    if (!request) return;
    e.preventDefault();
    var message = request.querySelector('[name="message"]');
    var requestID = request.dataset.permission || request.dataset.requestId || request.dataset.permissionId;
    chatMutation(
      "permissions/" + encodeURIComponent(requestID) + "/reply",
      { decision: decision.dataset.permissionDecision || decision.dataset.decision || decision.value, message: message && message.value ? message.value : undefined },
      decision
    ).catch(function () {});
  });

  document.addEventListener("submit", function (e) {
    var permission = e.target.closest("#chat-transcript form[data-permission], #chat-transcript form[data-request-id], #chat-transcript form[data-permission-id]");
    if (permission) {
      e.preventDefault();
      var submitter = e.submitter || permission.querySelector('[type="submit"]');
      var message = permission.querySelector('[name="message"]');
      var requestID = permission.dataset.permission || permission.dataset.requestId || permission.dataset.permissionId;
      chatMutation(
        "permissions/" + encodeURIComponent(requestID) + "/reply",
        { decision: submitter && (submitter.dataset.permissionDecision || submitter.dataset.decision || submitter.value), message: message && message.value ? message.value : undefined },
        submitter
      ).catch(function () {});
      return;
    }
    var form = e.target.closest("#chat-transcript form[data-form-id], #chat-transcript form[data-chat-form]");
    if (!form) return;
    e.preventDefault();
    var answers = {};
    form.querySelectorAll("[name]").forEach(function (field) {
      var type = field.dataset.fieldType || "string";
      if (type === "boolean") answers[field.name] = field.checked;
      else if (type === "multiselect") {
        answers[field.name] = Array.prototype.filter.call(field.options, function (o) { return o.selected; })
          .map(function (o) { return o.value; });
      } else if (type === "integer") {
        answers[field.name] = field.value === "" ? null : parseInt(field.value, 10);
      } else if (type === "number") {
        answers[field.name] = field.value === "" ? null : Number(field.value);
      } else {
        answers[field.name] = field.value;
      }
    });
    chatMutation(
      "forms/" + encodeURIComponent(form.dataset.formId || form.dataset.chatForm) + "/reply",
      { answers: answers },
      form.querySelector('[type="submit"]')
    ).catch(function () {});
  });

  var tstate = { term: null, ws: null, ro: null, session: null };
  // The change id the task panel currently shows for a non-board terminal
  // (null on board pages, where the panel mirrors the live board DOM).
  var terminalPanelChange = null;

  function openTerminal(sessionID, title) {
    var fromChat = chatOpen();
    closeChat(true, true);
    closeTerminal(fromChat);
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
        // Follow the accent palette; falls back to the legacy orange when
        // the CSS variable is unavailable.
        cursor: getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || "#e8641f",
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

  function closeTerminal(suppressRefresh) {
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
    if (!suppressRefresh && page === "index") scheduleRefresh();
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
    renderTaskPanel(panel, src, change);
  }

  function renderTaskPanel(panel, src, change) {
    var chatWork = panel.id === "chat-tasks";
    var html = '<div class="chat-view-head"><button type="button" class="chat-view-back btn-ghost" data-chat-view-back>Back to Chat</button><strong>Work</strong></div><div class="ttp-scroll" data-chat-view-scroll>';
    if (chatWork && change) {
      html += '<a class="ttp-plan ttp-plan-first" data-work-document="plan" hx-get="/changes/' +
        encodeURIComponent(change) + '/plan" hx-target="#detail" hx-swap="innerHTML"><span><strong>Plan</strong><small>Read the change plan</small></span><span aria-hidden="true">&rsaquo;</span></a>';
    }
    html += '<div class="ttp-head">Tasks</div>';
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
          html += '<a class="ttp-row"' + (chatWork ? ' data-work-document="task"' : '') + ' hx-get="' + esc(href) + '" hx-headers=\'{"Accept": "text/html"}\'' +
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
        if (!sessionOverlayOpen() || activeSessionID() !== sessionID) return;
        loadTerminalTasks(j.change);
      })
      .catch(function () {});
  }

  function loadTerminalTasks(changeID) {
    fetch("/changes/" + encodeURIComponent(changeID), { headers: { Accept: "text/html", "HX-Request": "true" } })
      .then(function (r) { return r.ok ? r.text() : null; })
      .then(function (html) {
        if (html == null || !sessionOverlayOpen()) return;
        var src = document.createElement("div");
        src.innerHTML = html;
        terminalPanelChange = changeID;
        if (terminalOpen()) {
          var panel = terminalTasksEl();
          if (!panel) return;
          panel.hidden = false;
          syncTerminalTasks(src, changeID);
        }
        if (chatOpen()) setChatTasks(src, changeID);
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
        openPreferredSession(sid, s ? s.title : sid);
      })
      .catch(function () { openPreferredSession(sid, sid); });
  }

  document.addEventListener("click", function (e) {
    if (e.target.closest("[data-close-terminal]")) { closeTerminal(); return; }
    if (e.target.closest("[data-close-chat]")) { closeChat(); return; }
    var ov = document.getElementById("terminal-overlay");
    if (ov && !ov.hidden && e.target === ov) closeTerminal();
    var chat = document.getElementById("chat-overlay");
    if (chat && !chat.hidden && e.target === chat) {
      if (!closeTopChatAuxiliary()) closeChat();
    }
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
      headers: { Accept: "application/json", "X-Lessmess-UI": "1" },
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

    // Worktree strip: explicit cleanup action. The server refuses a dirty
    // worktree (422) and a disabled pipeline (409); both surface here.
    var wtRemoveBtn = document.getElementById("worktree-remove-btn");
    if (wtRemoveBtn) {
      wtRemoveBtn.addEventListener("click", function () {
        if (!window.confirm("Remove this change's worktree? The branch and its commits are kept.")) return;
        wtRemoveBtn.disabled = true;
        fetch("/changes/" + changeID() + "/worktree/remove", {
          method: "POST",
          headers: { Accept: "application/json" },
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function () { location.reload(); })
          .catch(function (e) {
            wtRemoveBtn.disabled = false;
            alert("Worktree removal failed: " + e.message);
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
    if (chatOpen()) syncChatModalInert(false);
    if (cstate.detailViewportHandler && window.visualViewport) {
      window.visualViewport.removeEventListener("resize", cstate.detailViewportHandler);
    }
    if (cstate.detailTOCMedia && cstate.detailTOCHandler) {
      cstate.detailTOCMedia.removeEventListener("change", cstate.detailTOCHandler);
    }
    cstate.detailViewportHandler = null;
    cstate.detailTOCMedia = null;
    cstate.detailTOCHandler = null;
    var opener = cstate.detailOpener;
    cstate.detailOpener = null;
    if (opener && opener.isConnected) requestAnimationFrame(function () { opener.focus({ preventScroll: true }); });
  }

  function closeDetailContents(returnFocus) {
    var toggle = document.getElementById("detail-contents-toggle");
    var list = document.getElementById("detail-contents-list");
    if (!compactChatUI() || !toggle || !list || toggle.getAttribute("aria-expanded") !== "true") return false;
    toggle.setAttribute("aria-expanded", "false");
    list.hidden = true;
    if (returnFocus !== false) toggle.focus({ preventScroll: true });
    return true;
  }

  document.addEventListener("htmx:beforeRequest", function (e) {
    var opener = e.detail && e.detail.elt;
    if (opener && opener.matches('[hx-target="#detail"]')) {
      cstate.detailOpener = opener;
    }
  });

  document.addEventListener("click", function (e) {
    if (e.target.closest("[data-close-detail]")) { closeDetail(); return; }
    var d = document.getElementById("detail");
    if (d && !d.hidden && e.target === d && !closeDetailContents(true)) closeDetail(); // backdrop click
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    var d = document.getElementById("detail");
    if (d && !d.hidden) {
      if (!closeDetailContents(true)) closeDetail();
      return;
    } // the detail modal stacks above the terminal
    if (chatOpen()) {
      if (!closeTopChatAuxiliary()) closeChat();
      return;
    }
    if (terminalOpen()) { closeTerminal(); return; }
    closeDetail();
  });

  document.addEventListener("htmx:afterSwap", function (e) {
    if (e.target && e.target.id === "detail") {
      e.target.hidden = false;
      if (chatOpen()) syncChatModalInert(true);
      buildDetailTOC();
      requestAnimationFrame(function () {
        var close = e.target.querySelector("[data-close-detail]");
        if (close) close.focus({ preventScroll: true });
      });
    }
  });

  // --- detail modal TOC --------------------------------------------------------
  // The server renders headings with ids (goldmark auto heading IDs); we build
  // the contents from the swapped DOM so all detail shapes share one code
  // path. CSS keeps this as a desktop rail and moves it above the document at
  // compact widths.

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
    if (cstate.detailViewportHandler && window.visualViewport) {
      window.visualViewport.removeEventListener("resize", cstate.detailViewportHandler);
    }
    if (cstate.detailTOCMedia && cstate.detailTOCHandler) {
      cstate.detailTOCMedia.removeEventListener("change", cstate.detailTOCHandler);
    }
    cstate.detailViewportHandler = null;
    cstate.detailTOCMedia = null;
    cstate.detailTOCHandler = null;
    if (window.visualViewport) {
      cstate.detailViewportHandler = function () {
        modal.style.setProperty("--detail-viewport-height", window.visualViewport.height + "px");
      };
      cstate.detailViewportHandler();
      window.visualViewport.addEventListener("resize", cstate.detailViewportHandler);
    }
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
    var toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "toc-toggle";
    toggle.id = "detail-contents-toggle";
    toggle.setAttribute("aria-controls", "detail-contents-list");
    toggle.innerHTML = '<span>Contents</span><span class="toc-chevron" aria-hidden="true">⌄</span>';
    toc.appendChild(toggle);
    var list = document.createElement("div");
    list.className = "toc-links";
    list.id = "detail-contents-list";
    toc.appendChild(list);
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
        if (compactChatUI()) {
          toggle.setAttribute("aria-expanded", "false");
          list.hidden = true;
        }
        requestAnimationFrame(function () {
          body.scrollTo({ top: Math.max(0, paneOffset(body, h) - 12), behavior: "smooth" });
          if (compactChatUI()) {
            if (!h.hasAttribute("tabindex")) h.setAttribute("tabindex", "-1");
            h.focus({ preventScroll: true });
          }
        });
      });
      list.appendChild(a);
    });
    toc.hidden = false;
    modal.classList.add("has-toc");

    function syncDisclosure(compact) {
      toggle.setAttribute("aria-expanded", String(!compact));
      list.hidden = compact;
    }
    var media = window.matchMedia("(max-width: 840px)");
    syncDisclosure(media.matches);
    toggle.addEventListener("click", function () {
      if (!media.matches) return;
      var expanded = toggle.getAttribute("aria-expanded") === "true";
      toggle.setAttribute("aria-expanded", String(!expanded));
      list.hidden = expanded;
    });
    cstate.detailTOCMedia = media;
    cstate.detailTOCHandler = function (event) { syncDisclosure(event.matches); };
    media.addEventListener("change", cstate.detailTOCHandler);

    var links = list.querySelectorAll("a");
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

  // --- task expansion (sub plans) ----------------------------------------------
  // The expand button on a card is the user-instructed decomposition
  // action: POST /expand creates the container; both success (201) and
  // already-exists (409) end on the task's sub-board.
  document.addEventListener("click", function (e) {
    var btn = e.target.closest("[data-expand]");
    if (!btn) return;
    var board = boardEl();
    if (!board) return;
    var change = board.getAttribute("data-change");
    var task = btn.getAttribute("data-expand");
    btn.disabled = true;
    fetch("/changes/" + encodeURIComponent(change) + "/expand", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ task: task }),
    })
      .then(function (r) {
        if (!r.ok && r.status !== 409) {
          return r.json().then(function (j) { throw new Error(j.error || r.statusText); });
        }
        location.href = "/changes/" + encodeURIComponent(change) + "?task=" + encodeURIComponent(task);
      })
      .catch(function (err) {
        alert("Expand failed: " + err.message);
        btn.disabled = false;
      });
  });

  // --- detail modal links ------------------------------------------------------
  // Relative .md links in the markdown (ledger, tasks, plan) open in this
  // modal instead of navigating to a nonexistent URL.

  // resolveDocPath resolves a relative .md link against the document
  // currently shown in the modal (its change-relative href), collapsing
  // ../ segments lexically.
  function resolveDocPath(link, doc) {
    var base = String(doc || "").split("#")[0];
    if (!base) return link;
    var parts = base.split("/");
    parts.pop(); // the document file itself
    link.split("/").forEach(function (seg) {
      if (seg === "..") parts.pop();
      else if (seg !== "." && seg !== "") parts.push(seg);
    });
    return parts.join("/");
  }

  function detailLinkTarget(href) {
    if (!href || !/\.md$/i.test(href.split("#")[0])) return null;
    if (/^[a-z][a-z0-9+.-]*:/i.test(href) || href.charAt(0) === "/") return null; // absolute: leave alone
    var hash = href.indexOf("#") >= 0 ? href.slice(href.indexOf("#") + 1) : "";
    var path = href.split("#")[0];
    var id = currentChangeID();
    var m;
    // Resolve relative to the shown document when the modal carries one,
    // so ../ledger.md from a nested task opens that container's ledger.
    var modal = document.querySelector("#detail .modal");
    if (modal && modal.getAttribute("data-doc")) {
      path = resolveDocPath(path, modal.getAttribute("data-doc"));
    }
    if ((m = path.match(/^tasks\/(.+\.md)$/i)) && id) {
      // The last segment decides: a task file vs a container ledger.
      if (/\/ledger\.md$/i.test(m[1])) {
        return { url: "/changes/" + id + "/ledger?href=" + encodeURIComponent(path), frag: hash };
      }
      return { url: "/changes/" + id + "/tasks/" + m[1], frag: hash };
    }
    if ((m = path.match(/^(\d{4}-\d{2}-\d{2}-[a-z0-9]+)\/plan\.md$/i))) {
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
    // True between a swatch click and the next legitimate re-sync (load,
    // scope switch, successful save): render() must not clobber the
    // pending pick with the layer's stored value in that window.
    var accentTouched = false;
    // Palette entries by id (filled when the swatches are built), so a
    // pick can live-preview by rewriting the head style element.
    var accentsById = {};

    // previewAccent rewrites the #accent-style element so a pending pick
    // is visible before saving; with no id it falls back to the effective
    // accent (or the default), which is also how a pick is un-done.
    function previewAccent(id) {
      var el = document.getElementById("accent-style");
      if (!el) return;
      var effId = (view && view.effective && view.effective.ui && view.effective.ui.accent) || "orange";
      var a = accentsById[id || effId] || accentsById.orange;
      if (!a) return;
      el.textContent = ':root{--accent:' + a.hex + ';--accent-hover:' + a.darkHover +
        '}[data-theme="light"]{--accent:' + a.light + ';--accent-hover:' + a.lightHover + '}';
    }

    var BOOL_DEFAULTS = {
      "session.autoOpenTerminal": true,
      "ui.showArchived": true,
      "docs.autoGardenerOnClose": true,
      "git.worktrees": false,
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
      if (field === "git.reviewModel") return getPath(view.effective, "session.model");
      if (field === "general.projectName") return (view && view.defaultProjectName) || undefined;
      if (field in BOOL_DEFAULTS) return BOOL_DEFAULTS[field];
      return undefined;
    }

    function placeholderFor(field, fb) {
      if (field === "docs.gardenerModel" || field === "git.reviewModel") {
        return isSet(fb) ? "Session model (" + fb + ")" : "Service default";
      }
      if (isSet(fb)) return String(fb);
      if (field === "general.projectName") return (view && view.defaultProjectName) || "";
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
        } else if (kind === "accent") {
          // The picker's pending selection lives on the container as
          // data-value. Only sync it from the layer while untouched, and
          // always mark the chip matching the current data-value.
          if (!accentTouched) {
            input.setAttribute("data-value", isSet(lv) ? lv : "");
          }
          var picked = input.getAttribute("data-value") || "";
          input.querySelectorAll(".accent-swatch").forEach(function (chip) {
            var id = chip.getAttribute("data-id") || "";
            chip.classList.toggle("selected", id === picked);
            if (id === "") {
              chip.title = isSet(fb) ? "Auto — inherits " + fb + " until you roll again" : "Auto — a fresh random color";
            }
          });
          // Surface layering: personal wins over project, so a save into
          // the project scope is shadowed whenever the personal layer
          // defines the accent — point the user at the right scope.
          var note = f.querySelector(".accent-shadow-note");
          if (note) {
            var srcNow = (view.sources && view.sources[field]) || "default";
            var shadowed = scope === "project" && srcNow === "personal";
            note.textContent = shadowed ? "A personal-layer override wins over this scope — switch the scope to Personal to change the color." : "";
            note.hidden = !shadowed;
          }
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
      renderOpencodeDefaultNote();
    }

    // renderOpencodeDefaultNote warns when the repository's opencode.json
    // declares its own default_agent that differs from the effective
    // session.agent: sessions created outside lessmess (opencode TUI/CLI)
    // follow opencode's default, not the agent configured above. The align
    // button (an explicit, user-consented write into the committed
    // opencode.json) appears only on divergence.
    function renderOpencodeDefaultNote() {
      var note = document.getElementById("opencode-default-note");
      if (!note) return;
      var od = view.opencodeDefaultAgent;
      var effAgent = (view.effective && view.effective.session && view.effective.session.agent) || "";
      var divergent = od && od.status === "ok" && od.declared && od.declared !== effAgent;
      if (divergent) {
        document.getElementById("opencode-default-text").textContent =
          "opencode.json sets its own default agent \u201C" + od.declared +
          "\u201D. Sessions you create outside lessmess (opencode TUI/CLI) use it instead of the agent above" +
          (effAgent ? " (\u201C" + effAgent + "\u201D)" : "") + ".";
      }
      note.hidden = !divergent;
      var btn = document.getElementById("align-default-agent");
      if (btn) btn.hidden = !divergent;
    }

    function renderOptions() {
      var hint = document.getElementById("settings-options-hint");
      if (!options) {
        hint.hidden = false;
        return;
      }
      // The accent palette is static server data — build the picker even
      // when the opencode service is unreachable.
      buildAccentSwatches();
      // Local branch names come from git, not opencode — fill the
      // git.defaultBranch combobox regardless of service availability.
      var bl = document.getElementById("settings-branch-list");
      (options.branches || []).forEach(function (b) {
        var o = document.createElement("option");
        o.value = b;
        bl.appendChild(o);
      });
      if (!options.available) {
        hint.hidden = false;
        return;
      }
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
      hint.hidden = true;
    }

    // buildAccentSwatches fills the ui.accent picker once per options
    // load: one chip per palette entry plus the Auto chip. Chips only set
    // the container's pending data-value (and flag it as touched) — the
    // actual PUT happens with the section's Save button.
    function buildAccentSwatches() {
      var sw = root.querySelector('[data-field="ui.accent"] [data-input]');
      if (!sw || sw.childElementCount) return;
      accentsById = {};
      (options.accents || []).forEach(function (a) {
        accentsById[a.id] = a;
        var chip = document.createElement("button");
        chip.type = "button";
        chip.className = "accent-swatch";
        chip.style.background = a.hex;
        chip.setAttribute("data-id", a.id);
        chip.setAttribute("data-label", a.name);
        chip.title = a.name + " — click to preview, Save to keep";
        chip.addEventListener("click", function () {
          accentTouched = true;
          sw.setAttribute("data-value", a.id);
          previewAccent(a.id);
          render();
        });
        sw.appendChild(chip);
      });
      var auto = document.createElement("button");
      auto.type = "button";
      auto.className = "accent-swatch accent-inherit";
      auto.setAttribute("data-id", "");
      auto.textContent = "Auto";
      auto.title = "Auto — a fresh random color";
      auto.addEventListener("click", function () {
        accentTouched = true;
        sw.setAttribute("data-value", "");
        previewAccent(null);
        render();
      });
      sw.appendChild(auto);
      if (view) render();
    }

    // syncAccentValues resets the picker's pending selection to the layer
    // being edited: initial load, scope switch, and successful save.
    function syncAccentValues() {
      accentTouched = false;
      var layer = scope === "project" ? view.project : view.personal;
      root.querySelectorAll('[data-field="ui.accent"] [data-input]').forEach(function (input) {
        var lv = getPath(layer, "ui.accent");
        input.setAttribute("data-value", isSet(lv) ? lv : "");
      });
    }

    root.querySelectorAll('input[name="settings-scope"]').forEach(function (radio) {
      radio.addEventListener("change", function () {
        scope = radio.value;
        if (view) syncAccentValues();
        previewAccent(null); // discard any un-saved preview
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
        .then(function (j) { openPreferredSession(j.session, j.title); })
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
          } else if (kind === "accent") {
            payload[section][key] = input.getAttribute("data-value") || "";
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
            // The project name is server-rendered chrome (header, tab
            // title): reload so it repaints from the saved value.
            if (section === "general") { location.reload(); return; }
            view = j;
            syncAccentValues();
            previewAccent(null); // the saved value is now the effective one
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

    // loadSettings fetches the current payload and repaints; used at init
    // and after the align action (which changes server-side state the
    // notice renders).
    function loadSettings() {
      return fetch("/api/settings", { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) { view = j; render(); })
        .catch(function () {});
    }
    loadSettings();
    fetch("/api/settings/options", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) { options = j; renderOptions(); render(); })
      .catch(function () {
        document.getElementById("settings-options-hint").hidden = false;
      });

    // Align action: write opencode.json's default_agent so sessions created
    // outside lessmess default to the same agent configured here.
    var alignBtn = document.getElementById("align-default-agent");
    if (alignBtn) {
      alignBtn.addEventListener("click", function () {
        alignBtn.disabled = true;
        fetch("/api/settings/opencode-default-agent", {
          method: "POST",
          headers: { Accept: "application/json" },
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function () { loadSettings(); })
          .catch(function (err) { alert("Align failed: " + err.message); })
          .finally(function () { alignBtn.disabled = false; });
      });
    }

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
      nameSaved: false,
      docsOutcome: "",       // "seeded" | "skipped" | "pending" (failed)
    };

    var steps = ["prereqs", "name", "bootstrap", "agent", "docs", "finish"];

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
      if (name === "name") enterName();
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
    el("setup-prereqs-next").addEventListener("click", function () { showStep("name"); });

    // --- step 2: project name ---

    // Prefill from the effective settings (already carries the folder-name
    // fallback); only fill an untouched input so re-entering the step
    // never clobbers an edit, and degrade silently offline.
    function enterName() {
      var input = el("setup-project-name");
      if (input.value !== "") return;
      fetch("/api/settings", { headers: { Accept: "application/json" } })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          if (input.value !== "") return;
          if (j.effective && j.effective.general && j.effective.general.projectName) {
            input.value = j.effective.general.projectName;
          }
          if (j.defaultProjectName) input.placeholder = j.defaultProjectName;
        })
        .catch(function () {});
    }

    function nameScope() {
      var r = root.querySelector('input[name="setup-name-scope"]:checked');
      return r ? r.value : "project";
    }

    el("setup-name-save").addEventListener("click", function () {
      var status = el("setup-name-status");
      var payload = { general: { projectName: el("setup-project-name").value.trim() } };
      status.textContent = "Saving…";
      fetch("/api/settings?scope=" + nameScope(), {
        method: "PUT",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify(payload),
      })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(function () {
          state.nameSaved = true;
          status.textContent = "Saved ✓";
          showStep("bootstrap");
        })
        .catch(function (e) { status.textContent = e.message; });
    });
    el("setup-name-skip").addEventListener("click", function () { showStep("bootstrap"); });

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
      items.push(state.nameSaved ? "Project name saved" : "Project name follows the folder name");
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
      if (state.nameSaved) stepMarks.name = "set";
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

  // --- OpenCode service status -----------------------------------------------

  function initOpencodeStatus() {
    var root = document.getElementById("opencode-status-page");
    if (!root) return;
    var message = document.getElementById("oc-status-message");
    var summary = document.getElementById("oc-status-summary");
    var rediscover = document.getElementById("oc-rediscover");
    var returnChat = document.getElementById("oc-return-chat");
    returnChat.addEventListener("click", function () {
      var chat = document.getElementById("chat-btn");
      if (chat) chat.click();
    });

    function finding(section, finding) {
      var body = root.querySelector('[data-oc-section="' + section + '"] [data-oc-body]');
      body.textContent = "";
      var pill = document.createElement("span");
      pill.className = "oc-state oc-state-" + ((finding && finding.state) || "unknown");
      pill.textContent = (finding && finding.state) || "unknown";
      body.appendChild(pill);
      if (finding && finding.message) {
        var note = document.createElement("p");
        note.className = "muted";
        note.textContent = finding.message;
        body.appendChild(note);
      }
      return body;
    }

    function list(body, items, label) {
      if (!items || !items.length) {
        var empty = document.createElement("p");
        empty.className = "muted";
        empty.textContent = "No " + label + " reported.";
        body.appendChild(empty);
        return;
      }
      var ul = document.createElement("ul");
      ul.className = "oc-inventory";
      items.forEach(function (item) {
        var li = document.createElement("li");
        var name = document.createElement("strong");
        name.textContent = item.name || item.id || "Unnamed";
        li.appendChild(name);
        var detail = [];
        if (item.providerID) detail.push(item.providerID + "/" + item.id);
        else if (item.id && item.name) detail.push(item.id);
        if (item.activation) detail.push(item.activation);
        if (typeof item.enabled === "boolean") detail.push(item.enabled ? "enabled" : "disabled");
        if (item.status) detail.push(item.status);
        if (item.sourceKind) detail.push(item.sourceKind);
        if (item.failure) detail.push(item.failure.replace(/_/g, " "));
        if (detail.length) {
          var meta = document.createElement("span");
          meta.textContent = detail.join(" · ");
          li.appendChild(meta);
        }
        ul.appendChild(li);
      });
      body.appendChild(ul);
    }

    function render(view) {
      summary.textContent = "";
      var state = document.createElement("span");
      state.className = "oc-state oc-state-" + view.state;
      state.textContent = view.state;
      summary.appendChild(state);
      summary.appendChild(document.createTextNode(view.version ? " OpenCode " + view.version : " OpenCode service"));

      var service = finding("service", view.service);
      if (view.version) service.appendChild(document.createTextNode("Identity: " + view.version + " via " + (view.identityVia || "service API") + "."));
      var project = finding("project", view.project);
      if (view.location) project.appendChild(document.createTextNode("Project " + view.location.name + " (" + (view.location.matchesRepository ? "matched" : "different location") + ")."));
      list(finding("providers", view.providers), view.providerList, "providers");
      var models = finding("models", view.models);
      if (view.defaultModel) {
        var def = document.createElement("p");
        def.textContent = "Default: " + view.defaultModel.providerID + "/" + view.defaultModel.id;
        models.appendChild(def);
      }
      list(models, view.modelList, "models");
      list(finding("plugins", view.plugins), view.pluginList, "plugins");
      var capabilities = finding("capabilities", view.capability);
      var capItems = Object.keys(view.capabilities || {}).map(function (key) {
        return { name: key.replace(/([A-Z])/g, " $1"), id: key, status: view.capabilities[key] ? "supported" : "unsupported" };
      });
      list(capabilities, capItems, "capabilities");
      message.hidden = true;
    }

    function load() {
      summary.textContent = "Loading service status...";
      return fetch("/api/opencode/status", { headers: { Accept: "application/json" }, cache: "no-store" })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(render)
        .catch(function () { summary.textContent = "Status unavailable."; message.textContent = "Could not load OpenCode status."; message.hidden = false; });
    }

    rediscover.addEventListener("click", function () {
      rediscover.disabled = true;
      message.textContent = "Rediscovering the registered service...";
      message.hidden = false;
      fetch("/api/opencode/rediscover", { method: "POST", headers: { Accept: "application/json", "X-Lessmess-UI": "1" }, cache: "no-store" })
        .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
        .then(render)
        .catch(function (err) { message.textContent = err.message; message.hidden = false; })
        .finally(function () { rediscover.disabled = false; });
    });
    load();
  }

  // --- OpenCode integrations -------------------------------------------------

  function initOpencodeIntegrations() {
    var root = document.getElementById("opencode-status-page");
    var listRoot = document.getElementById("oc-integrations-list");
    if (!root || !listRoot) return;
    var dialog = document.getElementById("oc-integration-dialog");
    var detailRoot = document.getElementById("oc-integration-detail");
    var title = document.getElementById("oc-integration-title");
    var message = document.getElementById("oc-integration-message");
    var pollTimer = null;
    var pollRequest = null;
    var activeAttempt = null;

    function api(path, options) {
      options = options || {};
      options.cache = "no-store";
      options.headers = Object.assign({ Accept: "application/json" }, options.headers || {});
      if (options.method && options.method !== "GET") {
        options.headers["Content-Type"] = "application/json";
        options.headers["X-Lessmess-UI"] = "1";
      }
      return fetch(path, options).then(function (r) {
        return r.json().catch(function () { return {}; }).then(function (body) {
          if (!r.ok) throw new Error(body.error || "The integration operation failed.");
          return body;
        });
      });
    }

    function setMessage(text) {
      message.textContent = text || "";
      message.hidden = !text;
    }

    function clearPoll() {
      if (pollTimer) window.clearTimeout(pollTimer);
      pollTimer = null;
      if (pollRequest) pollRequest.abort();
      pollRequest = null;
    }

    function clearSensitive(form) {
      Array.prototype.forEach.call(form.querySelectorAll("input, select, textarea"), function (input) {
        if (input.type === "checkbox") input.checked = false;
        else input.value = "";
      });
    }

    function sensitiveInput(name, label, required) {
      var wrap = document.createElement("label");
      wrap.textContent = label;
      var input = document.createElement("input");
      input.type = "password";
      input.name = name;
      input.required = !!required;
      input.autocomplete = "new-password";
      input.autocapitalize = "none";
      input.spellcheck = false;
      wrap.appendChild(input);
      return wrap;
    }

    function valuesFor(form, fields) {
      var answer = {};
      fields.forEach(function (field) {
        if (field.type === "external") return;
        var input = form.elements[field.key];
        if (!input) return;
        if (field.type === "boolean") answer[field.key] = input.checked;
        else if (field.type === "number" || field.type === "integer") answer[field.key] = input.value === "" ? null : Number(input.value);
        else if (field.type === "multiselect") answer[field.key] = input.value ? input.value.split(",").map(function (v) { return v.trim(); }).filter(Boolean) : [];
        else answer[field.key] = input.value;
      });
      return answer;
    }

    function appendFields(form, fields) {
      (fields || []).forEach(function (field) {
        if (field.type === "external") {
          if (!field.url) return;
          var external = document.createElement("a");
          external.href = field.url;
          external.target = "_blank";
          external.rel = "noopener noreferrer external";
          external.referrerPolicy = "no-referrer";
          external.textContent = field.title || "Open provider instructions";
          form.appendChild(external);
          return;
        }
        var label = field.title || field.key;
        if (field.type === "boolean") {
          var checkboxLabel = document.createElement("label");
          var checkbox = document.createElement("input");
          checkbox.type = "checkbox";
          checkbox.name = field.key;
          checkbox.autocomplete = "off";
          checkboxLabel.appendChild(checkbox);
          checkboxLabel.appendChild(document.createTextNode(" " + label));
          form.appendChild(checkboxLabel);
        } else {
          form.appendChild(sensitiveInput(field.key, label, field.required));
        }
        if (field.description) {
          var help = document.createElement("small");
          help.className = "muted";
          help.textContent = field.description;
          form.appendChild(help);
        }
      });
    }

    function attemptExpired(expires) {
      if (!expires) return false;
      return Date.now() >= (expires < 100000000000 ? expires * 1000 : expires);
    }

    function showAttempt(kind, integration, attempt) {
      clearPoll();
      activeAttempt = { kind: kind, integration: integration, attempt: attempt };
      detailRoot.textContent = "";
      var box = document.createElement("section");
      box.className = "oc-attempt";
      var heading = document.createElement("h3");
      heading.textContent = kind === "oauth" ? "OAuth connection" : "Command connection";
      box.appendChild(heading);
      if (kind === "oauth" && attempt.mode === "auto") {
        var limit = document.createElement("p");
        limit.textContent = "Automatic loopback completion depends on the provider and may complete on the device running OpenCode, not this phone. Keep this dialog open; use code completion only when the provider supplies a code.";
        box.appendChild(limit);
      }
      if (attempt.instructions) {
        var instructions = document.createElement("p");
        instructions.textContent = attempt.instructions;
        box.appendChild(instructions);
      }
      if (attempt.url) {
        var link = document.createElement("a");
        link.href = attempt.url;
        link.target = "_blank";
        link.rel = "noopener noreferrer external";
        link.referrerPolicy = "no-referrer";
        link.textContent = "Continue with provider";
        box.appendChild(link);
      }
      var status = document.createElement("p");
      status.className = "muted";
      status.textContent = "Waiting for OpenCode...";
      box.appendChild(status);
      if (kind === "oauth") {
        var codeForm = document.createElement("form");
        codeForm.className = "oc-sensitive-form";
        codeForm.autocomplete = "off";
        codeForm.appendChild(sensitiveInput("code", "Authorization code (when requested)", false));
        var complete = document.createElement("button");
        complete.type = "submit";
        complete.textContent = "Complete with code";
        codeForm.appendChild(complete);
        codeForm.addEventListener("submit", function (event) {
          event.preventDefault();
          var payload = JSON.stringify({ code: codeForm.elements.code.value, confirm: true, expectedStatus: "pending" });
          clearSensitive(codeForm);
          complete.disabled = true;
          api("/api/opencode/integrations/" + encodeURIComponent(integration.id) + "/attempts/oauth/" + encodeURIComponent(attempt.attemptID) + "/complete", { method: "POST", body: payload })
            .then(function () { status.textContent = "Connection complete."; activeAttempt = null; clearPoll(); })
            .catch(function (err) { setMessage(err.message); })
            .finally(function () { complete.disabled = false; });
        });
        box.appendChild(codeForm);
      }
      var cancel = document.createElement("button");
      cancel.type = "button";
      cancel.className = "btn-ghost";
      cancel.textContent = "Cancel attempt";
      cancel.addEventListener("click", function () {
        if (!window.confirm("Cancel this connection attempt?")) return;
        api("/api/opencode/integrations/" + encodeURIComponent(integration.id) + "/attempts/" + kind + "/" + encodeURIComponent(attempt.attemptID) + "/cancel", { method: "POST", body: '{"confirm":true,"expectedStatus":"pending"}' })
          .then(function () { activeAttempt = null; clearPoll(); status.textContent = "Attempt cancelled."; })
          .catch(function (err) { setMessage(err.message); });
      });
      box.appendChild(cancel);
      detailRoot.appendChild(box);

      function poll() {
        pollTimer = null;
        if (!dialog.open || document.hidden || attemptExpired(attempt.expires)) {
          if (attemptExpired(attempt.expires)) { status.textContent = "Attempt expired. Start a new connection."; activeAttempt = null; }
          return;
        }
        pollRequest = new AbortController();
        api("/api/opencode/integrations/" + encodeURIComponent(integration.id) + "/attempts/" + kind + "/" + encodeURIComponent(attempt.attemptID), { signal: pollRequest.signal })
          .then(function (view) {
            pollRequest = null;
            status.textContent = view.message || (view.status === "pending" ? "Waiting for OpenCode..." : "Connection " + view.status + ".");
            if (view.status === "pending" && !attemptExpired(view.expires)) pollTimer = window.setTimeout(poll, 1500);
            else { clearPoll(); activeAttempt = null; }
          })
          .catch(function (err) { pollRequest = null; if (err.name !== "AbortError") status.textContent = err.message; clearPoll(); });
      }
      pollTimer = window.setTimeout(poll, 500);
    }

    function renderIntegration(integration) {
      activeAttempt = null;
      title.textContent = integration.name || integration.id || "Integration";
      detailRoot.textContent = "";
      setMessage("");
      var credentials = (integration.connections || []).filter(function (connection) { return connection.type === "credential"; });
      (integration.methods || []).forEach(function (method) {
        var section = document.createElement("section");
        section.className = "oc-method";
        var heading = document.createElement("h3");
        heading.textContent = method.label || (method.type === "key" ? "API key" : method.type === "oauth" ? "OAuth" : method.type === "command" ? "Provider command" : "Environment");
        section.appendChild(heading);
        if (method.type === "unknown") {
          section.appendChild(document.createTextNode("This connection method is not supported by this lessmess version."));
        } else if (method.type === "env") {
          var remedy = document.createElement("p");
          remedy.textContent = "Set " + ((method.names || []).join(" or ") || "the required variable") + " in the OpenCode service environment, then restart that service. Environment connections cannot be removed here.";
          section.appendChild(remedy);
        } else {
          var form = document.createElement("form");
          form.className = "oc-sensitive-form";
          form.autocomplete = "off";
          appendFields(form, method.fields || []);
          if (method.type === "key") form.insertBefore(sensitiveInput("key", method.label || "API key", true), form.firstChild);
          var button = document.createElement("button");
          button.type = "submit";
          button.textContent = credentials.length && method.type === "key" ? "Reconnect" : "Connect";
          form.appendChild(button);
          form.addEventListener("submit", function (event) {
            event.preventDefault();
            var confirmation = credentials.length && method.type === "key"
              ? "Reconnect this integration with a new key? The old credential remains available until you remove it."
              : "Start this " + method.type + " connection? OpenCode may save provider credentials.";
            if (!window.confirm(confirmation)) return;
            var payload = { answer: valuesFor(form, method.fields || []), confirm: true };
            var path = "/api/opencode/integrations/" + encodeURIComponent(integration.id) + "/connect/" + method.type;
            if (method.type === "key") {
              payload.key = form.elements.key.value;
              if (credentials.length) {
                payload.confirmReplace = true;
                payload.expectedCredentialID = credentials[0].id;
                payload.expectedLabel = credentials[0].label || "";
              }
            } else payload.methodID = method.id;
            var body = JSON.stringify(payload);
            clearSensitive(form);
            button.disabled = true;
            api(path, { method: "POST", body: body })
              .then(function (result) {
                if (method.type === "key") return openIntegration(integration.id);
                showAttempt(method.type, integration, result);
              })
              .catch(function (err) { setMessage(err.message); })
              .finally(function () { button.disabled = false; });
          });
          section.appendChild(form);
        }
        detailRoot.appendChild(section);
      });
      (integration.connections || []).forEach(function (connection) {
        var section = document.createElement("section");
        section.className = "oc-connection";
        var heading = document.createElement("h3");
        heading.textContent = connection.type === "credential" ? (connection.label || "Saved credential") : connection.type === "env" ? "Service environment" : "Unknown connection";
        section.appendChild(heading);
        if (connection.type === "unknown") {
          section.appendChild(document.createTextNode("This connection type is not supported by this lessmess version."));
        } else if (connection.type !== "credential") {
          var env = document.createElement("p");
          env.textContent = "Provided by " + (connection.name || "the OpenCode service environment") + ". Change or remove it in the service environment and restart OpenCode.";
          section.appendChild(env);
        } else {
          var actions = document.createElement("div");
          actions.className = "oc-credential-actions";
          function actionButton(text, action, methodName, body) {
            var btn = document.createElement("button");
            btn.type = "button";
            btn.className = "btn-ghost";
            btn.textContent = text;
            btn.addEventListener("click", function () {
              var prompt = action === "" ? "Delete this saved credential?" : "Activate this saved credential?";
              if (!window.confirm(prompt)) return;
              btn.disabled = true;
              api("/api/opencode/integrations/" + encodeURIComponent(integration.id) + "/credentials/" + encodeURIComponent(connection.id) + action, { method: methodName, body: JSON.stringify(body()) })
                .then(function () { return openIntegration(integration.id); })
                .catch(function (err) { setMessage(err.message); })
                .finally(function () { btn.disabled = false; });
            });
            actions.appendChild(btn);
          }
          actionButton("Activate", "/activate", "POST", function () { return { confirm: true, expectedLabel: connection.label || "" }; });
          var labelForm = document.createElement("form");
          labelForm.className = "oc-sensitive-form";
          labelForm.autocomplete = "off";
          labelForm.appendChild(sensitiveInput("label", "New label", true));
          var rename = document.createElement("button");
          rename.type = "submit";
          rename.textContent = "Rename";
          labelForm.appendChild(rename);
          labelForm.addEventListener("submit", function (event) {
            event.preventDefault();
            if (!window.confirm("Rename this saved credential?")) return;
            var body = JSON.stringify({ label: labelForm.elements.label.value, expectedLabel: connection.label || "", confirm: true });
            clearSensitive(labelForm);
            api("/api/opencode/integrations/" + encodeURIComponent(integration.id) + "/credentials/" + encodeURIComponent(connection.id), { method: "PATCH", body: body })
              .then(function () { return openIntegration(integration.id); })
              .catch(function (err) { setMessage(err.message); });
          });
          section.appendChild(labelForm);
          actionButton("Delete", "", "DELETE", function () { return { confirm: true, expectedLabel: connection.label || "" }; });
          section.appendChild(actions);
        }
        detailRoot.appendChild(section);
      });
      if (!dialog.open) dialog.showModal();
    }

    function openIntegration(id) {
      clearPoll();
      activeAttempt = null;
      return api("/api/opencode/integrations/" + encodeURIComponent(id)).then(renderIntegration).catch(function (err) { setMessage(err.message); });
    }

    function load() {
      listRoot.textContent = "";
      listRoot.appendChild(document.createTextNode("Loading integrations..."));
      return api("/api/opencode/integrations").then(function (view) {
        listRoot.textContent = "";
        if (!view.available) {
          listRoot.appendChild(document.createTextNode("Integration management is unsupported by this OpenCode service."));
          return;
        }
        if (!view.integrations.length) {
          var empty = document.createElement("p");
          empty.className = "muted";
          empty.textContent = "No integrations reported by this OpenCode service.";
          listRoot.appendChild(empty);
          return;
        }
        view.integrations.forEach(function (integration) {
          var row = document.createElement("button");
          row.type = "button";
          row.className = "oc-integration-row";
          var name = document.createElement("strong");
          name.textContent = integration.name || integration.id || "Unnamed integration";
          var count = document.createElement("span");
          count.textContent = (integration.connections || []).length + " connection(s)";
          row.appendChild(name);
          row.appendChild(count);
          row.addEventListener("click", function () { openIntegration(integration.id); });
          listRoot.appendChild(row);
        });
      }).catch(function () {
        listRoot.textContent = "";
        var unavailable = document.createElement("p");
        unavailable.className = "settings-error";
        unavailable.textContent = "Could not load connections. Check Status and try again.";
        listRoot.appendChild(unavailable);
      });
    }

    document.getElementById("oc-integrations-refresh").addEventListener("click", load);
    document.addEventListener("tt:open-integration", function (event) {
      if (event.detail && event.detail.id) openIntegration(event.detail.id);
    });
    function clearDialog() { clearPoll(); activeAttempt = null; detailRoot.textContent = ""; title.textContent = "Integration"; setMessage(""); }
    document.getElementById("oc-integration-close").addEventListener("click", function () { dialog.close(); clearDialog(); });
    dialog.addEventListener("close", clearDialog);
    document.addEventListener("visibilitychange", function () {
      if (document.hidden) clearPoll();
      else if (dialog.open && activeAttempt) showAttempt(activeAttempt.kind, activeAttempt.integration, activeAttempt.attempt);
    });
    load();
  }

  // --- MCP and permissions --------------------------------------------------

  function managementAPI(path, options) {
    options = options || {};
    options.cache = "no-store";
    options.headers = Object.assign({ Accept: "application/json" }, options.headers || {});
    if (options.method && options.method !== "GET") {
      options.headers["Content-Type"] = "application/json";
      options.headers["X-Lessmess-UI"] = "1";
    }
    return fetch(path, options).then(function (response) {
      return response.json().catch(function () { return {}; }).then(function (body) {
        if (!response.ok) throw new Error(body.error || "OpenCode management operation failed.");
        return body;
      });
    });
  }

  function managementRow(title, detail) {
    var row = document.createElement("div");
    row.className = "oc-management-row";
    var copy = document.createElement("div");
    var heading = document.createElement("strong");
    heading.textContent = title;
    copy.appendChild(heading);
    if (detail) {
      var text = document.createElement("p");
      text.textContent = detail;
      copy.appendChild(text);
    }
    var actions = document.createElement("div");
    actions.className = "oc-management-actions";
    row.appendChild(copy);
    row.appendChild(actions);
    return { row: row, actions: actions };
  }

  function initOpencodeMCP() {
    var list = document.getElementById("oc-mcp-list");
    if (!list) return;
    var resources = document.getElementById("oc-mcp-resources");
    var message = document.getElementById("oc-mcp-message");

    function setMessage(text) { message.textContent = text || ""; message.hidden = !text; }
    function mutate(server, connect, button) {
      if (!connect && !window.confirm("Disconnect " + server.name + " from this OpenCode runtime?")) return;
      button.disabled = true;
      managementAPI("/api/opencode/mcp/" + encodeURIComponent(server.name) + "/" + (connect ? "connect" : "disconnect"), {
        method: "POST",
        body: JSON.stringify({ expectedStatus: server.status, confirm: !connect }),
      }).then(load).catch(function (err) { setMessage(err.message); }).finally(function () { button.disabled = false; });
    }
    function load() {
      setMessage("");
      list.textContent = "Loading MCP servers...";
      resources.textContent = "";
      return managementAPI("/api/opencode/mcp").then(function (view) {
        list.textContent = "";
        resources.textContent = "";
        if (!view.available) {
          list.appendChild(document.createTextNode("MCP status is unavailable on this OpenCode service."));
          return;
        }
        var notice = document.createElement("p");
        notice.className = "muted";
        notice.textContent = view.runtimeNotice;
        list.appendChild(notice);
        var rows = document.createElement("div");
        rows.className = "oc-management-list";
        (view.servers || []).forEach(function (server) {
          var detail = server.status.replace(/_/g, " ");
          if (server.failure) detail += " · " + server.failure.replace(/_/g, " ");
          var item = managementRow(server.name || "Unnamed MCP server", detail);
          if (server.status === "needs_auth") {
            if (server.authIntegrationID) {
              var auth = document.createElement("button");
              auth.type = "button";
              auth.className = "btn-ghost";
              auth.textContent = "Open integration";
              auth.addEventListener("click", function () { document.dispatchEvent(new CustomEvent("tt:open-integration", { detail: { id: server.authIntegrationID } })); });
              item.actions.appendChild(auth);
            } else {
              item.row.firstChild.appendChild(document.createElement("p")).textContent = "Authenticate in the OpenCode TUI, or run: opencode2 mcp auth <server-name>";
            }
          }
          if ((server.status === "disabled" || server.status === "failed") && view.connectAvailable) {
            var connect = document.createElement("button");
            connect.type = "button";
            connect.textContent = server.status === "failed" ? "Reconnect" : "Connect";
            connect.addEventListener("click", function () { mutate(server, true, connect); });
            item.actions.appendChild(connect);
          }
          if (server.status === "connected" && view.disconnectAvailable) {
            var disconnect = document.createElement("button");
            disconnect.type = "button";
            disconnect.className = "btn-ghost";
            disconnect.textContent = "Disconnect";
            disconnect.addEventListener("click", function () { mutate(server, false, disconnect); });
            item.actions.appendChild(disconnect);
          }
          rows.appendChild(item.row);
        });
        if (!(view.servers || []).length) rows.appendChild(document.createTextNode("No MCP servers are configured for this repository."));
        list.appendChild(rows);
        var allResources = (view.resources || []).concat(view.templates || []);
        var heading = document.createElement("h3");
        heading.className = "oc-management-subhead";
        heading.textContent = "Resources";
        resources.appendChild(heading);
        if (!view.resourcesAvailable) resources.appendChild(document.createTextNode("Resource discovery is unavailable."));
        else if (!allResources.length) resources.appendChild(document.createTextNode("No resources reported by connected servers."));
        else {
          var resourceRows = document.createElement("div");
          resourceRows.className = "oc-management-list";
          allResources.forEach(function (resource) {
            resourceRows.appendChild(managementRow(resource.name || "Unnamed resource", (resource.server || "unknown server") + " · " + (resource.uri || resource.uriTemplate || "URI redacted")).row);
          });
          resources.appendChild(resourceRows);
        }
      }).catch(function (err) { list.textContent = "MCP status unavailable."; resources.textContent = ""; setMessage(err.message); });
    }
    document.getElementById("oc-mcp-refresh").addEventListener("click", load);
    load();
  }

  function initOpencodePermissions() {
    var activeRoot = document.getElementById("oc-active-permissions");
    if (!activeRoot) return;
    var savedRoot = document.getElementById("oc-saved-permissions");
    var message = document.getElementById("oc-permissions-message");
    function setMessage(text) { message.textContent = text || ""; message.hidden = !text; }
    function section(root, title) {
      root.textContent = "";
      var heading = document.createElement("h3");
      heading.className = "oc-management-subhead";
      heading.textContent = title;
      root.appendChild(heading);
      var rows = document.createElement("div");
      rows.className = "oc-management-list";
      root.appendChild(rows);
      return rows;
    }
    function load() {
      setMessage("");
      activeRoot.textContent = "Loading active permissions...";
      savedRoot.textContent = "Loading saved permissions...";
      return managementAPI("/api/opencode/permissions").then(function (view) {
        var active = section(activeRoot, "Active requests");
        if (!view.activeAvailable) active.appendChild(document.createTextNode("Active permission discovery is unavailable."));
        else if (!view.active.length) active.appendChild(document.createTextNode("No active permission requests."));
        (view.active || []).forEach(function (request) {
          var item = managementRow(request.action || "Permission request", request.resourceCount + " resource(s) · " + request.sessionID);
          if (request.chat) {
            var chat = document.createElement("button");
            chat.type = "button";
            chat.textContent = "Open Chat";
            chat.addEventListener("click", function () { openChat(request.sessionID, request.sessionTitle || request.sessionID); });
            item.actions.appendChild(chat);
          }
          active.appendChild(item.row);
        });
        var saved = section(savedRoot, "Saved allow rules");
        if (!view.savedAvailable) saved.appendChild(document.createTextNode("Saved permission review is unavailable."));
        else if (!view.saved.length) saved.appendChild(document.createTextNode("No saved allow rules for this project."));
        (view.saved || []).forEach(function (rule) {
          var item = managementRow(rule.action || "allow", rule.resource || "all resources");
          if (view.removeAvailable) {
            var remove = document.createElement("button");
            remove.type = "button";
            remove.className = "btn-ghost";
            remove.textContent = "Remove";
            remove.addEventListener("click", function () {
              if (!window.confirm("Remove this saved allow rule? OpenCode may ask again next time.")) return;
              remove.disabled = true;
              managementAPI("/api/opencode/permissions/saved/" + encodeURIComponent(rule.id), {
                method: "DELETE",
                body: JSON.stringify({ confirm: true, expectedProject: rule.projectID, expectedAction: rule.action, expectedResource: rule.resource }),
              }).then(load).catch(function (err) { setMessage(err.message); }).finally(function () { remove.disabled = false; });
            });
            item.actions.appendChild(remove);
          }
          saved.appendChild(item.row);
        });
      }).catch(function (err) {
        activeRoot.textContent = "Active permission status unavailable.";
        savedRoot.textContent = "Saved permission status unavailable.";
        setMessage(err.message);
      });
    }
    document.getElementById("oc-permissions-refresh").addEventListener("click", load);
    load();
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
    initOpencodeStatus();
    initOpencodeIntegrations();
    initOpencodeMCP();
    initOpencodePermissions();
    initSetup();
    initOnboardingBanner();
    checkValidation();
    autoOpenSession();
    loadDiscussions();
  });
})();
