// Accounts: totals, the account table and every account operation the TUI
// offers (add, update amount, value history, delete).
import { api } from "../api.js";
import {
  busy,
  cell,
  formatAmount,
  formatMoney,
  formatUpdatedAt,
  formError,
  loadedStatus,
  localDate,
  setStatus,
  signClass,
} from "../ui.js";

const SORT_KEY = "cents.accountSort";

const $ = (id) => document.getElementById(id);

const el = {
  addAccount: $("add-account"),
  total: $("total"),
  currencyTotals: $("currency-totals"),
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
  sort: readSort(),
  overview: null,
  // The account open in the account dialog.
  current: null,
};

export const title = "Accounts";

export function init() {
  el.addAccount.addEventListener("click", openAddAccount);
  el.sort.addEventListener("change", () => {
    state.sort = Number(el.sort.value);
    writeSort(state.sort);
    show();
  });
  el.addForm.addEventListener("submit", saveNewAccount);
  el.amountForm.addEventListener("submit", saveAmount);
  el.logForm.addEventListener("submit", saveLog);
  el.deleteAccount.addEventListener("click", askDelete);
  el.confirmDelete.addEventListener("click", confirmDelete);
}

// show reloads the accounts; message replaces the default status line.
export async function show(message) {
  try {
    const overview = await api.Accounts(state.sort);
    state.overview = overview;
    state.sort = overview.sort;

    renderTotal(overview);
    renderSortOptions(overview);
    renderAccounts(overview);

    if (message) setStatus(message);
    else loadedStatus(overview.loadedAt);
  } catch (err) {
    setStatus(`Failed to load accounts: ${err}`);
  }
}

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

// --- list ------------------------------------------------------------------

function renderTotal(overview) {
  el.total.textContent = formatMoney(overview.baseCurrency, overview.totalCents);
  el.total.className = `amount ${signClass(overview.totalCents)}`;
  renderCurrencyTotals(overview);

  el.notes.textContent = overview.ignoredCount > 0 ? `${overview.ignoredCount} account(s) ignored in summaries.` : "";
}

// Non-base currencies, each in its own currency with the base equivalent.
function renderCurrencyTotals(overview) {
  const totals = overview.currencyTotals ?? [];
  el.currencyTotals.hidden = totals.length === 0;
  el.currencyTotals.replaceChildren(
    ...totals.map((total) => {
      const item = document.createElement("li");

      const amount = document.createElement("span");
      amount.className = "currency-amount";
      amount.textContent = formatMoney(total.currency, total.cents);

      const base = document.createElement("span");
      base.className = total.hasRate ? "muted" : "negative";
      base.textContent = total.hasRate ? `≈ ${formatMoney(overview.baseCurrency, total.baseCents)}` : "no rate, not in total";

      item.append(amount, base);
      return item;
    }),
  );
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

    const base = acct.hasRate ? formatMoney(overview.baseCurrency, acct.baseCents) : "no rate";

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
    await show(`saved account ${input.name.trim()}`);
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
    await show(`updated amount for ${acct.name}${warning}`);
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
    await show(`deleted account ${acct.name}`);
  } catch (err) {
    formError(el.deleteDialog, String(err));
  }
}
