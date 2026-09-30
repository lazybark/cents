// App shell: database setup, the main menu and switching between views.
// Each view module exports a title and show(); init() is optional, and so
// is enter(), called when the view is switched to (not on a refresh).
import { api } from "./api.js";
import { showCurrencySetup } from "./currency-setup.js";
import { loadNotes } from "./notes.js";
import { busy, formError, setDBPath, setStatus, takeNotice } from "./ui.js";
import * as accounts from "./views/accounts.js";
import * as analytics from "./views/analytics.js";
import * as backup from "./views/backup.js";
import * as budgets from "./views/budgets.js";
import * as cashflow from "./views/cashflow.js";
import * as credits from "./views/credits.js";
import * as debts from "./views/debts.js";
import * as goals from "./views/goals.js";
import * as investments from "./views/investments.js";
import * as obligations from "./views/obligations.js";
import * as invoices from "./views/invoices.js";
import * as overview from "./views/overview.js";
import * as property from "./views/property.js";
import * as settings from "./views/settings.js";
import * as subscriptions from "./views/subscriptions.js";
import * as taxes from "./views/taxes.js";

const views = { overview, analytics, accounts, property, investments, cashflow, budgets, subscriptions, obligations, invoices, debts, credits, goals, taxes, backup, settings };
const DEFAULT_VIEW = "overview";

const $ = (id) => document.getElementById(id);

const el = {
  setup: $("setup"),
  setupNote: $("setup-note"),
  setupError: $("setup-error"),
  createDB: $("create-db"),
  openDB: $("open-db"),

  unlock: $("unlock"),
  unlockPath: $("unlock-path"),
  unlockForm: $("unlock-form"),
  unlockOther: $("unlock-other"),

  app: $("app"),
  viewTitle: $("view-title"),
  refresh: $("refresh"),
  // Toolbar buttons that belong to one view, named by data-for.
  viewActions: document.querySelectorAll(".toolbar [data-for]"),
  navItems: document.querySelectorAll(".nav-item"),
};

let current = DEFAULT_VIEW;

// --- setup -----------------------------------------------------------------

function showSetup(note) {
  el.setup.hidden = false;
  el.app.hidden = true;
  el.setupNote.hidden = !note;
  el.setupNote.textContent = note;
}

// showApp opens the app on the default view, resolving once it's shown.
function showApp(dbPath) {
  setDBPath(dbPath);
  el.setup.hidden = true;
  el.app.hidden = false;
  return navigate(DEFAULT_VIEW);
}

// showUnlock asks for the password of the encrypted database picked.
function showUnlock(status) {
  el.setup.hidden = true;
  el.app.hidden = true;
  el.unlock.hidden = false;
  el.unlockPath.textContent = status.lockedPath;
  el.unlockForm.reset();
  formError(el.unlockForm, "");
  el.unlockForm.elements.password.focus();
}

async function unlock(event) {
  event.preventDefault();
  try {
    await busy(el.unlockForm, () => api.Unlock(el.unlockForm.elements.password.value));
    el.unlock.hidden = true;
    openDatabase(await api.Status());
  } catch (err) {
    formError(el.unlockForm, String(err));
    el.unlockForm.elements.password.select();
  }
}

// chooseAnother gives up on the locked database: back to the one open, if
// any, or to choosing one.
async function chooseAnother() {
  await api.CancelUnlock();
  el.unlock.hidden = true;
  const status = await api.Status();
  if (status.ready) openDatabase(status);
  else showSetup(status.note);
}

// openDatabase shows the app, after picking currencies for a new database;
// an encrypted one asks for its password first.
function openDatabase(status) {
  if (status.locked) {
    showUnlock(status);
    return;
  }

  if (!status.needsCurrencies) {
    // Said after the first view loads, which sets the status itself.
    const notice = takeNotice();
    showApp(status.dbPath).then(() => notice && setStatus(notice));
    return;
  }

  el.setup.hidden = true;
  showCurrencySetup(async (rates) => {
    // Said after the first view loads, which sets the status itself.
    await showApp(status.dbPath);
    if (rates?.lastError) setStatus(`Currencies saved; rates couldn't be fetched yet and will be tried again: ${rates.lastError}`);
    else if (rates) setStatus(`Currencies saved with rates from ${rates.source}`);
  });
}

// pick runs a setup dialog; the Go side resolves false when it was cancelled.
async function pick(choose) {
  el.setupError.hidden = true;

  try {
    const chosen = await busy(el.setup, choose);
    if (chosen) openDatabase(await api.Status());
  } catch (err) {
    el.setupError.textContent = String(err);
    el.setupError.hidden = false;
  }
}

// --- navigation ------------------------------------------------------------

function navigate(name) {
  const previous = current;
  current = views[name] ? name : DEFAULT_VIEW;
  if (current !== previous) views[current].enter?.();

  for (const key of Object.keys(views)) {
    $(`view-${key}`).hidden = key !== current;
  }

  for (const item of el.navItems) {
    if (item.dataset.view === current) item.setAttribute("aria-current", "page");
    else item.removeAttribute("aria-current");
  }

  el.viewTitle.textContent = views[current].title;
  for (const action of el.viewActions) action.hidden = action.dataset.for !== current;
  return refresh();
}

async function refresh() {
  el.refresh.disabled = true;

  try {
    // Charts and month headings show month notes.
    await loadNotes();
    await views[current].show();
  } finally {
    el.refresh.disabled = false;
  }
}

// --- wiring ----------------------------------------------------------------

document.querySelectorAll("[data-close]").forEach((button) => {
  button.addEventListener("click", () => button.closest("dialog").close());
});

for (const item of el.navItems) {
  item.addEventListener("click", () => navigate(item.dataset.view));
}

for (const view of Object.values(views)) {
  view.init?.();
}

el.createDB.addEventListener("click", () => pick(api.CreateDatabase));
el.openDB.addEventListener("click", () => pick(api.OpenDatabase));
el.refresh.addEventListener("click", refresh);
el.unlockForm.addEventListener("submit", unlock);
el.unlockOther.addEventListener("click", chooseAnother);

async function start() {
  const status = await api.Status();
  if (status.ready || status.locked) openDatabase(status);
  else showSetup(status.note);
}

// Rates fetched in the background (once a day) change converted amounts, so
// the open view is shown again. window.runtime is Wails' event bridge.
// Another view asks to show a month of incomes and expenses (a payment
// marked paid, say).
window.addEventListener("cents:open-month", (event) => {
  cashflow.focusMonth(event.detail);
  navigate("cashflow");
});

// ⌘F / Ctrl+F: search incomes and expenses, from anywhere.
document.addEventListener("keydown", (event) => {
  if (event.key.toLowerCase() !== "f" || !(event.metaKey || event.ctrlKey) || event.shiftKey || event.altKey || el.app.hidden) return;
  if (document.querySelector("dialog[open]")) return;

  event.preventDefault();
  cashflow.openSearch();
  navigate("cashflow");
});

// A month note was saved: the open view shows it.
window.addEventListener("cents:notes-changed", () => refresh());

// Another view asks to show a list (a forecast item's, say).
window.addEventListener("cents:open-view", (event) => navigate(event.detail));

window.runtime?.EventsOn?.("rates-updated", () => {
  if (!el.app.hidden) refresh();
});

start();
