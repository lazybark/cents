// Budgets: monthly limits per expense category (or for all spending)
// against what was spent, month by month. One dialog adds and edits.
import { api } from "../api.js";
import { monthLabel } from "../charts.js";
import { noteLine } from "../notes.js";
import { badge, busy, confirmDelete, fillSelect, formatAmount, formatMoney, formError, setStatus } from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  add: $("add-budget"),
  prev: $("budgets-prev"),
  next: $("budgets-next"),
  current: $("budgets-current"),
  month: $("budgets-month"),
  summary: $("budgets-summary"),
  note: $("budgets-note"),
  list: $("budgets-list"),
  empty: $("budgets-empty"),
  missing: $("budgets-missing"),

  dialog: $("budget-dialog"),
  dialogTitle: $("budget-dialog-title"),
  form: $("budget-form"),
  base: $("budget-base"),
  usual: $("budget-usual"),
  useUsual: $("budget-use-usual"),
  delete: $("budget-delete"),
};

// The value of "All spending" in the category picker.
const TOTAL = "";

const state = {
  // The month shown, "YYYY-MM"; empty for this month.
  month: "",
  // The month to open on next time the view is switched to (focusMonth),
  // instead of this month.
  next: "",
  view: null,
  // The budget being edited, or null when adding one.
  current: null,
};

export const title = "Budgets";

// focusMonth makes the view open on month ("YYYY-MM") next time it's
// switched to, for views that send the user here about a month.
export function focusMonth(month) {
  state.next = month;
}

// enter starts on this month, or the month asked for with focusMonth: the
// month looked at last time is no reason to open on it again.
export function enter() {
  state.month = state.next;
  state.next = "";
}

export function init() {
  el.add.addEventListener("click", () => openDialog(null));
  el.prev.addEventListener("click", () => moveMonth(-1));
  el.next.addEventListener("click", () => moveMonth(1));
  el.current.addEventListener("click", () => {
    state.month = "";
    show();
  });
  el.form.addEventListener("submit", save);
  el.form.elements.category.addEventListener("change", showUsual);
  el.useUsual.addEventListener("click", () => {
    const cents = usualFor(el.form.elements.category.value);
    el.form.elements.limit.value = formatAmount(roundUp(cents));
  });
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    const view = await api.Budgets(state.month);
    state.view = view;
    state.month = view.month;
    render(view);
    setStatus(message ?? `${view.rows.length} budget(s) for ${monthLabel(view.month, true)}`);
  } catch (err) {
    setStatus(`Failed to load budgets: ${err}`);
  }
}

function moveMonth(delta) {
  const [year, index] = state.month.split("-").map(Number);
  const date = new Date(year, index - 1 + delta, 1);
  state.month = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
  show();
}

function render(view) {
  el.month.textContent = monthLabel(view.month, true);
  el.note.replaceChildren(noteLine(view.month));
  el.current.disabled = view.isCurrent;

  const over = view.rows.filter((r) => r.state === "over").length;
  const close = view.rows.filter((r) => r.state === "close").length;
  const parts = [];
  // Today counts as a day to go, like in each budget's daily allowance.
  const daysLeft = view.daysInMonth - view.daysIn + 1;
  if (view.isCurrent) parts.push(daysLeft === 1 ? "the month's last day" : `${daysLeft} days to go in the month, today included`);
  if (over > 0) parts.push(`${over} over the limit`);
  if (close > 0) parts.push(`${close} close to it`);
  if (view.rows.length > 0 && over === 0 && close === 0) parts.push("all within their limits");
  el.summary.textContent = parts.length > 0 ? `${parts.join(", ")}.` : "";
  el.summary.className = over > 0 ? "negative" : "muted";

  el.empty.hidden = view.rows.length > 0;
  el.list.replaceChildren(...view.rows.map((row) => budgetRow(row, view.baseCurrency, { month: view, open: () => openDialog(row) })));

  el.missing.hidden = view.missingRates === 0;
  el.missing.textContent = `${view.missingRates} expense${view.missingRates === 1 ? " has" : "s have"} no rate to the base currency and ${view.missingRates === 1 ? "isn't" : "aren't"} counted.`;
}

