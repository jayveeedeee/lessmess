// tasktracker board UI glue: SortableJS drag-and-drop, SSE live refresh,
// htmx form follow-ups, validation banner.
(function () {
  "use strict";

  var page = document.body.getAttribute("data-page");

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
    if (e.target && e.target.id === "board") initSortable();
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
  });

  // --- SSE live updates ----------------------------------------------------

  if (typeof EventSource !== "undefined") {
    var es = new EventSource("/events");
    var timer = null;
    es.addEventListener("fs", scheduleRefresh);
    es.addEventListener("write", scheduleRefresh);
    function scheduleRefresh() {
      clearTimeout(timer);
      timer = setTimeout(function () {
        checkValidation();
        if (page === "board" && boardEl()) refreshBoard();
        else if (page === "index") location.reload();
      }, 250);
    }
  }

  // --- validation banner ---------------------------------------------------

  function checkValidation() {
    fetch("/api/validate", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) {
        var banner = document.getElementById("banner");
        if (!banner) return;
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
      })
      .catch(function () {});
  }

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

  // --- sessions panel + terminal ----------------------------------------------

  function changeID() {
    var b = boardEl();
    return b ? b.getAttribute("data-change") : null;
  }

  function loadSessions() {
    fetch("/changes/" + changeID() + "/sessions", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (j) {
        var ul = document.getElementById("sessions-list");
        if (!ul) return;
        ul.innerHTML = "";
        var sessions = j.sessions || [];
        if (!sessions.length) {
          var empty = document.createElement("li");
          empty.className = "session-empty";
          empty.textContent = "No sessions yet — start one.";
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
      })
      .catch(function () {});
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
          .then(function (s) { loadSessions(); openTerminal(s.session, s.title); })
          .catch(function (e) { alert("Create session failed: " + e.message); });
      });
    }
  }

  var tstate = { term: null, ws: null, ro: null };

  function openTerminal(sessionID, title) {
    closeTerminal();
    var overlay = document.getElementById("terminal-overlay");
    overlay.hidden = false;
    document.getElementById("terminal-session").textContent = sessionID;
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

    tstate = { term: term, ws: ws, ro: ro };
  }

  function closeTerminal() {
    if (tstate.ro) tstate.ro.disconnect();
    if (tstate.ws && tstate.ws.readyState <= 1) tstate.ws.close();
    if (tstate.term) tstate.term.dispose();
    tstate = { term: null, ws: null, ro: null };
    var overlay = document.getElementById("terminal-overlay");
    if (overlay) overlay.hidden = true;
  }

  function terminalOpen() {
    var ov = document.getElementById("terminal-overlay");
    return ov && !ov.hidden;
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
        fetch("/changes/" + changeID() + "/commit", {
          method: "POST",
          headers: { Accept: "application/json" },
        })
          .then(function (r) { return r.json().then(function (j) { if (!r.ok) throw new Error(j.error || r.statusText); return j; }); })
          .then(function (j) { openTerminal(j.session, changeID() + " — git commit"); })
          .catch(function (e) { alert("Commit failed: " + e.message); });
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
    if (terminalOpen()) { closeTerminal(); return; }
    closeDetail();
  });

  document.addEventListener("htmx:afterSwap", function (e) {
    if (e.target && e.target.id === "detail") e.target.hidden = false;
  });

  // --- init -----------------------------------------------------------------

  document.addEventListener("DOMContentLoaded", function () {
    initTheme();
    initSortable();
    initSessions();
    initLifecycle();
    checkValidation();
    autoOpenSession();
  });
})();
