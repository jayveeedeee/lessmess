import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("./static/app.js", import.meta.url), "utf8");
const functions = [
  "chatBusy", "chatExecutionState", "chatInboxFresh", "canResumeChatPending", "updateChatActionButton",
  "setChatHeaderState", "updateChatDeliveryControls", "renderChatControlsAvailability", "inboxTypeLabel",
  "chatPendingLabel", "chatInboxNotice", "renderChatInbox", "focusChatPending", "loadChatInbox",
  "refreshChatInbox", "mutateChatInbox", "lifecycleURL", "lifecycleResponse", "pollChat", "chatMutation", "loadChatLifecycle",
];
const client = functions.map((name) => {
  const start = source.indexOf("  function " + name + "(");
  assert.notEqual(start, -1, "missing production function " + name);
  const end = source.indexOf("\n  function ", start + 1);
  return source.slice(start, end);
}).join("\n");

class Element {
  constructor(tag = "div") {
    this.tagName = tag;
    this.children = [];
    this.dataset = {};
    this.attributes = {};
    this.className = "";
    this.value = "";
    this.textContent = "";
    this.style = { setProperty() {} };
    this.classList = {
      contains: (name) => this.className.split(" ").includes(name),
      toggle: (name, on) => {
        const classes = new Set(this.className.split(" ").filter(Boolean));
        if (on) classes.add(name); else classes.delete(name);
        this.className = [...classes].join(" ");
      },
      remove: (name) => this.classList.toggle(name, false),
    };
  }
  setAttribute(name, value) { this.attributes[name] = value; }
  removeAttribute(name) { delete this.attributes[name]; }
  appendChild(node) { this.children.push(node); node.parent = this; return node; }
  replaceChildren() { this.children = []; }
  remove() { this.parent.children = this.parent.children.filter((node) => node !== this); }
  focus() {}
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
  querySelectorAll(selector) {
    const nodes = this.children.flatMap((node) => [node, ...node.querySelectorAll("*")]);
    if (selector === "*") return nodes;
    if (selector === "#chat-pending-messages") return nodes.filter((node) => node.id === "chat-pending-messages");
    if (selector === "[data-inbox-id]") return nodes.filter((node) => node.dataset.inboxId);
    if (selector === ".chat-snapshot [data-message]") return this.markers || [];
    if (selector === ".chat-snapshot") return this.snapshot ? [this.snapshot] : [];
    return [];
  }
  set innerHTML(html) {
    this.children = [];
    if (!html.includes('class="chat-snapshot"')) return;
    this.snapshot = new Element();
    for (const [, name, value] of html.matchAll(/data-([a-z-]+)="([^"]*)"/g)) {
      this.snapshot.dataset[name.replace(/-([a-z])/g, (_, c) => c.toUpperCase())] = value;
    }
    this.markers = [...html.matchAll(/data-message="([^"]*)"/g)].map(([, id]) => ({ dataset: { message: id } }));
  }
}

const item = (id = "msg_pending") => ({ id, type: "user", known: true, delivery: "queue", text: "complete the tasks" });
const response = (items, status = 200) => ({ ok: status < 400, status, json: async () => status < 400 ? { items } : { error: "upstream unavailable" } });
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};
const flush = () => new Promise((resolve) => setImmediate(resolve));

