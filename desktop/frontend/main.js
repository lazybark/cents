// Go methods bound in desktop.Run are exposed by the Wails runtime as
// window.go.<package>.<Struct>.<Method>() and return Promises. A Go error
// rejects the Promise with its message.
const api = window.go.desktop.API;

const SORT_KEY = "cents.accountSort";

const $ = (id) => document.getElementById(id);

const el = {
  toolbar: $("toolbar"),
  refresh: $("refresh"),
  addAccount: $("add-account"),
  status: $("status"),

  setup: $("setup"),
  setupNote: $("setup-note"),
  setupError: $("setup-error"),
  createDB: $("create-db"),
  openDB: $("open-db"),

  dashboard: $("dashboard"),
  total: $("total"),
  notes: $("notes"),
  sort: $("sort"),
  baseHeading: $("base-heading"),
  accounts: $("accounts"),
  empty: $("empty"),
  accountsHint: $("accounts-hint"),

  addDialog: $("add-dialog"),
  addForm: $("add-form"),

  accountDialog: $("account-dialog"),
  accountTitle: $("account-title"),
  accountSubtitle: $("account-subtitle"),
  amountForm: $("amount-form"),
  logForm: $("log-form"),
  logs: $("logs"),
  logsEmpty: $("logs-empty"),
  deleteAccount: $("delete-account"),

  deleteDialog: $("delete-dialog"),
  deleteMessage: $("delete-message"),
  confirmDelete: $("confirm-delete"),
};

const state = {
  dbPath: "",
  sort: readSort(),
  overview: null,
  // The account open in the account dialog.
  current: null,
};

// --- formatting ------------------------------------------------------------

// Mirrors renderMoneyWithCurrency in app/render.go: "-$ 12.34".
function formatMoney(currency, cents) {
  const sign = cents < 0 ? "-" : "";
  const amount = formatAmount(Math.abs(cents));

  return currency ? `${sign}${currency} ${amount}` : `${sign}${amount}`;
}

// Mirrors formatAmount in app/format.go, for prefilling inputs.
function formatAmount(cents) {
  return (cents / 100).toFixed(2);
}

