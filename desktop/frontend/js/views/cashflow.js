// Incomes & expenses: one month's entries with totals, or every month's
// totals side by side. Entries can be added and deleted, like in the TUI.
import { api } from "../api.js";
import {
  busy,
  cell,
  clickableRow,
  confirmDelete,
  fillSelect,
  formatMoney,
  formError,
  localDate,
  rowAction,
  setStatus,
  signClass,
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
  monthBaseHeading: $("month-base-heading"),
  monthEntries: $("month-entries"),
  monthEmpty: $("month-empty"),

  monthsPane: $("cashflow-months"),
  monthsIncomeHeading: $("months-income-heading"),
  monthsExpenseHeading: $("months-expense-heading"),
  monthsNetHeading: $("months-net-heading"),
  monthsRows: $("months-rows"),
  monthsEmpty: $("months-empty"),
  monthsMissing: $("months-missing"),
  monthsHint: $("months-hint"),

  dialog: $("cashflow-dialog"),
  form: $("cashflow-form"),
  formTitle: $("cashflow-title"),
  noCategories: $("cashflow-no-categories"),
};

const state = {
  tab: "month",
  // "YYYY-MM"; empty until the first load picks the current month.
  month: "",
  options: null,
  isIncome: true,
};

export const title = "Incomes & expenses";

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
}

export async function show(message) {
  for (const tab of el.tabs) tab.setAttribute("aria-selected", String(tab.dataset.tab === state.tab));
  el.monthPane.hidden = state.tab !== "month";
  el.monthsPane.hidden = state.tab !== "months";

  try {
    if (state.tab === "month") await loadMonth();
    else await loadMonths();

    setStatus(message ?? (state.tab === "month" ? `showing ${monthName(state.month)}` : "all months"));
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

  el.monthLabel.textContent = monthName(data.month);
  el.monthIncome.textContent = money(data.incomeCents);
  el.monthExpense.textContent = money(data.expenseCents);
  el.monthNet.textContent = money(data.netCents);
  el.monthNet.className = `figure ${signClass(data.netCents)}`;
  el.monthMissing.hidden = data.missingRates === 0;
  el.monthMissing.textContent = `${data.missingRates} record(s) excluded from totals: missing conversion rate.`;

  el.monthBaseHeading.textContent = `In ${data.baseCurrency}`;
  el.monthEmpty.hidden = data.entries.length > 0;
  el.monthEntries.replaceChildren(...data.entries.map((entry) => entryRow(entry, data.baseCurrency)));
}

function entryRow(entry, baseCurrency) {
  // Incomes read as "+$ 10.00" in green, expenses as "-$ 10.00".
  const sign = entry.isIncome ? 1 : -1;
  const signed = (currency, cents) => (entry.isIncome ? "+" : "") + formatMoney(currency, sign * cents);
  const amountClass = entry.isIncome ? "num positive" : "num";

  const row = document.createElement("tr");
  const remove = rowAction("Delete", () => askDelete(entry));

  row.append(
    cell(entry.date, "nowrap"),
    cell(entry.category),
    cell(entry.account || "—", entry.account ? "" : "muted"),
    cell(entry.comment, "muted wrap"),
    cell(signed(entry.currency, entry.amountCents), amountClass),
    cell(entry.hasRate ? signed(baseCurrency, entry.baseCents) : "no rate", entry.hasRate ? amountClass : "num muted"),
    cell(remove, "num"),
  );

  return row;
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
  el.monthsMissing.textContent = `${data.missingRates} record(s) skipped: missing conversion rate.`;

  el.monthsRows.replaceChildren(
    ...data.rows.map((item) => {
      const row = document.createElement("tr");
      const delta = item.hasPrev ? (item.deltaCents > 0 ? "+" : "") + money(item.deltaCents) : "—";

      row.append(
        cell(monthName(item.month)),
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

// --- add -------------------------------------------------------------------

async function openAdd(isIncome) {
  if (!state.options) {
    try {
      state.options = (await api.CashflowMonth("")).options;
    } catch (err) {
      setStatus(`Failed to load form options: ${err}`);
      return;
    }
  }

  const form = el.form;
  const options = state.options;
  const categories = isIncome ? options.incomeCategories : options.expenseCategories;
  const kind = isIncome ? "income" : "expense";

  state.isIncome = isIncome;
  form.reset();
  el.formTitle.textContent = isIncome ? "Add income" : "Add expense";
  fillSelect(form.elements.currency, options.currencies);
  fillSelect(form.elements.category, categories);
  fillSelect(form.elements.account, ["", ...options.accounts], ["— none —", ...options.accounts]);
  form.elements.date.value = localDate(new Date());

  // An entry needs a category, so say where to add one up front.
  const missing = categories.length === 0;
  el.noCategories.hidden = !missing;
  el.noCategories.textContent = `No ${kind} categories yet. Add one in Settings first.`;
  form.querySelector('[type="submit"]').disabled = missing;

  formError(form, "");
  el.dialog.showModal();
}

async function save(event) {
  event.preventDefault();
  const form = el.form.elements;
  const input = {
    isIncome: state.isIncome,
    currency: form.currency.value,
    amount: form.amount.value,
    date: form.date.value,
    category: form.category.value,
    account: form.account.value,
    comment: form.comment.value,
  };

  try {
    const result = await busy(el.form, () => api.CreateCashflow(input));
    el.dialog.close();
    state.month = result.month;
    state.tab = "month";
    await show(input.isIncome ? "saved income" : "saved expense");
  } catch (err) {
    formError(el.form, String(err));
  }
}
