// Credits: loans the user owes, still being paid or paid off. Each credit
// can be edited, take logged payments and additions (which can be deleted
// again, undoing them) and be deleted. A credit keeps the rate to the base
// currency it was recorded with, like debts.
import { api } from "../api.js";
import { cashflowInput, setupCashflow, showCashflow } from "../payment-cashflow.js";
import {
  badge,
  busy,
  cell,
  clickableRow,
  confirmDelete,
  dueCell,
  fillSelect,
  formatAmount,
  formatMoney,
  formError,
  localDate,
  progressText,
  rateInput,
  rateText,
  rowAction,
  setProgress,
  setStatus,
  showRate,
  syncRate,
  withBase,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  add: $("add-credit"),
  tabs: document.querySelectorAll("#view-credits .tab"),
  left: $("credits-left"),
  progressText: $("credits-progress-text"),
  progressFill: $("credits-progress-fill"),
  rows: $("credits-rows"),
  empty: $("credits-empty"),
  hint: $("credits-hint"),
  purposes: $("credit-purposes"),

  addDialog: $("credit-add-dialog"),
  addForm: $("credit-add-form"),

  editDialog: $("credit-edit-dialog"),
  editTitle: $("credit-title"),
  editSubtitle: $("credit-subtitle"),
  editProgress: $("credit-progress-fill"),
  editForm: $("credit-edit-form"),
  logForm: $("credit-log-form"),
  logs: $("credit-logs"),
  logsEmpty: $("credit-logs-empty"),
  delete: $("credit-delete"),
};

const TAB_LABELS = { active: "Active", paid: "Paid off" };
const KIND_LABELS = { payment: "Payment", addition: "Added" };

const state = {
  mode: "active",
  view: null,
  current: null,
};

export const title = "Credits";

export function init() {
  for (const tab of el.tabs) {
    tab.addEventListener("click", () => {
      state.mode = tab.dataset.mode;
      show();
    });
  }

  el.add.addEventListener("click", openAdd);
  el.addForm.addEventListener("submit", saveNew);
  el.addForm.elements.currency.addEventListener("change", () => syncRate(el.addForm, state.view.currencies));
  el.editForm.addEventListener("submit", saveEdit);
  el.logForm.addEventListener("submit", logEntry);
  // Only a payment is money spent; an addition grows the credit.
  el.logForm.elements.kind.addEventListener("change", () => showCashflow(el.logForm, el.logForm.elements.kind.value === "payment"));
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    const view = await api.Credits(state.mode);
    state.view = view;
    state.mode = view.mode;
    render(view);
    setStatus(message ?? `${view.credits.length} credit(s)`);
  } catch (err) {
    setStatus(`Failed to load credits: ${err}`);
  }
}

function interestText(credit) {
  return credit.interestPercent > 0 ? `${rateText(credit.interestPercent)}%` : "—";
}

function render(view) {
  const base = view.baseCurrency;

  for (const tab of el.tabs) {
    tab.setAttribute("aria-selected", String(tab.dataset.mode === view.mode));
    tab.textContent = `${TAB_LABELS[tab.dataset.mode]} (${view.counts[tab.dataset.mode] ?? 0})`;
  }

  el.left.textContent = formatMoney(base, view.leftCents);
  el.left.classList.toggle("negative", view.leftCents > 0);
  el.progressText.textContent = progressText(base, view.progress.paidCents, view.progress.totalCents);
  setProgress(el.progressFill, view.progress.paidCents, view.progress.totalCents);
  el.purposes.replaceChildren(...view.purposes.map((p) => new Option(p)));

  el.empty.hidden = view.credits.length > 0;
  el.hint.hidden = view.credits.length === 0;
  el.rows.replaceChildren(
    ...view.credits.map((credit) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = credit.name;

      const details = document.createElement("div");
      details.className = "account-description";
      const interest = credit.interestPercent > 0 ? `${interestText(credit)} a year` : "";
      details.textContent = [credit.issuer, credit.purpose, interest, credit.comment].filter(Boolean).join(" · ");

      const nameCell = document.createElement("div");
      nameCell.append(name, details);

      row.append(
        cell(nameCell),
        cell(credit.startDate, "nowrap"),
        dueCell(credit.dueDate, credit.overdue),
        cell(withBase(credit, credit.totalCents, base, credit.baseTotalCents), "num"),
        cell(withBase(credit, credit.paidCents, base, credit.basePaidCents), "num"),
        cell(withBase(credit, credit.leftCents, base, credit.baseLeftCents), credit.leftCents > 0 ? "num" : "num muted"),
      );
      clickableRow(row, () => openEdit(credit));

      return row;
    }),
  );
}

