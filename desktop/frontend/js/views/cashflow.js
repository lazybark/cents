// Incomes & expenses: one month's entries with totals, or every month's
// totals side by side. Entries can be added and deleted, like in the TUI.
// Each keeps the rate to the base currency it was made at; the rate of an
// entry (or of a whole month's entries in its currency) can be corrected.
import { api } from "../api.js";
import { chart, legend, monthLabel } from "../charts.js";
import { budgetRow, focusMonth as focusBudgetMonth } from "./budgets.js";
import { initCategories, loadCategories } from "./cashflow-categories.js";
import { focusSearch, initSearch, loadSearch, presetSearch } from "./cashflow-search.js";
import { initTags, loadTags } from "./cashflow-tags.js";
import { commentWithTags, setTagSuggestions, tagsInput } from "../tags-input.js";
import { noteFor, noteLine } from "../notes.js";
import {
  badge,
  busy,
  cell,
  clickableRow,
  confirmDelete,
  fillSelect,
  formatAmount,
  formatMoney,
  formError,
  localDate,
  rateInput,
  rateText,
  rowAction,
  setStatus,
  showRate,
  signClass,
  syncRate,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  addIncome: $("add-income"),
  addExpense: $("add-expense"),
  tabs: document.querySelectorAll("#view-cashflow .tab"),

  monthPane: $("cashflow-month"),
  monthPrev: $("month-prev"),
  monthNext: $("month-next"),
  monthCurrent: $("month-current"),
  monthLabel: $("month-label"),
  monthIncome: $("month-income"),
  monthExpense: $("month-expense"),
  monthNet: $("month-net"),
  monthMissing: $("month-missing"),
  monthNote: $("month-note"),
  monthBaseHeading: $("month-base-heading"),
  monthEntries: $("month-entries"),
  monthSortButtons: document.querySelectorAll("#month-table .sort-button"),
  monthEmpty: $("month-empty"),
  monthBudgets: $("month-budgets"),
  monthBudgetsList: $("month-budgets-list"),
  monthBudgetsOpen: $("month-budgets-open"),

  monthsPane: $("cashflow-months"),
  monthsIncomeHeading: $("months-income-heading"),
  monthsExpenseHeading: $("months-expense-heading"),
  monthsNetHeading: $("months-net-heading"),
  monthsRows: $("months-rows"),
  monthsEmpty: $("months-empty"),
  monthsMissing: $("months-missing"),
  monthsHint: $("months-hint"),

  statsPane: $("cashflow-stats"),
  categoriesPane: $("cashflow-categories"),
  searchPane: $("cashflow-search"),
  tagsPane: $("cashflow-tags-pane"),
  statsRange: $("stats-range"),
  statsChart: $("stats-chart"),
  statsLegend: $("stats-legend"),
  statsEmpty: $("stats-empty"),
  statsMissing: $("stats-missing"),
  statsArchived: $("stats-archived"),
  statsArchivedNote: $("stats-archived-note"),
  statsFigures: $("stats-figures-panel"),
  statsAverage: $("stats-average"),
  statsTotal: $("stats-total"),
  statsBest: $("stats-best"),
  statsBestMonth: $("stats-best-month"),
  statsWorst: $("stats-worst"),
  statsWorstMonth: $("stats-worst-month"),
  statsMore: $("stats-more"),

  dialog: $("cashflow-dialog"),
  form: $("cashflow-form"),
  formTitle: $("cashflow-title"),
  noCategories: $("cashflow-no-categories"),
  kindField: $("cashflow-kind-field"),
  saveMore: $("cashflow-save-more"),
  saveMoreHint: $("cashflow-save-more-hint"),
  saved: $("cashflow-saved"),
  budget: $("cashflow-budget"),

  rateDialog: $("cashflow-rate-dialog"),
  rateForm: $("cashflow-rate-form"),
  rateEntry: $("cashflow-rate-entry"),
  rateMonth: $("cashflow-rate-month"),
};

