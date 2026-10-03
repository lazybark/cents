// App shell: database setup, the main menu and switching between views.
// Each view module exports a title and show(); init() is optional.
import { api } from "./api.js";
import { showCurrencySetup } from "./currency-setup.js";
import { busy, setDBPath, setStatus } from "./ui.js";
import * as accounts from "./views/accounts.js";
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

const views = { overview, accounts, property, investments, cashflow, subscriptions, obligations, invoices, debts, credits, goals, taxes, settings };
const DEFAULT_VIEW = "overview";

const $ = (id) => document.getElementById(id);

const el = {
  setup: $("setup"),
  setupNote: $("setup-note"),
  setupError: $("setup-error"),
  createDB: $("create-db"),
  openDB: $("open-db"),

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

// openDatabase shows the app, after picking currencies for a new database.
function openDatabase(status) {
  if (!status.needsCurrencies) {
    showApp(status.dbPath);
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
  current = views[name] ? name : DEFAULT_VIEW;

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

async function start() {
  const status = await api.Status();
  if (status.ready) openDatabase(status);
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

window.runtime?.EventsOn?.("rates-updated", () => {
  if (!el.app.hidden) refresh();
});

start();