function fixture() {
  const ids = Object.fromEntries([
    "chat-transcript", "chat-send-btn", "chat-prompt", "chat-action-pill", "chat-cancel-btn", "chat-steer-btn",
    "chat-queue-btn", "chat-resume-btn", "chat-copy-message-btn", "chat-fork-message-btn", "chat-revert-message-btn",
    "chat-session-state", "chat-file-input",
  ].map((id) => [id, new Element()]));
  const transcript = ids["chat-transcript"];
  transcript.scrollHeight = 1000;
  transcript.scrollTop = 500;
  transcript.clientHeight = 500;
  transcript.snapshot = new Element();
  transcript.snapshot.dataset = { busy: "false", busyKnown: "true", degraded: "false", outcome: "failed" };
  const document = {
    activeElement: null,
    getElementById: (id) => ids[id] || transcript.querySelector("#" + id),
    querySelector: (selector) => selector === "#chat-transcript .chat-snapshot" ? transcript.snapshot : null,
    querySelectorAll: () => [],
    createElement: (tag) => new Element(tag),
    contains: () => true,
  };
  const cstate = {
    session: "ses_queue", inbox: [item()], inboxState: "fresh", inboxLoaded: true, inboxCheckedAt: Date.now(),
    snapshotCheckedAt: Date.now(), snapshotFailed: false, inboxRequest: null, inboxPromise: null, request: null,
    mutation: false, selectedPendingID: "msg_pending", selectedMessageID: null,
    lifecycle: { busy: false, capabilities: { inboxList: true, inboxDelivery: true, inboxCancel: true } },
    compactPending: {},
  };
  const status = [], calls = [];
  const context = vm.createContext({
    cstate, document, BASE: "/p/project", AbortController, FormData, window: {},
    fetch: (url, options) => { calls.push({ url, options }); return context.reply(url, options); },
    reply: async () => response([item()]),
    setChatStatus: (text, error) => status.push({ text, error }),
    chatDraftFiles: () => [], chatDraftReferences: () => [], chatDraftSkills: () => [],
    selectedChatMessage: () => null, renderChatLifecycle() {}, captureChatForms() {}, settlePendingUserBoundary() {},
    mountChatForm() {}, restoreChatForms() {}, syncChatMessageSelection() {}, renderChatManagement() {}, loadChatNavigation() {},
    chatDisplayTitle: (title) => title, syncChatHeader() {},
    chatOpen: () => true, scheduleChatPoll() {}, refreshChat: () => { context.refreshes++; }, refreshes: 0,
  });
  vm.runInContext(client, context);
  return { context, state: cstate, ids, transcript, status, calls };
}

test("waiting, failed, Stop, idle, and unknown states never resume automatically", () => {
  const f = fixture();
  for (const [busy, known, outcome, expected] of [
    ["true", "true", "failed", "Queued · waiting for response"],
    ["false", "true", "failed", "Paused · response failed"],
    ["false", "true", "interrupted", "Paused · response stopped"],
    ["false", "true", "succeeded", "Pending · session idle"],
    ["false", "false", "failed", "Pending · execution state unavailable"],
  ]) {
    Object.assign(f.transcript.snapshot.dataset, { busy, busyKnown: known, outcome });
    assert.equal(f.context.chatPendingLabel(item()), expected);
    f.context.renderChatInbox();
    assert.equal(f.calls.length, 0);
  }
  f.transcript.snapshot.dataset.busyKnown = "true";
  f.state.inbox[0].delivery = "steer";
  f.transcript.snapshot.dataset.busy = "true";
  assert.equal(f.context.chatPendingLabel(f.state.inbox[0]), "Steering · awaiting delivery");
});

test("idle unchanged transcripts still refresh stale lifecycle metadata for automatic names", async () => {
  const f = fixture();
  const html = '<div class="chat-snapshot" data-busy="false" data-busy-known="true"></div>';
  f.state.snapshot = html;
  f.state.lifecycleCheckedAt = Date.now() - 6000;
  let metadataReads = 0;
  f.context.loadChatLifecycle = async () => { metadataReads++; };
  f.context.reply = async () => ({ ok: true, text: async () => html });
  f.context.pollChat(false);
  await flush();
  assert.equal(metadataReads, 1);
  assert.equal(f.state.snapshot, html);
  assert.equal(f.state.session, "ses_queue");
});

test("snapshot titles update an open chat even when lifecycle capabilities are unavailable", async () => {
  const f = fixture();
  f.context.reply = async () => ({ ok: true, text: async () => '<div class="chat-snapshot" data-title="Readable fallback title" data-busy="false" data-busy-known="true"></div>' });
  f.context.pollChat(false);
  await flush();
  assert.equal(f.state.title, "Readable fallback title");
  assert.equal(f.state.session, "ses_queue");
});