// Mirrors formatUpdatedAt in app/format.go: accounts never updated carry a
// 1970 placeholder date.
function formatUpdatedAt(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 1971) return "—";

  return `${localDate(date)} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function localDate(date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function pad(value) {
  return String(value).padStart(2, "0");
}

function signClass(cents) {
  if (cents > 0) return "positive";
  if (cents < 0) return "negative";
  return "";
}

// --- small helpers ---------------------------------------------------------

function readSort() {
  try {
    const value = Number.parseInt(localStorage.getItem(SORT_KEY) ?? "", 10);
    return Number.isNaN(value) ? 0 : value;
  } catch {
    return 0;
  }
}

function writeSort(value) {
  try {
    localStorage.setItem(SORT_KEY, String(value));
  } catch {
    // Storage can be unavailable; sorting just won't be remembered.
  }
}

function setStatus(message) {
  el.status.textContent = state.dbPath ? `${state.dbPath} · ${message}` : message;
}

function cell(content, className) {
  const td = document.createElement("td");
  if (content instanceof Node) td.append(content);
  else td.textContent = content;
  if (className) td.className = className;
  return td;
}

function formError(form, message) {
  const error = form.querySelector(".form-error");
  error.textContent = message ?? "";
  error.hidden = !message;
}

// busy disables a form's buttons while an API call is in flight.
async function busy(container, work) {
  const buttons = container.querySelectorAll("button");
  buttons.forEach((button) => (button.disabled = true));

  try {
    return await work();
  } finally {
    buttons.forEach((button) => (button.disabled = false));
  }
}

// --- setup -----------------------------------------------------------------

function showSetup(note) {
  el.setup.hidden = false;
  el.dashboard.hidden = true;
  el.toolbar.hidden = true;
  el.setupNote.hidden = !note;
  el.setupNote.textContent = note;
  el.status.textContent = "No database configured yet";
}

function showDashboard(path) {
  state.dbPath = path;
  el.setup.hidden = true;
  el.dashboard.hidden = false;
  el.toolbar.hidden = false;
  load();
}

// pick runs a setup dialog; the Go side resolves false when it was cancelled.
async function pick(choose) {
  el.setupError.hidden = true;

  try {
    const chosen = await busy(el.setup, choose);
    if (chosen) {
      const status = await api.Status();
      showDashboard(status.dbPath);
    }
  } catch (err) {
    el.setupError.textContent = String(err);
    el.setupError.hidden = false;
  }
}

// --- accounts list ---------------------------------------------------------

async function load(message) {
  el.refresh.disabled = true;

  try {
    const overview = await api.Accounts(state.sort);
    state.overview = overview;
    state.sort = overview.sort;

    renderTotal(overview);
    renderSortOptions(overview);
    renderAccounts(overview);
    setStatus(message ?? `updated ${new Date(overview.loadedAt).toLocaleString()}`);
  } catch (err) {
    setStatus(`Failed to load accounts: ${err}`);
  } finally {
    el.refresh.disabled = false;
  }
}

function renderTotal(overview) {
  el.total.textContent = formatMoney(overview.baseCurrency, overview.totalCents);
  el.total.className = `amount ${signClass(overview.totalCents)}`;

  const notes = [];
  if (overview.ignoredCount > 0) notes.push(`${overview.ignoredCount} account(s) ignored in summaries.`);
  if (overview.missingRates > 0) notes.push(`${overview.missingRates} account(s) excluded: missing conversion rate.`);
  el.notes.textContent = notes.join(" ");
}

function renderSortOptions(overview) {
  if (el.sort.options.length !== overview.sortOptions.length) {
    el.sort.replaceChildren(...overview.sortOptions.map((label, index) => new Option(label, String(index))));
  }

  el.sort.value = String(overview.sort);
}

function renderAccounts(overview) {
  el.baseHeading.textContent = `In ${overview.baseCurrency}`;
  el.accounts.replaceChildren();
  el.empty.hidden = overview.accounts.length > 0;
  el.accountsHint.hidden = overview.accounts.length === 0;

  for (const acct of overview.accounts) {
    const row = document.createElement("tr");
    row.tabIndex = 0;
    if (acct.ignoreInSummaries) row.className = "ignored";

    const name = document.createElement("div");
    name.className = "account-name";
    name.textContent = acct.name;
    if (acct.ignoreInSummaries) {
      const badge = document.createElement("span");
      badge.className = "badge";
      badge.textContent = "ignored";
      name.append(badge);
    }

    const description = document.createElement("div");
    description.className = "account-description";
    description.textContent = acct.description;

    const nameCell = document.createElement("div");
    nameCell.append(name, description);

    let base = "no rate";
    if (acct.hasRate) base = formatMoney(overview.baseCurrency, acct.baseCents);

    row.append(
      cell(nameCell),
      cell(acct.currency),
      cell(formatMoney(acct.currency, acct.balanceCents), "num"),
      cell(base, "num"),
      cell(formatUpdatedAt(acct.lastUpdatedAt)),
    );

    row.addEventListener("click", () => openAccount(acct));
    row.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        openAccount(acct);
      }
    });

    el.accounts.append(row);
  }
}

// --- add account -----------------------------------------------------------

function openAddAccount() {
  const form = el.addForm;
  const currencies = state.overview?.currencies ?? [];

  form.reset();
  form.elements.currency.replaceChildren(...currencies.map((name) => new Option(name, name)));
  formError(form, "");
  el.addDialog.showModal();
}

async function saveNewAccount(event) {
  event.preventDefault();
  const form = el.addForm;
  const input = {
    name: form.elements.name.value,
    description: form.elements.description.value,
    currency: form.elements.currency.value,
    amount: form.elements.amount.value,
    ignoreInSummaries: form.elements.ignoreInSummaries.checked,
  };

  try {
    await busy(form, () => api.CreateAccount(input));
    el.addDialog.close();
    await load(`saved account ${input.name.trim()}`);
  } catch (err) {
    formError(form, String(err));
  }
}

// --- account dialog --------------------------------------------------------

function openAccount(acct) {
  state.current = acct;

  el.accountTitle.textContent = acct.name;
  el.accountSubtitle.textContent = [acct.currency, formatMoney(acct.currency, acct.balanceCents), acct.description]
    .filter(Boolean)
    .join(" · ");

  const amount = el.amountForm.elements;
  amount.amount.value = formatAmount(acct.balanceCents);
  amount.updateLog.checked = false;
  amount.ignoreInSummaries.checked = acct.ignoreInSummaries;
  formError(el.amountForm, "");

  const log = el.logForm.elements;
  log.date.value = localDate(new Date());
  log.value.value = formatAmount(acct.balanceCents);
  formError(el.logForm, "");

  el.logs.replaceChildren();
  el.logsEmpty.hidden = true;
  el.accountDialog.showModal();
  loadLogs();
}

async function loadLogs() {
  const acct = state.current;

  try {
    const logs = await api.AccountValueLogs(acct.id);
    el.logs.replaceChildren(
      ...logs.map((entry) => {
        const row = document.createElement("tr");
        row.append(cell(entry.date), cell(formatMoney(acct.currency, entry.valueCents), "num"));
        return row;
      }),
    );
    el.logsEmpty.hidden = logs.length > 0;
  } catch (err) {
    formError(el.logForm, `Failed to load value history: ${err}`);
  }
}

async function saveAmount(event) {
  event.preventDefault();
  const acct = state.current;
  const form = el.amountForm.elements;
  const input = {
    id: acct.id,
    amount: form.amount.value,
    ignoreInSummaries: form.ignoreInSummaries.checked,
    updateLog: form.updateLog.checked,
  };

  try {
    const result = await busy(el.amountForm, () => api.UpdateAccountAmount(input));
    el.accountDialog.close();

    const warning = result.warning ? ` (${result.warning})` : "";
    await load(`updated amount for ${acct.name}${warning}`);
  } catch (err) {
    formError(el.amountForm, String(err));
  }
}

async function saveLog(event) {
  event.preventDefault();
  const acct = state.current;
  const form = el.logForm.elements;
  const input = { accountId: acct.id, date: form.date.value, value: form.value.value };

  try {
    await busy(el.logForm, () => api.SaveAccountValueLog(input));
    formError(el.logForm, "");
    await loadLogs();
    setStatus(`saved log value for ${input.date}`);
  } catch (err) {
    formError(el.logForm, String(err));
  }
}

// --- delete ----------------------------------------------------------------

function askDelete() {
  el.deleteMessage.textContent = `“${state.current.name}” and its value history will be deleted. This can't be undone.`;
  formError(el.deleteDialog, "");
  el.deleteDialog.showModal();
}

