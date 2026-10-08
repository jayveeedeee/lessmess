import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("./static/app.js", import.meta.url), "utf8");
const names = ["refreshChats", "openChatPromotion", "submitChatPromotion", "closeChatPromotion", "syncChatPromotion", "followSession", "locationBaseTrail", "syncLocationFromChat", "lifecycleResponse"];
const client = names.map((name) => {
  const start = source.indexOf("  function " + name + "(");
  assert.notEqual(start, -1, name);
  return source.slice(start, source.indexOf("\n  function ", start + 1));
}).join("\n");

class Element {
  children = [];
  dataset = {};
  value = "";
  textContent = "";
  open = false;
  hidden = false;
  disabled = false;
  appendChild(child) { this.children.push(child); }
  replaceChildren() { this.children = []; }
  showModal() { this.open = true; }
  close() { this.open = false; }
  focus() { this.focused = true; }
}

function fixture() {
  const ids = Object.fromEntries(["chat-cards", "chats-empty", "chats-status", "chat-promote-btn", "chat-promote-dialog", "chat-promote-name", "chat-promote-prefix", "chat-promote-error", "chat-promote-confirm", "location-nav", "detail"].map((name) => [name, new Element()]));
  ids.detail.hidden = true;
  const calls = [], history = [], events = {};
  const state = { session: "ses_chat", title: "Improve chat navigation", drafts: { ses_chat: "keep my draft" }, lifecycle: { promotable: true, busy: false }, promotionConfirmation: null };
  const data = { promotable: true, busy: false, session: { title: "Improve chat navigation", updated: 10 } };
  const context = vm.createContext({
    cstate: state, BASE: "/p/demo", page: "chats", chatsRefreshPending: false, sessionTasksChange: null, activeChangeTitle: "",
    document: { getElementById: (id) => ids[id], createElement: () => new Element(), addEventListener: (name, cb) => { events[name] = cb; }, body: { getAttribute: () => "Demo" } },
    history: { replaceState: (_a, _b, url) => history.push(url) },
    fetch: (url, options) => { calls.push({ url, options }); return context.reply(url, options); },
    reply: async () => ({ ok: true, json: async () => ({ ok: true }) }),
    loadChatLifecycle: async () => data,
    closeChatMore() {}, setChatStatus() {}, refreshChat() {},
    activeSessionID: () => state.session, sessionOverlayOpen: () => true, chatOpen: () => true,
    loadSessionTasks: (change) => { context.sessionTasksChange = change; },
    compactChatUI: () => false, activeChatLocationView: () => "chat",
    setLocationTrail: (trail) => { context.trail = trail; },
  });
  vm.runInContext(client, context);
  return { context, ids, state, calls, history, events, data };
}

test("Chats cards use exact prefixed links and safe title text; offline refresh retains cards", async () => {
  const f = fixture();
  f.context.reply = async () => ({ ok: true, json: async () => ({ chats: [{ session: "ses_chat", title: "<script>not markup</script>", url: "/p/demo/chats?session=ses_chat", updated: "2026-10-08T12:00:00Z" }] }) });
  await f.context.refreshChats();
  assert.equal(f.calls[0].url, "/p/demo/api/chats");
  assert.equal(f.ids["chat-cards"].children[0].href, "/p/demo/chats?session=ses_chat");
  assert.equal(f.ids["chat-cards"].children[0].children[0].textContent, "<script>not markup</script>");
  f.context.reply = async () => ({ ok: false, status: 503, json: async () => ({ error: "offline" }) });
  await f.context.refreshChats();
  assert.equal(f.ids["chat-cards"].children.length, 1);
  assert.match(f.ids["chats-status"].textContent, /Previously loaded/);
  assert.equal(f.state.drafts.ses_chat, "keep my draft");
});

test("opening/cancelling promotion only confirms; busy or bound chats cannot open it", async () => {
  const f = fixture();
  await f.context.openChatPromotion();
  assert.equal(f.ids["chat-promote-dialog"].open, true);
  assert.equal(f.ids["chat-promote-name"].value, "Improve chat navigation");
  assert.equal(f.ids["chat-promote-prefix"].value, "ICN");
  assert.equal(f.ids["chat-promote-name"].focused, true);
  f.context.closeChatPromotion();
  assert.equal(f.state.promotionConfirmation, null);
  assert.equal(f.calls.length, 0);
  f.data.busy = true;
  await f.context.openChatPromotion();
  assert.equal(f.ids["chat-promote-dialog"].open, false);
  f.state.lifecycle.promotable = false;
  f.context.syncChatPromotion();
  assert.equal(f.ids["chat-promote-btn"].hidden, true);
});

test("promotion sends agreed fields and revision once, preserving the same session and draft", async () => {
  const f = fixture();
  await f.context.openChatPromotion();
  f.ids["chat-promote-name"].value = "First-class chats";
  f.ids["chat-promote-prefix"].value = "chat";
  let release;
  f.context.reply = () => new Promise((resolve) => { release = resolve; });
  const first = f.context.submitChatPromotion();
  await f.context.submitChatPromotion();
  assert.equal(f.calls.length, 1);
  assert.equal(f.calls[0].url, "/p/demo/api/sessions/ses_chat/promote");
  assert.deepEqual(JSON.parse(f.calls[0].options.body), { title: "First-class chats", prefix: "CHAT", confirmation: { sessionID: "ses_chat", updated: 10 } });
  release({ ok: true, json: async () => ({ ok: true }) });
  await first;
  assert.equal(f.ids["chat-promote-dialog"].open, false);
  assert.equal(f.state.session, "ses_chat");
  assert.equal(f.state.drafts.ses_chat, "keep my draft");
});

test("stale confirmation keeps chat and editable dialog available", async () => {
  const f = fixture();
  await f.context.openChatPromotion();
  f.context.reply = async () => ({ ok: false, status: 409, json: async () => ({ error: "session changed" }) });
  await f.context.submitChatPromotion();
  assert.equal(f.ids["chat-promote-dialog"].open, true);
  assert.equal(f.ids["chat-promote-confirm"].disabled, false);
  assert.match(f.ids["chat-promote-error"].textContent, /reopen/);
});

test("scaffold follow binds controls and updates URL in place without reopening or losing drafts", async () => {
  const f = fixture();
  f.context.reply = async (url) => ({ ok: true, json: async () => url.endsWith("/change") ? { change: "2026-10-08-new" } : { chats: [] } });
  await f.context.followSession();
  assert.equal(f.context.sessionTasksChange, "2026-10-08-new");
  assert.equal(f.history[0], "/p/demo/?change=2026-10-08-new&session=ses_chat");
  assert.equal(f.state.session, "ses_chat");
  assert.equal(f.state.drafts.ses_chat, "keep my draft");
  f.context.syncLocationFromChat();
  assert.equal(f.context.trail.at(-1).kind, "change");
  assert.equal(f.context.trail.some((crumb) => crumb.kind === "chats"), false);
});