test("selected idle message offers Resume; busy message offers Stop and Send now", () => {
  const f = fixture();
  f.context.updateChatActionButton();
  assert.equal(f.ids["chat-send-btn"].dataset.action, "resume");
  assert.equal(f.ids["chat-send-btn"].type, "button");
  assert.equal(f.ids["chat-send-btn"].disabled, false);
  assert.equal(f.ids["chat-action-pill"].dataset.segments, "2");
  f.transcript.snapshot.dataset.busy = "true";
  f.context.updateChatActionButton();
  assert.equal(f.ids["chat-send-btn"].dataset.action, "stop");
  assert.equal(f.ids["chat-resume-btn"].disabled, false);
  assert.equal(f.ids["chat-action-pill"].dataset.segments, "3");
});

test("unknown, degraded, stale, expired, unsupported, and non-user items cannot resume", () => {
  for (const scenario of ["unknown", "degraded", "stale", "expired inbox", "expired snapshot", "unsupported", "non-user", "lifecycle unavailable"]) {
    const f = fixture();
    if (scenario === "unknown") f.transcript.snapshot.dataset.busyKnown = "false";
    if (scenario === "degraded") f.transcript.snapshot.dataset.degraded = "true";
    if (scenario === "stale") f.state.inboxState = "stale";
    if (scenario === "expired inbox") f.state.inboxCheckedAt -= 20000;
    if (scenario === "expired snapshot") f.state.snapshotCheckedAt -= 30000;
    if (scenario === "unsupported") f.state.lifecycle.capabilities.inboxDelivery = false;
    if (scenario === "non-user") f.state.inbox[0].type = "compaction";
    if (scenario === "lifecycle unavailable") f.state.lifecycleError = "offline";
    assert.equal(f.context.canResumeChatPending("msg_pending"), false, scenario);
    f.context.updateChatActionButton();
    assert.equal(f.ids["chat-send-btn"].disabled, true, scenario);
  }
});

test("failed inbox read retains rows and a warning that transcript success cannot erase", async () => {
  const f = fixture();
  f.context.reply = async () => response([], 503);
  await f.context.loadChatInbox(false);
  assert.equal(f.state.inboxState, "stale");
  assert.equal(f.state.inbox[0].id, "msg_pending");
  const notice = f.context.chatInboxNotice();
  assert.match(notice, /last-known.*unconfirmed/);
  f.context.setChatStatus("");
  f.context.renderChatInbox();
  assert.equal(f.context.chatInboxNotice(), notice);
  assert.equal(f.transcript.querySelector("#chat-pending-messages").children[0].textContent, notice);
  f.context.reply = async () => response([item()]);
  await f.context.loadChatInbox(false);
  assert.equal(f.state.inboxState, "fresh");
  assert.doesNotMatch(f.context.chatInboxNotice(), /last-known/);
});

test("initial inbox failures and stale empty lists still show uncertainty", async () => {
  const f = fixture();
  f.state.inbox = [];
  f.state.inboxLoaded = false;
  f.context.reply = async () => response([], 503);
  await f.context.loadChatInbox(false);
  assert.equal(f.state.inboxState, "unavailable");
  assert.match(f.context.chatInboxNotice(), /cannot be confirmed/);
  f.state.inboxLoaded = true;
  await f.context.loadChatInbox(false);
  assert.equal(f.state.inboxState, "stale");
  assert.ok(f.transcript.querySelector("#chat-pending-messages"));
});

test("delivery reconciles by message ID across a snapshot swap and compaction", () => {
  const f = fixture();
  f.transcript.markers = [{ dataset: { message: "msg_pending" } }, { dataset: { message: "msg_compaction" } }];
  f.context.renderChatInbox();
  assert.equal(f.state.selectedPendingID, null);
  assert.equal(f.transcript.querySelector("#chat-pending-messages"), null);
  f.transcript.markers = [{ dataset: { message: "msg_compaction" } }];
  f.context.renderChatInbox();
  assert.equal(f.transcript.querySelector("#chat-pending-messages").querySelectorAll("[data-inbox-id]").length, 1);
});

test("late inbox success cannot overwrite a newer refresh or another session", async () => {
  for (const switchSession of [false, true]) {
    const f = fixture();
    const old = deferred();
    f.context.reply = () => old.promise;
    const first = f.context.loadChatInbox(false);
    if (switchSession) f.state.session = "ses_other";
    f.context.reply = async () => response([item("msg_new")]);
    await f.context.refreshChatInbox(false);
    old.resolve(response([item("msg_old")]));
    await first;
    assert.equal(f.state.inbox[0].id, "msg_new");
    assert.equal(f.state.inboxState, "fresh");
  }
});