async function confirmDelete() {
  const acct = state.current;

  try {
    await busy(el.deleteDialog, () => api.DeleteAccount(acct.id));
    el.deleteDialog.close();
    el.accountDialog.close();
    await load(`deleted account ${acct.name}`);
  } catch (err) {
    formError(el.deleteDialog, String(err));
  }
}

// --- wiring ----------------------------------------------------------------

async function start() {
  const status = await api.Status();
  if (status.ready) showDashboard(status.dbPath);
  else showSetup(status.note);
}

document.querySelectorAll("[data-close]").forEach((button) => {
  button.addEventListener("click", () => button.closest("dialog").close());
});

el.createDB.addEventListener("click", () => pick(api.CreateDatabase));
el.openDB.addEventListener("click", () => pick(api.OpenDatabase));
el.refresh.addEventListener("click", () => load());
el.addAccount.addEventListener("click", openAddAccount);
el.sort.addEventListener("change", () => {
  state.sort = Number(el.sort.value);
  writeSort(state.sort);
  load();
});
el.addForm.addEventListener("submit", saveNewAccount);
el.amountForm.addEventListener("submit", saveAmount);
el.logForm.addEventListener("submit", saveLog);
el.deleteAccount.addEventListener("click", askDelete);
el.confirmDelete.addEventListener("click", confirmDelete);

start();