// budgetRow shows a budget: its name, spent of the limit, a bar, and what's
// left (with a daily allowance for the rest of the running month, when
// month is given and is the running one). Compact rows keep to one line
// and the bar.
export function budgetRow(row, baseCurrency, { month = null, open, compact = false }) {
  const money = (cents) => formatMoney(baseCurrency, cents);
  const item = document.createElement("div");
  item.className = `budget-row budget-${row.state}${row.isTotal ? " budget-total" : ""}${compact ? " budget-compact" : ""}`;
  item.tabIndex = 0;
  item.addEventListener("click", open);
  item.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      open();
    }
  });

  const name = document.createElement("div");
  name.className = "account-name";
  name.textContent = row.name;
  if (row.archived) name.append(badge("archived"));
  if (row.state === "over") name.append(badge("over"));
  if (row.state === "close") name.append(badge("close"));

  const amounts = document.createElement("div");
  amounts.className = "budget-amounts";
  amounts.textContent = `${money(row.spentCents)} of ${money(row.limitCents)}`;
  if (compact) {
    const left = document.createElement("span");
    left.className = row.leftCents < 0 ? "negative" : "muted";
    left.textContent = row.leftCents < 0 ? ` · ${money(-row.leftCents)} over` : ` · ${money(row.leftCents)} left`;
    amounts.append(left);
  }

  const bar = document.createElement("div");
  bar.className = "progress";
  const fill = document.createElement("div");
  fill.className = "progress-fill";
  fill.style.width = `${Math.min(100, row.percent)}%`;
  bar.append(fill);

  const detail = document.createElement("div");
  detail.className = "budget-detail";
  const lines = [];
  if (row.leftCents >= 0) lines.push(`${money(row.leftCents)} left (${Math.round(row.percent)}% used)`);
  else lines.push(`${money(-row.leftCents)} over (${Math.round(row.percent)}% used)`);

  const daysLeft = month?.isCurrent ? month.daysInMonth - month.daysIn + 1 : 0;
  if (daysLeft > 0 && row.leftCents > 0) lines.push(`about ${money(Math.floor(row.leftCents / daysLeft))} a day for the rest of the month`);
  if (row.usualCents > 0) lines.push(`usually ${money(row.usualCents)} a month`);
  detail.textContent = lines.join(" · ");

  item.append(name, amounts, bar);
  if (!compact) item.append(detail);
  return item;
}

// --- add and edit ----------------------------------------------------------

function usualFor(category) {
  return state.view?.options.usual[category] ?? 0;
}

// roundUp suggests a round limit at or above cents: to 10 below 1000, to
// 50 above.
function roundUp(cents) {
  const step = cents < 100000 ? 1000 : 5000;
  return Math.ceil(cents / step) * step;
}

function showUsual() {
  const cents = usualFor(el.form.elements.category.value);
  const money = (c) => formatMoney(state.view.baseCurrency, c);
  el.usual.textContent = cents > 0 ? `Usually ${money(cents)} a month over the last 12 months.` : "No spending on it in the last 12 months to go by.";
  el.useUsual.hidden = cents <= 0;
}

function openDialog(row) {
  const view = state.view;
  state.current = row;
  el.form.reset();
  formError(el.form, "");
  el.base.textContent = view.baseCurrency;

  const select = el.form.elements.category;
  if (row) {
    // A budget keeps its category; delete it to budget another.
    fillSelect(select, [row.category], [row.name]);
    select.disabled = true;
    el.form.elements.limit.value = formatAmount(row.limitCents);
    el.dialogTitle.textContent = `Budget for ${row.isTotal ? "all spending" : row.name}`;
  } else {
    const values = [...(view.options.hasTotal ? [] : [TOTAL]), ...view.options.categories];
    const labels = values.map((v) => (v === TOTAL ? "All spending" : v));
    if (values.length === 0) {
      setStatus("Every expense category has a budget already; add a category in Settings first.");
      return;
    }

    fillSelect(select, values, labels);
    select.disabled = false;
    el.dialogTitle.textContent = "Add budget";
  }

  showUsual();
  el.delete.hidden = !row;
  el.dialog.showModal();
  el.form.elements.limit.focus();
}

async function save(event) {
  event.preventDefault();
  const form = el.form.elements;
  const editing = state.current;
  const input = { id: editing?.id ?? 0, category: form.category.value, limit: form.limit.value };
  const name = input.category || "all spending";

  try {
    await busy(el.form, () => api.SaveBudget(input));
    el.dialog.close();
    await show(`${editing ? "updated" : "added"} the budget for ${name}`);
  } catch (err) {
    formError(el.form, String(err));
  }
}

function askDelete() {
  const row = state.current;
  confirmDelete("Delete budget?", `The budget for ${row.isTotal ? "all spending" : row.name} will be deleted. Your incomes and expenses stay as they are.`, async () => {
    await api.DeleteBudget(row.id);
    el.dialog.close();
    await show(`deleted the budget for ${row.isTotal ? "all spending" : row.name}`);
  });
}
