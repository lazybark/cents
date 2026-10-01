// Taxes: unpaid or paid, with paid progress. Each tax can be edited, take
// logged payments and be deleted, like in the TUI. Amounts are always in the
// base currency.
import { api } from "../api.js";
import {
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
  renderPaymentLogs,
  setProgress,
  setStatus,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  add: $("add-tax"),
  tabs: document.querySelectorAll("#view-taxes .tab"),
  progressText: $("taxes-progress-text"),
  progressFill: $("taxes-progress-fill"),
  rows: $("taxes-rows"),
  empty: $("taxes-empty"),
  hint: $("taxes-hint"),

  addDialog: $("tax-add-dialog"),
  addForm: $("tax-add-form"),
  noTypes: $("tax-no-types"),
  addBase: $("tax-add-base"),

  editDialog: $("tax-edit-dialog"),
  editTitle: $("tax-title"),
  editSubtitle: $("tax-subtitle"),
  editProgress: $("tax-progress-fill"),
  editForm: $("tax-edit-form"),
  paymentForm: $("tax-payment-form"),
  logs: $("tax-logs"),
  logsEmpty: $("tax-logs-empty"),
  delete: $("tax-delete"),
};

const TAB_LABELS = { unpaid: "Unpaid", paid: "Paid" };

const state = {
  mode: "unpaid",
  view: null,
  current: null,
};

export const title = "Taxes";

export function init() {
  for (const tab of el.tabs) {
    tab.addEventListener("click", () => {
      state.mode = tab.dataset.mode;
      show();
    });
  }

  el.add.addEventListener("click", openAdd);
  el.addForm.addEventListener("submit", saveNew);
  el.editForm.addEventListener("submit", saveEdit);
  el.paymentForm.addEventListener("submit", logPayment);
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    const view = await api.Taxes(state.mode);
    state.view = view;
    state.mode = view.mode;
    render(view);
    setStatus(message ?? `${view.taxes.length} tax(es)`);
  } catch (err) {
    setStatus(`Failed to load taxes: ${err}`);
  }
}

function label(tax) {
  return `${tax.country} / ${tax.typeName}`;
}

function render(view) {
  const money = (cents) => formatMoney(view.baseCurrency, cents);

  for (const tab of el.tabs) {
    tab.setAttribute("aria-selected", String(tab.dataset.mode === view.mode));
    tab.textContent = `${TAB_LABELS[tab.dataset.mode]} (${view.counts[tab.dataset.mode] ?? 0})`;
  }

  el.progressText.textContent = progressText(view.baseCurrency, view.progress.paidCents, view.progress.totalCents);
  setProgress(el.progressFill, view.progress.paidCents, view.progress.totalCents);

  el.empty.hidden = view.taxes.length > 0;
  el.hint.hidden = view.taxes.length === 0;
  el.rows.replaceChildren(
    ...view.taxes.map((tax) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = label(tax);

      const comment = document.createElement("div");
      comment.className = "account-description";
      comment.textContent = tax.comment;

      const nameCell = document.createElement("div");
      nameCell.append(name, comment);

      row.append(
        cell(nameCell),
        cell(tax.period),
        dueCell(tax.dueDate, tax.overdue),
        cell(money(tax.dueCents), "num"),
        cell(money(tax.paidCents), "num"),
        cell(money(tax.leftCents), tax.leftCents > 0 ? "num" : "num muted"),
        cell(`${tax.paidPercent.toFixed(1)}%`, "num"),
      );
      clickableRow(row, () => openEdit(tax));

      return row;
    }),
  );
}

// --- add -------------------------------------------------------------------