test("late inbox failure cannot stale a newer successful refresh", async () => {
  const f = fixture();
  const old = deferred();
  f.context.reply = () => old.promise;
  const first = f.context.loadChatInbox(false);
  f.context.reply = async () => response([item("msg_new")]);
  await f.context.refreshChatInbox(false);
  old.reject(new Error("old outage"));
  await first;
  assert.equal(f.state.inboxState, "fresh");
});

test("explicit Resume updates only the existing item and rejects duplicate clicks", async () => {
  const f = fixture();
  const patch = deferred();
  f.context.reply = (url, options) => options.method === "PATCH" ? patch.promise : Promise.resolve(response([item()]));
  const first = f.context.mutateChatInbox("msg_pending", f.ids["chat-send-btn"], "steer");
  await assert.rejects(f.context.mutateChatInbox("msg_pending", f.ids["chat-send-btn"], "steer"), /in progress/);
  await flush();
  const updates = f.calls.filter((call) => call.options.method === "PATCH");
  assert.equal(updates.length, 1);
  assert.equal(updates[0].url, "/p/project/api/sessions/ses_queue/inbox/msg_pending");
  assert.equal(updates[0].options.body, '{"delivery":"steer"}');
  assert.equal(f.calls.some((call) => call.url.includes("prompt") || call.url.includes("deliver")), false);
  patch.resolve(response([], 204));
  await first;
  assert.equal(f.state.mutation, false);
  assert.match(f.status.at(-1).text, /still awaiting delivery/);
});

test("a consumed item is not resubmitted when it disappears during preflight", async () => {
  const f = fixture();
  f.context.reply = async () => response([]);
  await f.context.mutateChatInbox("msg_pending", f.ids["chat-send-btn"], "steer");
  assert.equal(f.calls.filter((call) => call.options.method).length, 0);
  assert.match(f.status.at(-1).text, /no longer pending/);
});

test("a delivery-update disappearance race refreshes without duplicate submission", async () => {
  const f = fixture();
  let read = 0;
  f.context.reply = async (url, options) => options.method === "PATCH" ? response([], 409) : response(++read === 1 ? [item()] : []);
  await f.context.mutateChatInbox("msg_pending", f.ids["chat-send-btn"], "steer");
  assert.match(f.status.at(-1).text, /no longer pending/);
  assert.equal(f.calls.filter((call) => call.options.method === "PATCH").length, 1);
});

test("a lost delivery-update response stays conservative and never posts a prompt", async () => {
  const f = fixture();
  f.context.reply = async (url, options) => {
    if (options.method === "PATCH") throw new Error("connection lost");
    return response([item()]);
  };
  await f.context.mutateChatInbox("msg_pending", f.ids["chat-send-btn"], "steer");
  assert.match(f.status.at(-1).text, /connection lost/);
  assert.equal(f.calls.some((call) => call.options.method === "POST"), false);
});

test("accepted Resume with an unavailable confirmation retains stale rows without claiming delivery", async () => {
  const f = fixture();
  let accepted = false;
  f.context.reply = async (url, options) => {
    if (options.method === "PATCH") { accepted = true; return response([], 204); }
    return accepted ? response([], 503) : response([item()]);
  };
  await f.context.mutateChatInbox("msg_pending", f.ids["chat-send-btn"], "steer");
  assert.equal(f.state.inboxState, "stale");
  assert.equal(f.state.inbox.length, 1);
  assert.match(f.status.at(-1).text, /accepted.*could not be confirmed/);
  assert.equal(f.context.canResumeChatPending("msg_pending"), false);
});

test("switching sessions during Resume cannot refresh or unlock the new session", async () => {
  const f = fixture();
  const patch = deferred();
  f.context.reply = (url, options) => options.method === "PATCH" ? patch.promise : Promise.resolve(response([item()]));
  const first = f.context.mutateChatInbox("msg_pending", f.ids["chat-send-btn"], "steer");
  await flush();
  f.state.session = "ses_other";
  const nextOperation = {};
  f.state.mutation = nextOperation;
  patch.resolve(response([], 204));
  await first;
  assert.equal(f.state.mutation, nextOperation);
  assert.equal(f.context.refreshes, 0);
  assert.equal(f.calls.filter((call) => call.url.includes("ses_other")).length, 0);
});

