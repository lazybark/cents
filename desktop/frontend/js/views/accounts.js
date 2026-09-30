// Accounts: totals, the account table and every account operation the TUI
// offers (add, update amount, value history, delete).
import { api } from "../api.js";
import { chart, monthLabel, monthRange } from "../charts.js";
import { openMerge } from "../merge.js";
import {
  badge,
  busy,
  cell,
  clickableRow,
  confirmDelete,
  fillSelect,
  formatAmount,
  formatMoney,
  formatUpdatedAt,
  formError,
  loadedStatus,
  localDate,
  rowAction,
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
  chart: $("account-chart"),
  chartStats: $("account-chart-stats"),
  chartEmpty: $("account-chart-empty"),
  deleteAccount: $("delete-account"),
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
    if (acct.ignoreInSummaries || acct.archived) row.className = "ignored";

    const name = document.createElement("div");
    name.className = "account-name";
    name.textContent = acct.name;
    if (acct.ignoreInSummaries) name.append(badge("ignored"));
    if (acct.archived) name.append(badge("archived"));

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

    clickableRow(row, () => openAccount(acct));
    el.accounts.append(row);
  }
}

// --- add account -----------------------------------------------------------

function openAddAccount() {
  const form = el.addForm;
  const currencies = state.overview?.currencies ?? [];

  form.reset();
  fillSelect(form.elements.currency, currencies);
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
  amount.archived.checked = acct.archived;
  formError(el.amountForm, "");

  const log = el.logForm.elements;
  log.date.value = localDate(new Date());
  log.value.value = formatAmount(acct.balanceCents);
  formError(el.logForm, "");

  el.logs.replaceChildren();
  el.logsEmpty.hidden = true;
  el.chart.replaceChildren();
  el.chartStats.replaceChildren();
  el.chartEmpty.hidden = true;
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
        row.append(
          cell(entry.date),
          cell(formatMoney(acct.currency, entry.valueCents), "num"),
          cell(rowAction("Delete", () => askDeleteLog(entry)), "num"),
        );
        return row;
      }),
    );
    el.logsEmpty.hidden = logs.length > 0;
  } catch (err) {
    formError(el.logForm, `Failed to load value history: ${err}`);
  }

  await loadChart();
}

// loadChart draws the account's value month by month from its history: each
// month's last logged value. Months with nothing logged stay empty.
async function loadChart() {
  const acct = state.current;
  let points = [];

  try {
    points = await api.AccountMonthlyValues(acct.id);
  } catch (err) {
    formError(el.logForm, `Failed to load the chart: ${err}`);
  }

  const enough = points.length >= 2;
  el.chartEmpty.hidden = enough;
  el.chartStats.hidden = !enough;
  if (!enough) {
    el.chart.replaceChildren();
    return;
  }

  const money = (cents) => formatMoney(acct.currency, cents);
  const signed = (cents) => (cents > 0 ? "+" : "") + money(cents);
  const byMonth = new Map(points.map((p) => [p.month, p]));
  const months = monthRange(points[0].month, points[points.length - 1].month);

  chart(el.chart, {
    months,
    series: [{ type: "line", values: months.map((m) => byMonth.get(m)?.valueCents ?? null), className: "line-value" }],
    zero: false,
    tooltip: (i) => {
      const point = byMonth.get(months[i]);
      if (!point) return [monthLabel(months[i], true), "nothing logged"];

      const index = points.indexOf(point);
      const lines = [monthLabel(point.month, true), `${money(point.valueCents)} on ${point.date}`];
      if (index > 0) lines.push(`${signed(point.valueCents - points[index - 1].valueCents)} since ${monthLabel(points[index - 1].month)}`);
      return lines;
    },
    label: `${acct.name} month by month`,
  });

  const first = points[0];
  const last = points[points.length - 1];
  const change = last.valueCents - first.valueCents;
  const percent = first.valueCents !== 0 ? ` (${change > 0 ? "+" : ""}${((change / Math.abs(first.valueCents)) * 100).toFixed(1)}%)` : "";
  const high = points.reduce((a, b) => (b.valueCents > a.valueCents ? b : a));
  const low = points.reduce((a, b) => (b.valueCents < a.valueCents ? b : a));
  const [year, month] = last.month.split("-");
  const yearAgo = byMonth.get(`${Number(year) - 1}-${month}`);
  const stats = [
    [`Change since ${monthLabel(first.month)}`, `${signed(change)}${percent}`, signClass(change)],
    ["Highest", `${money(high.valueCents)} · ${monthLabel(high.month)}`],
    ["Lowest", `${money(low.valueCents)} · ${monthLabel(low.month)}`],
    ["Average change a month", signed(Math.round(change / (months.length - 1)))],
  ];
  if (yearAgo) stats.splice(1, 0, ["Change over 12 months", signed(last.valueCents - yearAgo.valueCents), signClass(last.valueCents - yearAgo.valueCents)]);

  el.chartStats.replaceChildren(
    ...stats.flatMap(([term, value, className]) => {
      const dt = document.createElement("dt");
      dt.textContent = term;
      const dd = document.createElement("dd");
      dd.textContent = value;
      if (className) dd.className = className;
      return [dt, dd];
    }),
  );
}

// History entries are snapshots, so deleting one leaves the balance alone.
function askDeleteLog(entry) {
  const acct = state.current;
  const value = formatMoney(acct.currency, entry.valueCents);

  confirmDelete("Delete history entry?", `The value ${value} for ${entry.date} will be deleted. The account's current amount doesn't change.`, async () => {
    await api.DeleteAccountValueLog(acct.id, entry.id);
    await loadLogs();
    setStatus(`deleted log value for ${entry.date}`);
  });
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
    archived: form.archived.checked,
  };

  try {
    const result = await busy(el.amountForm, () => api.UpdateAccountAmount(input));
    el.accountDialog.close();

    const warning = result.warning ? ` (${result.warning})` : "";
    const action = input.archived === acct.archived ? "updated amount for" : input.archived ? "archived" : "unarchived";
    await show(`${action} ${acct.name}${warning}`);
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
  const acct = state.current;

  // Entries link to it: it can go only by moving them to another account.
  if (acct.usedBy > 0) {
    const others = (state.overview?.accounts ?? []).filter((other) => other.id !== acct.id);
    openMerge({
      title: "Merge account?",
      message: `${acct.usedBy} income or expense record(s) are on “${acct.name}”, so it can't just be deleted. Merge it into another account: they move there, and “${acct.name}” is deleted with its value history.`,
      note: "To keep it but stop offering it for new records, archive it instead.",
      options: others.map((other) => ({ id: other.id, label: `${other.name} (${other.currency})${other.archived ? ", archived" : ""}` })),
      merge: async (into) => {
        const result = await api.Merge({ kind: "account", fromId: acct.id, intoId: into });
        el.accountDialog.close();
        const target = others.find((o) => o.id === into).name;
        await show(`merged account ${acct.name} into ${target}: ${result.moved} record(s) moved`);
      },
    });
    return;
  }

  confirmDelete("Delete account?", `“${acct.name}” and its value history will be deleted. This can't be undone.`, async () => {
    await api.DeleteAccount(acct.id);
    el.accountDialog.close();
    await show(`deleted account ${acct.name}`);
  });
}