// --- sorting the month's entries ------------------------------------------

const SORT_KEY = "cents.cashflowSort";

// How each column sorts. first is the direction a column starts with when
// picked; value gives what to compare, with null (nothing to compare) last.
const text = (pick) => (entry) => pick(entry)?.trim() || null;
const signedBase = (entry) => (entry.hasRate ? (entry.isIncome ? 1 : -1) * entry.baseCents : null);
const SORTS = {
  date: { first: "desc", value: (entry) => entry.date },
  category: { first: "asc", value: text((entry) => entry.category) },
  account: { first: "asc", value: text((entry) => entry.account) },
  comment: { first: "asc", value: text((entry) => entry.comment) },
  // Amounts in different currencies don't compare, so group by currency.
  amount: { first: "desc", value: (entry) => [entry.currency, (entry.isIncome ? 1 : -1) * entry.amountCents] },
  base: { first: "desc", value: signedBase },
};
const DEFAULT_SORT = { key: "category", dir: "asc" };

function readSort() {
  try {
    const saved = JSON.parse(localStorage.getItem(SORT_KEY) ?? "null");
    if (saved && SORTS[saved.key] && (saved.dir === "asc" || saved.dir === "desc")) return saved;
  } catch {
    // No storage (or a bad value): use the default.
  }

  return DEFAULT_SORT;
}

function writeSort(sort) {
  try {
    localStorage.setItem(SORT_KEY, JSON.stringify(sort));
  } catch {
    // The choice just isn't remembered.
  }
}

function compareValues(a, b) {
  if (Array.isArray(a)) {
    for (let i = 0; i < a.length; i++) {
      const order = compareValues(a[i], b[i]);
      if (order !== 0) return order;
    }
    return 0;
  }

  if (typeof a === "number") return a - b;
  return String(a).localeCompare(String(b), undefined, { sensitivity: "base", numeric: true });
}

// sortEntries orders entries by the chosen column; ties fall back to the
// newest date first, then the newest entry.
function sortEntries(entries, { key, dir }) {
  const { value } = SORTS[key];
  const sign = dir === "asc" ? 1 : -1;

  return [...entries].sort((a, b) => {
    const va = value(a);
    const vb = value(b);
    if (va === null || vb === null) {
      if (va !== vb) return va === null ? 1 : -1;
    } else {
      const order = compareValues(va, vb) * sign;
      if (order !== 0) return order;
    }

    return b.date.localeCompare(a.date) || b.id - a.id;
  });
}

const state = {
  tab: "month",
  // "YYYY-MM"; empty until the first load picks the current month.
  month: "",
  options: null,
  isIncome: true,
  sort: readSort(),
  // The month's entries as loaded, to sort without loading them again.
  entries: [],
  baseCurrency: "",
  // The entry whose rate is being set.
  rated: null,
  // The entry open in the dialog for editing, or null when adding one.
  editing: null,
  // How many entries "Save & add another" saved since the dialog opened.
  savedCount: 0,
  // Budgets against spending by month ("YYYY-MM"), for the dialog's budget
  // line; the shown month's come with it, others are loaded when needed.
  budgets: new Map(),
};

export const title = "Incomes & expenses";

// focusMonth makes the view open on one month's entries ("YYYY-MM") the
// next time it shows.
export function focusMonth(month) {
  state.month = month;
  state.tab = "month";
}

