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
    if (e.key === "Escape") closeDetail();
  });

  document.addEventListener("htmx:afterSwap", function (e) {
    if (e.target && e.target.id === "detail") e.target.hidden = false;
  });

  // --- init -----------------------------------------------------------------

  document.addEventListener("DOMContentLoaded", function () {
    initTheme();
    initSortable();
    checkValidation();
  });
})();
