// Go methods bound in desktop.Run are exposed by the Wails runtime as
// window.go.<package>.<Struct>.<Method>() and return Promises.
const api = window.go.desktop.API;

const el = {
  setup: document.getElementById("setup"),
  setupNote: document.getElementById("setup-note"),
  setupError: document.getElementById("setup-error"),
  createDB: document.getElementById("create-db"),
  openDB: document.getElementById("open-db"),
  dashboard: document.getElementById("dashboard"),
  total: document.getElementById("total"),
  notes: document.getElementById("notes"),
  accounts: document.getElementById("accounts"),
  empty: document.getElementById("empty"),
  status: document.getElementById("status"),
  refresh: document.getElementById("refresh"),
};

// Mirrors renderMoneyWithCurrency in app/render.go: "-$ 12.34".
function formatMoney(currency, cents) {
  const sign = cents < 0 ? "-" : "";
  const amount = (Math.abs(cents) / 100).toFixed(2);

  return currency ? `${sign}${currency} ${amount}` : `${sign}${amount}`;
}

function signClass(cents) {
  if (cents > 0) return "positive";
  if (cents < 0) return "negative";
  return "";
}

function cell(text, className) {
  const td = document.createElement("td");
  td.textContent = text;
  if (className) td.className = className;
  return td;
}

function renderAccounts(balance) {
  el.accounts.replaceChildren();
  el.empty.hidden = balance.accounts.length > 0;

  for (const acct of balance.accounts) {
    const row = document.createElement("tr");
    if (acct.ignoreInSummaries) row.className = "ignored";

    let base = "—";
    if (acct.ignoreInSummaries) base = "ignored";
    else if (acct.hasRate) base = formatMoney(balance.baseCurrency, acct.baseCents);
    else base = "no rate";

    row.append(
      cell(acct.name),
      cell(formatMoney(acct.currency, acct.balanceCents), "num"),
      cell(base, "num"),
    );
    el.accounts.append(row);
  }
}

function renderNotes(balance) {
  const notes = [];
  if (balance.ignoredCount > 0) notes.push(`${balance.ignoredCount} account(s) ignored in summaries.`);
  if (balance.missingRates > 0) notes.push(`${balance.missingRates} account(s) excluded: missing conversion rate.`);
  el.notes.textContent = notes.join(" ");
}

let dbPath = "";

function showSetup(note) {
  el.setup.hidden = false;
  el.dashboard.hidden = true;
  el.refresh.hidden = true;
  el.setupNote.hidden = !note;
  el.setupNote.textContent = note;
  el.status.textContent = "No database configured yet";
}

function showDashboard(path) {
  dbPath = path;
  el.setup.hidden = true;
  el.dashboard.hidden = false;
  el.refresh.hidden = false;
  load();
}

// pick runs a setup dialog; the Go side resolves false when it was cancelled.
async function pick(choose) {
  el.createDB.disabled = true;
  el.openDB.disabled = true;
  el.setupError.hidden = true;

  try {
    if (await choose()) {
      const status = await api.Status();
      showDashboard(status.dbPath);
    }
  } catch (err) {
    el.setupError.textContent = String(err);
    el.setupError.hidden = false;
  } finally {
    el.createDB.disabled = false;
    el.openDB.disabled = false;
  }
}

async function load() {
  el.refresh.disabled = true;
  el.status.textContent = "Loading…";

  try {
    const balance = await api.Balance();

    el.total.textContent = formatMoney(balance.baseCurrency, balance.totalCents);
    el.total.className = `amount ${signClass(balance.totalCents)}`;
    renderNotes(balance);
    renderAccounts(balance);
    el.status.textContent = `${dbPath} · updated ${new Date(balance.loadedAt).toLocaleString()}`;
  } catch (err) {
    el.status.textContent = `Failed to load balance: ${err}`;
  } finally {
    el.refresh.disabled = false;
  }
}

async function start() {
  const status = await api.Status();
  if (status.ready) showDashboard(status.dbPath);
  else showSetup(status.note);
}

el.refresh.addEventListener("click", load);
el.createDB.addEventListener("click", () => pick(api.CreateDatabase));
el.openDB.addEventListener("click", () => pick(api.OpenDatabase));
start();