export function init() {
  for (const tab of el.tabs) {
    tab.addEventListener("click", () => {
      state.tab = tab.dataset.tab;
      show();
    });
  }

  el.monthPrev.addEventListener("click", () => goToMonth(shiftMonth(state.month, -1)));
  el.monthNext.addEventListener("click", () => goToMonth(shiftMonth(state.month, 1)));
  el.monthCurrent.addEventListener("click", () => goToMonth(""));
  el.addIncome.addEventListener("click", () => openAdd(true));
  el.addExpense.addEventListener("click", () => openAdd(false));
  el.form.addEventListener("submit", save);
  el.form.elements.currency.addEventListener("change", currencyPicked);
  // ⌘ / Ctrl + Enter saves and keeps the dialog open, while adding.
  el.form.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey) && !el.saveMore.hidden) {
      event.preventDefault();
      el.form.requestSubmit(el.saveMore);
    }
  });
  el.form.elements.kind.addEventListener("change", () => {
    fillCategories(el.form.elements.kind.value === "income");
    showBudget();
  });
  // The budget line follows whatever changes what the entry adds to it.
  for (const name of ["amount", "rate", "date", "category", "currency"]) {
    el.form.elements[name].addEventListener(name === "amount" || name === "rate" ? "input" : "change", showBudget);
  }
  el.monthBudgetsOpen.addEventListener("click", () => openBudgets(state.month));
  el.rateForm.addEventListener("submit", saveRate);
  el.statsRange.addEventListener("change", () => show());
  el.statsArchived.addEventListener("change", () => show());
  for (const button of el.monthSortButtons) {
    const key = button.dataset.sort;
    button.addEventListener("click", () => {
      const same = state.sort.key === key;
      state.sort = { key, dir: same ? (state.sort.dir === "asc" ? "desc" : "asc") : SORTS[key].first };
      writeSort(state.sort);
      renderEntries();
    });
  }
  initCategories();
  initSearch({ openEdit });
  initTags({ showEntries: searchTag });
  state.tags = tagsInput($("cashflow-tags"));
}


// openSearch makes the view open on Search with the cursor in its box.
export function openSearch() {
  state.tab = "search";
  state.focusSearch = true;
}

// searchTag switches to Search showing the entries with a tag.
export async function searchTag(name) {
  presetSearch({ tag: name });
  state.tab = "search";
  await show();
}

export async function show(message) {
  for (const tab of el.tabs) tab.setAttribute("aria-selected", String(tab.dataset.tab === state.tab));
  el.monthPane.hidden = state.tab !== "month";
  el.monthsPane.hidden = state.tab !== "months";
  el.statsPane.hidden = state.tab !== "stats";
  el.categoriesPane.hidden = state.tab !== "categories";
  el.searchPane.hidden = state.tab !== "search";
  el.tagsPane.hidden = state.tab !== "tags";

  try {
    if (state.tab === "month") await loadMonth();
    else if (state.tab === "months") await loadMonths();
    else if (state.tab === "stats") await loadStats();
    else if (state.tab === "search") await loadSearch();
    else if (state.tab === "tags") await loadTags();
    else await loadCategories();

    if (state.tab === "search" && state.focusSearch) {
      state.focusSearch = false;
      focusSearch();
    }

    const shown = { month: `showing ${monthName(state.month)}`, months: "all months", stats: "statistics", categories: "by category", search: "search", tags: "tags" };
    setStatus(message ?? shown[state.tab]);
  } catch (err) {
    setStatus(`Failed to load incomes and expenses: ${err}`);
  }
}

function goToMonth(month) {
  state.month = month;
  state.tab = "month";
  show();
}

// --- months ----------------------------------------------------------------