function openAdd() {
  const view = state.view;
  const form = el.addForm;
  const types = view?.taxTypes ?? [];

  form.reset();
  fillSelect(
    form.elements.taxTypeId,
    types.map((t) => String(t.id)),
    types.map((t) => t.label),
  );
  // The TUI's form starts with today as the due date too.
  form.elements.dueDate.value = localDate(new Date());
  el.addBase.textContent = `Amounts are in ${view?.baseCurrency ?? "the base currency"}.`;

  const missing = types.length === 0;
  el.noTypes.hidden = !missing;
  form.querySelector('[type="submit"]').disabled = missing;

  formError(form, "");
  el.addDialog.showModal();
}

async function saveNew(event) {
  event.preventDefault();
  const form = el.addForm.elements;
  const input = {
    taxTypeId: Number(form.taxTypeId.value),
    amountDue: form.amountDue.value,
    amountPaid: form.amountPaid.value,
    period: form.period.value,
    dueDate: form.dueDate.value,
    comment: form.comment.value,
  };

  try {
    const created = await busy(el.addForm, () => api.CreateTax(input));
    el.addDialog.close();
    state.mode = created.mode;
    await show(`saved tax ${form.taxTypeId.selectedOptions[0]?.text ?? ""}`.trim());
  } catch (err) {
    formError(el.addForm, String(err));
  }
}

// --- edit, payments, delete ------------------------------------------------

function openEdit(tax) {
  state.current = tax;
  const form = el.editForm.elements;

  form.amountDue.value = formatAmount(tax.dueCents);
  form.amountPaid.value = formatAmount(tax.paidCents);
  form.period.value = tax.period;
  form.dueDate.value = tax.dueDate;
  form.comment.value = tax.comment;
  formError(el.editForm, "");

  el.paymentForm.reset();
  el.paymentForm.elements.date.value = localDate(new Date());
  formError(el.paymentForm, "");

  showTaxHeader(tax);
  el.logs.replaceChildren();
  el.logsEmpty.hidden = true;
  el.editDialog.showModal();
  loadLogs();
}

function showTaxHeader(tax) {
  const base = state.view.baseCurrency;
  el.editTitle.textContent = label(tax);
  el.editSubtitle.textContent = `${tax.period} · ${progressText(base, tax.paidCents, tax.dueCents)}`;
  setProgress(el.editProgress, tax.paidCents, tax.dueCents);
}

async function loadLogs() {
  const tax = state.current;

  try {
    renderPaymentLogs(el.logs, el.logsEmpty, await api.TaxLogs(tax.id), state.view.baseCurrency);
  } catch (err) {
    formError(el.paymentForm, `Failed to load payments: ${err}`);
  }
}

async function saveEdit(event) {
  event.preventDefault();
  const tax = state.current;
  const form = el.editForm.elements;
  const input = {
    id: tax.id,
    amountDue: form.amountDue.value,
    amountPaid: form.amountPaid.value,
    period: form.period.value,
    dueDate: form.dueDate.value,
    comment: form.comment.value,
  };

  try {
    await busy(el.editForm, () => api.UpdateTax(input));
    el.editDialog.close();
    await show(`updated tax ${label(tax)}`);
  } catch (err) {
    formError(el.editForm, String(err));
  }
}

async function logPayment(event) {
  event.preventDefault();
  const form = el.paymentForm.elements;
  const input = { id: state.current.id, delta: form.delta.value, date: form.date.value, note: form.note.value };

  try {
    const updated = await busy(el.paymentForm, () => api.AddTaxPayment(input));
    state.current = updated;
    showTaxHeader(updated);
    el.editForm.elements.amountPaid.value = formatAmount(updated.paidCents);
    form.delta.value = "";
    form.note.value = "";
    formError(el.paymentForm, "");
    await loadLogs();
    await show("logged tax payment");
  } catch (err) {
    formError(el.paymentForm, String(err));
  }
}

function askDelete() {
  const tax = state.current;

  confirmDelete("Delete tax?", `“${label(tax)}” for ${tax.period} and its payment log will be deleted. This can't be undone.`, async () => {
    await api.DeleteTax(tax.id);
    el.editDialog.close();
    await show(`deleted tax ${label(tax)}`);
  });
}