test("cancellation confirms only that the item is no longer pending", async () => {
  const f = fixture();
  let cancelled = false;
  f.context.reply = async (url, options) => {
    if (options.method === "DELETE") { cancelled = true; return response([], 204); }
    return response(cancelled ? [] : [item()]);
  };
  await f.context.mutateChatInbox("msg_pending", f.ids["chat-cancel-btn"]);
  assert.equal(f.calls.filter((call) => call.options.method === "DELETE").length, 1);
  assert.match(f.status.at(-1).text, /may have been delivered or cancelled/);
});

test("Stop calls only interrupt and does not resume a queued follow-up", async () => {
  const f = fixture();
  f.transcript.snapshot.dataset.busy = "true";
  f.context.reply = async () => response([], 204);
  await f.context.chatMutation("interrupt", undefined, f.ids["chat-send-btn"]);
  assert.equal(f.calls.length, 1);
  assert.equal(f.calls[0].url, "/p/project/api/sessions/ses_queue/chat/interrupt");
  assert.equal(f.state.inbox[0].delivery, "queue");
});

test("transcript failures block recovery and an unchanged reconnect restores it", async () => {
  const f = fixture();
  const html = '<div class="chat-snapshot" data-busy="false" data-busy-known="true" data-degraded="false" data-outcome="failed"></div>';
  f.state.snapshot = html;
  f.context.reply = async () => { throw new Error("offline"); };
  f.context.pollChat(false);
  await flush();
  assert.equal(f.state.snapshotFailed, true);
  assert.equal(f.context.canResumeChatPending("msg_pending"), false);
  f.context.reply = async (url) => url.endsWith("/chat") ? { ok: true, text: async () => html } : response([item()]);
  f.context.pollChat(false);
  await flush();
  assert.equal(f.state.snapshotFailed, false);
  assert.equal(f.context.canResumeChatPending("msg_pending"), true);
  assert.equal(f.ids["chat-session-state"].textContent, "Failed");
  assert.equal(f.calls.some((call) => call.options.method === "PATCH"), false);
});

test("an obsolete transcript error cannot mark a newer view disconnected", async () => {
  const f = fixture();
  const old = deferred();
  f.context.reply = () => old.promise;
  f.context.pollChat(false);
  f.state.request = null;
  f.state.session = "ses_other";
  old.reject(new Error("old offline"));
  await flush();
  assert.equal(f.state.snapshotFailed, false);
  assert.equal(f.status.length, 0);
});

test("metadata reconnect refreshes capabilities without automatically resuming messages", async () => {
  const f = fixture();
  f.state.lifecycleError = "offline";
  const html = '<div class="chat-snapshot" data-busy="false" data-busy-known="true" data-degraded="false" data-outcome="failed"></div>';
  f.state.snapshot = html;
  f.context.reply = async (url) => {
    if (url.endsWith("/chat")) return { ok: true, text: async () => html };
    if (url.endsWith("/lifecycle")) return { ok: true, status: 200, json: async () => ({ busy: false, capabilities: { inboxList: true, inboxDelivery: true } }) };
    return response([item()]);
  };
  f.context.pollChat(false);
  await flush();
  assert.equal(f.state.lifecycleError, "");
  assert.equal(f.context.canResumeChatPending("msg_pending"), true);
  assert.equal(f.calls.some((call) => call.options.method === "PATCH" || call.options.method === "POST"), false);
});

test("inbox refresh uses current capabilities and disables unsupported recovery", async () => {
  const f = fixture();
  f.context.reply = async () => ({ ok: true, status: 200, json: async () => ({ items: [item()], capabilities: { inboxList: true, inboxDelivery: false } }) });
  await f.context.loadChatInbox(false);
  assert.equal(f.context.canResumeChatPending("msg_pending"), false);
  assert.match(f.context.chatInboxNotice(), /Resume is unavailable/);
});