// --- add -------------------------------------------------------------------

function openAdd() {
  const form = el.addForm;
  const currencies = state.view?.currencies ?? [];

  form.reset();
  fillSelect(
    form.elements.currency,
    currencies.map((c) => c.name),
  );
  syncRate(form, currencies);
  form.elements.startDate.value = localDate(new Date());
  formError(form, "");
  el.addDialog.showModal();
}

function detailsInput(form) {
  const fields = form.elements;

  return {
    rate: rateInput(form),
    name: fields.name.value,
    purpose: fields.purpose.value,
    issuer: fields.issuer.value,
    total: fields.total.value,
    paid: fields.paid.value,
    interestPercent: fields.interestPercent.value,
    startDate: fields.startDate.value,
    dueDate: fields.dueDate.value,
    comment: fields.comment.value,
  };
}

async function saveNew(event) {
  event.preventDefault();
  const input = { ...detailsInput(el.addForm), currency: el.addForm.elements.currency.value };

  try {
    const created = await busy(el.addForm, () => api.CreateCredit(input));
    el.addDialog.close();
    state.mode = created.mode;
    await show(`saved credit ${input.name.trim()}`);
  } catch (err) {
    formError(el.addForm, String(err));
  }
}

// --- edit, log, delete -----------------------------------------------------

function openEdit(credit) {
  state.current = credit;
  const fields = el.editForm.elements;

  fields.name.value = credit.name;
  fields.purpose.value = credit.purpose;
  fields.issuer.value = credit.issuer;
  fields.total.value = formatAmount(credit.totalCents);
  fields.paid.value = formatAmount(credit.paidCents);
  fields.interestPercent.value = credit.interestPercent > 0 ? rateText(credit.interestPercent) : "";
  fields.startDate.value = credit.startDate;
  fields.dueDate.value = credit.dueDate;
  fields.comment.value = credit.comment;
  showRate(el.editForm, state.view.baseCurrency, credit.currency, credit.rateToBase, credit.isBase);
  formError(el.editForm, "");

  el.logForm.reset();
  el.logForm.elements.date.value = localDate(new Date());
  setupCashflow(el.logForm, { isIncome: false, options: state.view.cashflow });
  showCashflow(el.logForm, true);
  formError(el.logForm, "");

  showHeader(credit);
  el.logs.replaceChildren();
  el.logsEmpty.hidden = true;
  el.editDialog.showModal();
  loadLogs();
}

function showHeader(credit) {
  const base = state.view.baseCurrency;
  const parts = [progressText(credit.currency, credit.paidCents, credit.totalCents)];
  if (credit.interestPercent > 0) parts.push(`${interestText(credit)} a year`);
  if (!credit.isBase) parts.push(credit.hasRate ? `${formatMoney(base, credit.baseLeftCents)} left at ${rateText(credit.rateToBase)}` : `no rate to ${base}`);

  el.editTitle.textContent = credit.name;
  el.editSubtitle.textContent = parts.join(" · ");
  setProgress(el.editProgress, credit.paidCents, credit.totalCents);
}