function shiftMonth(month, delta) {
  const [year, index] = month.split("-").map(Number);
  const date = new Date(year, index - 1 + delta, 1);

  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function monthName(month) {
  const [year, index] = month.split("-").map(Number);

  return new Date(year, index - 1, 1).toLocaleString(undefined, { month: "long", year: "numeric" });
}

// --- one month -------------------------------------------------------------

async function loadMonth() {
  const data = await api.CashflowMonth(state.month);
  const money = (cents) => formatMoney(data.baseCurrency, cents);

  state.month = data.month;
  state.options = data.options;
  setTagSuggestions(data.options.tags);
  state.baseCurrency = data.baseCurrency;

  el.monthLabel.textContent = monthName(data.month);
  el.monthNote.replaceChildren(noteLine(data.month));
  el.monthIncome.textContent = money(data.incomeCents);
  el.monthExpense.textContent = money(data.expenseCents);
  el.monthNet.textContent = money(data.netCents);
  el.monthNet.className = `figure ${signClass(data.netCents)}`;
  el.monthMissing.hidden = data.missingRates === 0;
  el.monthMissing.textContent = `${data.missingRates} record(s) have no rate and aren't counted in totals. Use “Rate” on them to add one.`;

  // Saving or deleting entries changes what budgets have spent.
  state.budgets = new Map([[data.month, data.budgets]]);
  el.monthBudgets.hidden = data.budgets.length === 0;
  el.monthBudgetsList.replaceChildren(
    ...data.budgets.map((row) => budgetRow(row, data.baseCurrency, { compact: true, open: () => openBudgets(data.month) })),
  );

  el.monthBaseHeading.textContent = `In ${data.baseCurrency}`;
  el.monthEmpty.hidden = data.entries.length > 0;
  state.entries = data.entries;
  renderEntries();
}

function renderEntries() {
  for (const button of el.monthSortButtons) {
    const header = button.closest("th");
    if (button.dataset.sort === state.sort.key) header.setAttribute("aria-sort", state.sort.dir === "asc" ? "ascending" : "descending");
    else header.removeAttribute("aria-sort");
  }

  el.monthEntries.replaceChildren(...sortEntries(state.entries, state.sort).map((entry) => entryRow(entry, state.baseCurrency)));
}

function entryRow(entry, baseCurrency) {
  // Incomes read as "+$ 10.00" in green, expenses as "-$ 10.00".
  const sign = entry.isIncome ? 1 : -1;
  const signed = (currency, cents) => (entry.isIncome ? "+" : "") + formatMoney(currency, sign * cents);
  const amountClass = entry.isIncome ? "num positive" : "num";

  const row = document.createElement("tr");
  const actions = document.createElement("span");
  actions.className = "nowrap";
  // Base currency entries always use rate 1, so there's nothing to set.
  actions.append(rowAction("Edit", () => openEdit(entry)));
  if (!entry.isBase) actions.append(rowAction("Rate", () => openRate(entry)));
  actions.append(rowAction("Delete", () => askDelete(entry)));

  const base = document.createElement("div");
  base.append(entry.hasRate ? signed(baseCurrency, entry.baseCents) : "no rate");
  if (!entry.isBase && entry.hasRate) {
    const rate = document.createElement("div");
    rate.className = "account-description";
    rate.textContent = `at ${rateText(entry.rateToBase)}`;
    base.append(rate);
  }

  row.append(
    cell(entry.date, "nowrap"),
    cell(entry.categoryArchived ? withBadge(entry.category, "archived") : entry.category),
    cell(entry.account || "—", entry.account ? "" : "muted"),
    cell(commentWithTags(entry), "muted wrap"),
    cell(signed(entry.currency, entry.amountCents), amountClass),
    cell(base, entry.hasRate ? amountClass : "num muted"),
    cell(actions, "num"),
  );

  return row;
}

// --- rates -----------------------------------------------------------------

function openRate(entry) {
  state.rated = entry;
  const form = el.rateForm;
  const kind = entry.isIncome ? "income" : "expense";

  form.reset();
  el.rateEntry.textContent = `${formatMoney(entry.currency, entry.amountCents)} ${kind} on ${entry.date} · ${entry.category}`;
  showRate(form, state.baseCurrency, entry.currency, entry.rateToBase);
  el.rateMonth.textContent = `Use this rate for every ${entry.currency} entry in ${monthName(state.month)}`;
  formError(form, "");
  el.rateDialog.showModal();
}

async function saveRate(event) {
  event.preventDefault();
  const form = el.rateForm.elements;
  const input = { id: state.rated.id, rate: form.rate.value, wholeMonth: form.wholeMonth.checked };

  try {
    const changed = await busy(el.rateForm, () => api.SetCashflowRate(input));
    el.rateDialog.close();
    await show(`set rate ${rateText(Number(input.rate))} on ${changed} entr${changed === 1 ? "y" : "ies"}`);
  } catch (err) {
    formError(el.rateForm, String(err));
  }
}

function withBadge(text, label) {
  const span = document.createElement("span");
  span.append(text, badge(label));
  return span;
}

function askDelete(entry) {
  const kind = entry.isIncome ? "income" : "expense";

  confirmDelete(`Delete ${kind}?`, `The ${kind} “${entry.date} ${entry.category}” will be deleted. This can't be undone.`, async () => {
    await api.DeleteCashflow(entry.id);
    await show("deleted cashflow entry");
  });
}

// --- all months ------------------------------------------------------------

async function loadMonths() {
  const data = await api.CashflowOverview();
  const money = (cents) => formatMoney(data.baseCurrency, cents);

  el.monthsIncomeHeading.textContent = `Income (${data.baseCurrency})`;
  el.monthsExpenseHeading.textContent = `Expense (${data.baseCurrency})`;
  el.monthsNetHeading.textContent = `Net (${data.baseCurrency})`;
  el.monthsEmpty.hidden = data.rows.length > 0;
  el.monthsHint.hidden = data.rows.length === 0;
  el.monthsMissing.hidden = data.missingRates === 0;
  el.monthsMissing.textContent = `${data.missingRates} record(s) have no rate and aren't counted. Open their month to add one.`;

  el.monthsRows.replaceChildren(
    ...data.rows.map((item) => {
      const row = document.createElement("tr");
      const delta = item.hasPrev ? (item.deltaCents > 0 ? "+" : "") + money(item.deltaCents) : "—";

      // The month's note, if it has one, under its name.
      const name = document.createElement("div");
      name.append(monthName(item.month));
      if (noteFor(item.month)) {
        const note = document.createElement("div");
        note.className = "months-note";
        note.textContent = noteFor(item.month);
        name.append(note);
      }

      row.append(
        cell(name),
        cell(money(item.incomeCents), "num"),
        cell(money(item.expenseCents), "num"),
        cell(money(item.netCents), `num ${signClass(item.netCents)}`),
        cell(delta, `num ${item.hasPrev ? signClass(item.deltaCents) : "muted"}`),
      );
      clickableRow(row, () => goToMonth(item.month));

      return row;
    }),
  );
}

// --- statistics ------------------------------------------------------------

// loadStats charts the net result of the chosen months, with income and
// expense as lines behind it, and sums them up below. Entries in archived
// categories count unless "Include archived" is unticked.
async function loadStats() {
  const data = await api.CashflowStats(el.statsArchived.checked);
  const base = data.baseCurrency;
  const money = (cents) => formatMoney(base, cents);
  const signed = (cents) => (cents > 0 ? "+" : "") + money(cents);

  // The overview lists months newest first, with empty months in between.
  const all = [...data.rows].reverse();
  const range = Number(el.statsRange.value);
  const rows = range > 0 ? all.slice(-range) : all;

  el.statsEmpty.hidden = rows.length > 0;
  el.statsFigures.hidden = rows.length === 0;
  el.statsMissing.hidden = data.missingRates === 0;
  el.statsMissing.textContent = `${data.missingRates} record(s) have no rate and aren't counted. Open their month to add one.`;
  el.statsArchivedNote.hidden = data.archivedLeftOut === 0;
  el.statsArchivedNote.textContent = `${data.archivedLeftOut} entr${data.archivedLeftOut === 1 ? "y" : "ies"} in archived categories left out. They still count in each month's entries and totals.`;

  chart(el.statsChart, {
    months: rows.map((r) => r.month),
    series: [
      { type: "line", values: rows.map((r) => r.incomeCents), className: "line-income" },
      { type: "line", values: rows.map((r) => r.expenseCents), className: "line-expense" },
      { type: "bar", values: rows.map((r) => r.netCents) },
    ],
    tooltip: (i) => [
      monthLabel(rows[i].month, true),
      `Income  ${money(rows[i].incomeCents)}`,
      `Expense ${money(rows[i].expenseCents)}`,
      `Net     ${signed(rows[i].netCents)}`,
    ],
    label: `Net result by month in ${base}`,
  });
  legend(el.statsLegend, rows.length === 0 ? [] : [
    ["bar-positive", `Net (${base})`],
    ["line-income", "Income"],
    ["line-expense", "Expense"],
  ]);

  if (rows.length === 0) return;

  const total = rows.reduce((sum, r) => sum + r.netCents, 0);
  const best = rows.reduce((a, b) => (b.netCents > a.netCents ? b : a));
  const worst = rows.reduce((a, b) => (b.netCents < a.netCents ? b : a));
  const average = Math.round(total / rows.length);
  const tone = (node, cents) => {
    node.classList.toggle("positive", cents > 0);
    node.classList.toggle("negative", cents < 0);
  };

  el.statsAverage.textContent = signed(average);
  tone(el.statsAverage, average);
  el.statsTotal.textContent = signed(total);
  tone(el.statsTotal, total);
  el.statsBest.textContent = signed(best.netCents);
  tone(el.statsBest, best.netCents);
  el.statsBestMonth.textContent = monthName(best.month);
  el.statsWorst.textContent = signed(worst.netCents);
  tone(el.statsWorst, worst.netCents);
  el.statsWorstMonth.textContent = monthName(worst.month);

  const income = rows.reduce((sum, r) => sum + r.incomeCents, 0);
  const expense = rows.reduce((sum, r) => sum + r.expenseCents, 0);
  const positive = rows.filter((r) => r.netCents > 0).length;
  const more = [
    ["Months shown", `${rows.length}, from ${monthName(rows[0].month)}`],
    ["Months with a positive net", `${positive} of ${rows.length}`],
    ["Average income a month", money(Math.round(income / rows.length))],
    ["Average expense a month", money(Math.round(expense / rows.length))],
    ["Share of income kept", income > 0 ? `${((total / income) * 100).toFixed(1)}%` : "—"],
  ];
  el.statsMore.replaceChildren(
    ...more.flatMap(([term, value]) => {
      const dt = document.createElement("dt");
      dt.textContent = term;
      const dd = document.createElement("dd");
      dd.textContent = value;
      return [dt, dd];
    }),
  );
}

// --- add and edit ----------------------------------------------------------

async function loadOptions() {
  if (state.options) return true;

  try {
    state.options = (await api.CashflowMonth("")).options;
    setTagSuggestions(state.options.tags);
    return true;
  } catch (err) {
    setStatus(`Failed to load form options: ${err}`);
    return false;
  }
}

// withKept lists values with kept added when it isn't one of them, for an
// edited entry that keeps something no longer offered (an archived
// category, say).
function withKept(values, kept) {
  if (!kept || values.some((v) => v.toLowerCase() === kept.toLowerCase())) return values;
  return [...values, kept];
}

// fillCategories offers the categories of the chosen kind; an edited entry
// keeps its own while it stays that kind.
function fillCategories(isIncome) {
  const form = el.form;
  const options = state.options;
  const editing = state.editing;
  const kept = editing && editing.isIncome === isIncome ? editing.category : "";
  const categories = withKept(isIncome ? options.incomeCategories : options.expenseCategories, kept);
  const labels = categories.map((c) => (c === kept && editing.categoryArchived ? `${c} (archived)` : c));

  fillSelect(form.elements.category, categories, labels);
  if (kept) form.elements.category.value = kept;

  // An entry needs a category, so say where to add one up front.
  const missing = categories.length === 0;
  el.noCategories.hidden = !missing;
  el.noCategories.textContent = `No ${isIncome ? "income" : "expense"} categories yet. Add one in Settings first.`;
  form.querySelector('[type="submit"]').disabled = missing;
}

// currencyPicked shows the rate for the picked currency: an edited entry's
// own while it stays in its currency, else the one in settings now.
function currencyPicked() {
  const editing = state.editing;
  const currency = el.form.elements.currency.value;

  if (editing && currency.toLowerCase() === editing.currency.toLowerCase()) {
    showRate(el.form, state.baseCurrency, currency, editing.rateToBase, editing.isBase);
    return;
  }

  syncRate(el.form, state.options.rates);
}

async function openAdd(isIncome) {
  if (!(await loadOptions())) return;

  const form = el.form;
  const options = state.options;

  state.editing = null;
  state.isIncome = isIncome;
  state.savedCount = 0;
  form.reset();
  el.formTitle.textContent = isIncome ? "Add income" : "Add expense";
  el.kindField.hidden = true;
  showSaveMore(true);
  fillSelect(form.elements.currency, options.currencies);
  fillCategories(isIncome);
  fillSelect(form.elements.account, ["", ...options.accounts], ["— none —", ...options.accounts]);
  form.elements.date.value = localDate(new Date());
  syncRate(form, options.rates);
  state.tags.set([]);

  formError(form, "");
  el.dialog.showModal();
  showBudget();
}

async function openEdit(entry) {
  if (!(await loadOptions())) return;

  const form = el.form;
  const fields = form.elements;
  const options = state.options;

  state.editing = entry;
  form.reset();
  el.formTitle.textContent = entry.isIncome ? "Edit income" : "Edit expense";
  el.kindField.hidden = false;
  showSaveMore(false);
  fields.kind.value = entry.isIncome ? "income" : "expense";

  const currencies = withKept(options.currencies, entry.currency);
  const accounts = withKept(options.accounts, entry.account);
  fillSelect(fields.currency, currencies);
  fillCategories(entry.isIncome);
  fillSelect(fields.account, ["", ...accounts], ["— none —", ...accounts]);

  fields.date.value = entry.date;
  fields.amount.value = formatAmount(entry.amountCents);
  fields.currency.value = currencies.find((c) => c.toLowerCase() === entry.currency.toLowerCase()) ?? entry.currency;
  fields.account.value = entry.account;
  fields.comment.value = entry.comment;
  state.tags.set(entry.tags);
  currencyPicked();

  formError(form, "");
  el.dialog.showModal();
  showBudget();
}

// showSaveMore shows "Save & add another" (and its hint) while adding.
function showSaveMore(adding) {
  el.saveMore.hidden = !adding;
  el.saveMoreHint.hidden = !adding;
  el.saved.hidden = true;
}

async function save(event) {
  event.preventDefault();
  const form = el.form.elements;
  const editing = state.editing;
  // "Save & add another" keeps the dialog open with the same values, so
  // a run of entries (old ones, especially) only needs what changes.
  const keepOpen = !editing && event.submitter === el.saveMore;
  const isIncome = editing ? form.kind.value === "income" : state.isIncome;
  const input = {
    isIncome,
    currency: form.currency.value,
    rate: rateInput(el.form),
    amount: form.amount.value,
    date: form.date.value,
    category: form.category.value,
    account: form.account.value,
    comment: form.comment.value,
    tags: state.tags.get(),
  };

  // The rate is shown rounded; sent back untouched, it would round the
  // recorded one. Left empty, the recorded rate is kept.
  if (editing && input.currency.toLowerCase() === editing.currency.toLowerCase() && input.rate === rateText(editing.rateToBase)) {
    input.rate = "";
  }

  try {
    const result = await busy(el.form, () => (editing ? api.UpdateCashflow({ id: editing.id, ...input }) : api.CreateCashflow(input)));
    const kind = isIncome ? "income" : "expense";
    state.month = result.month;
    // Edited from Search, it stays on Search; otherwise the entry's month.
    if (state.tab !== "search") state.tab = "month";

    if (keepOpen) {
      state.savedCount += 1;
      const amount = formatMoney(input.currency, Math.round(Number(input.amount) * 100));
      const count = state.savedCount === 1 ? `Saved 1 ${kind}` : `Saved ${state.savedCount} ${kind}s`;
      el.saved.textContent = `${count}. Last: ${amount} on ${input.date} · ${input.category}.`;
      el.saved.hidden = false;
      formError(el.form, "");
      // The list behind shows the saved entry's month; the dialog stays.
      await show(`saved ${kind}`);
      showBudget();
      form.amount.focus();
      form.amount.select();
      return;
    }

    el.dialog.close();
    state.editing = null;
    await show(editing ? `updated ${kind}` : `saved ${kind}`);
  } catch (err) {
    formError(el.form, String(err));
  }
}

// --- budgets ---------------------------------------------------------------

function openBudgets(month) {
  focusBudgetMonth(month);
  window.dispatchEvent(new CustomEvent("cents:open-view", { detail: "budgets" }));
}

async function budgetsFor(month) {
  if (!state.budgets.has(month)) state.budgets.set(month, (await api.Budgets(month)).rows);
  return state.budgets.get(month);
}

const sameName = (a, b) => (a ?? "").trim().toLowerCase() === (b ?? "").trim().toLowerCase();

// entryBaseCents is what the dialog's entry comes to in the base currency,
// or null while the amount (or a needed rate) isn't a number yet.
function entryBaseCents() {
  const form = el.form.elements;
  const amount = Number(form.amount.value);
  if (form.amount.value.trim() === "" || !Number.isFinite(amount) || amount <= 0) return null;

  const typed = rateInput(el.form);
  const known = state.options.rates.find((r) => sameName(r.name, form.currency.value))?.rate ?? 0;
  const rate = el.form.querySelector(".rate-field").hidden ? 1 : typed.trim() !== "" ? Number(typed) : known;
  if (!Number.isFinite(rate) || rate <= 0) return null;

  return Math.round(amount * 100 * rate);
}

// showBudget tells, for an expense, how its category's budget (and the
// one for all spending) stands in the entry's month, and where this entry
// takes it. It's there to see before saving, not to stop it.
async function showBudget() {
  const form = el.form.elements;
  const isIncome = state.editing ? form.kind.value === "income" : state.isIncome;
  const month = form.date.value.slice(0, 7);
  el.budget.hidden = true;
  if (isIncome || month.length !== 7 || !el.dialog.open) return;

  const asked = `${month}|${form.category.value}`;
  el.budget.dataset.asked = asked;
  let rows;
  try {
    rows = await budgetsFor(month);
  } catch {
    return;
  }

  // Another change came in while loading; that one shows instead.
  if (el.budget.dataset.asked !== asked) return;

  const category = form.category.value;
  const lines = [rows.find((r) => !r.isTotal && sameName(r.category, category)), rows.find((r) => r.isTotal)].filter(Boolean);
  if (lines.length === 0) return;

  // An edited entry already counts where it was saved; take that out.
  const editing = state.editing;
  const counted = (row) =>
    editing && !editing.isIncome && editing.hasRate && editing.date.slice(0, 7) === month && (row.isTotal || sameName(editing.category, row.category)) ? editing.baseCents : 0;

  const adds = entryBaseCents();
  const money = (cents) => formatMoney(state.baseCurrency, cents);
  el.budget.replaceChildren(
    ...lines.map((row) => {
      const line = document.createElement("div");
      const spent = row.spentCents - counted(row);
      const name = row.isTotal ? "All spending" : `${row.name} budget`;
      const besides = counted(row) > 0 ? " besides this one" : "";
      line.append(`${name} in ${monthLabel(month, true)}: ${money(spent)} of ${money(row.limitCents)} spent${besides}`);

      if (adds !== null) {
        const after = spent + adds;
        const left = row.limitCents - after;
        const result = document.createElement("span");
        result.className = left < 0 ? "negative" : after >= row.limitCents * 0.8 ? "close" : "";
        result.textContent = left < 0 ? `; with this one ${money(after)}, ${money(-left)} over` : `; with this one ${money(after)}, ${money(left)} left`;
        line.append(result);
      }

      line.append(".");
      return line;
    }),
  );
  el.budget.hidden = false;
}