async function loadLogs() {
  const credit = state.current;

  try {
    const logs = await api.CreditLogs(credit.id);
    el.logsEmpty.hidden = logs.length > 0;
    el.logs.replaceChildren(
      ...logs.map((entry) => {
        const row = document.createElement("tr");
        const isPayment = entry.kind === "payment";
        row.append(
          cell(entry.when, "nowrap"),
          cell(KIND_LABELS[entry.kind] ?? entry.kind),
          cell(noteWithLink(entry), "muted wrap"),
          cell(`${isPayment ? "−" : "+"}${formatMoney(credit.currency, entry.deltaCents)}`, `num ${isPayment ? "positive" : "negative"}`),
          cell(rowAction("Delete", () => askDeleteLog(entry)), "num"),
        );
        return row;
      }),
    );
  } catch (err) {
    formError(el.logForm, `Failed to load the log: ${err}`);
  }
}

function noteWithLink(entry) {
  const note = document.createElement("span");
  note.append(entry.note);
  if (entry.cashflowId) note.append(badge("in incomes & expenses"));
  return note;
}

async function saveEdit(event) {
  event.preventDefault();
  const credit = state.current;
  const input = { ...detailsInput(el.editForm), id: credit.id };

  try {
    const saved = await busy(el.editForm, () => api.UpdateCredit(input));
    el.editDialog.close();
    const moved = saved.mode !== state.mode;
    state.mode = saved.mode;
    await show(`updated credit ${credit.name}${moved ? `, now under “${TAB_LABELS[saved.mode]}”` : ""}`);
  } catch (err) {
    formError(el.editForm, String(err));
  }
}

// After an entry is logged or deleted the dialog stays open: it updates the
// header, the amount fields and the log, and refreshes the list behind.
async function changed(updated, message) {
  state.current = updated;
  showHeader(updated);
  el.editForm.elements.total.value = formatAmount(updated.totalCents);
  el.editForm.elements.paid.value = formatAmount(updated.paidCents);
  formError(el.logForm, "");
  await loadLogs();
  await show(message);
}

async function logEntry(event) {
  event.preventDefault();
  const fields = el.logForm.elements;
  const input = { id: state.current.id, kind: fields.kind.value, amount: fields.amount.value, date: fields.date.value, note: fields.note.value, cashflow: cashflowInput(el.logForm) };

  try {
    const updated = await busy(el.logForm, () => api.AddCreditLog(input));
    fields.amount.value = "";
    fields.note.value = "";
    await changed(updated, input.kind !== "payment" ? "logged addition" : input.cashflow.add ? "logged payment and added it as an expense" : "logged payment");
  } catch (err) {
    formError(el.logForm, String(err));
  }
}

// Deleting an entry undoes it, so say which amount goes back.
function askDeleteLog(entry) {
  const credit = state.current;
  const money = (cents) => formatMoney(credit.currency, cents);
  const isPayment = entry.kind === "payment";
  const change = isPayment
    ? `Amount paid goes from ${money(credit.paidCents)} to ${money(credit.paidCents - entry.deltaCents)}.`
    : `The amount goes from ${money(credit.totalCents)} to ${money(credit.totalCents - entry.deltaCents)}.`;
  const linked = entry.cashflowId ? " Its expense in Incomes & expenses is deleted too." : "";

  confirmDelete(`Delete ${isPayment ? "payment" : "addition"}?`, `The ${money(entry.deltaCents)} ${isPayment ? "payment" : "addition"} from ${entry.when} will be deleted. ${change}${linked}`, async () => {
    await changed(await api.DeleteCreditLog(credit.id, entry.id), `deleted ${isPayment ? "payment" : "addition"}`);
  });
}

function askDelete() {
  const credit = state.current;

  confirmDelete("Delete credit?", `The credit “${credit.name}” and its log will be deleted. This can't be undone.`, async () => {
    await api.DeleteCredit(credit.id);
    el.editDialog.close();
    await show(`deleted credit ${credit.name}`);
  });
}
