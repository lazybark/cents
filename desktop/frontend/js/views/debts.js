// Debts: what I owe and what I'm owed, unpaid or paid, with paid progress.
// Each debt can be edited, take logged payments and be deleted, like in the
// TUI. A debt keeps the rate to the base currency it was recorded with.
import { api } from "../api.js";
import { cashflowInput, setupCashflow } from "../payment-cashflow.js";
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
  renderPaymentLogs,
  setProgress,
  setStatus,
  showRate,
  signedMoney,
  syncRate,
  withBase,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  add: $("add-debt"),
  tabs: document.querySelectorAll("#view-debts .tab"),
  progressText: $("debts-progress-text"),
  progressFill: $("debts-progress-fill"),
  rows: $("debts-rows"),
  empty: $("debts-empty"),
  hint: $("debts-hint"),

  addDialog: $("debt-add-dialog"),
  addForm: $("debt-add-form"),

  editDialog: $("debt-edit-dialog"),
  editTitle: $("debt-title"),
  editSubtitle: $("debt-subtitle"),
  editProgress: $("debt-progress-fill"),
  editForm: $("debt-edit-form"),
  paymentForm: $("debt-payment-form"),
  logs: $("debt-logs"),
  logsEmpty: $("debt-logs-empty"),
  delete: $("debt-delete"),
};

const TAB_LABELS = { outgoing: "I owe", incoming: "Owed to me", paid: "Paid" };

const state = {
  mode: "outgoing",
  view: null,
  current: null,
};

export const title = "Debts";

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
  el.paymentForm.addEventListener("submit", logPayment);
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    const view = await api.Debts(state.mode);
    state.view = view;
    state.mode = view.mode;
    render(view);
    setStatus(message ?? `${view.debts.length} debt(s)`);
  } catch (err) {
    setStatus(`Failed to load debts: ${err}`);
  }
}

function direction(debt) {
  return debt.isOwedToUser ? "owed to me" : "I owe";
}

function render(view) {
  for (const tab of el.tabs) {
    tab.setAttribute("aria-selected", String(tab.dataset.mode === view.mode));
    tab.textContent = `${TAB_LABELS[tab.dataset.mode]} (${view.counts[tab.dataset.mode] ?? 0})`;
  }

  el.progressText.textContent = progressText(view.baseCurrency, view.progress.paidCents, view.progress.totalCents);
  setProgress(el.progressFill, view.progress.paidCents, view.progress.totalCents);

  el.empty.hidden = view.debts.length > 0;
  el.hint.hidden = view.debts.length === 0;
  el.rows.replaceChildren(
    ...view.debts.map((debt) => {
      const row = document.createElement("tr");

      const peer = document.createElement("div");
      peer.className = "account-name";
      peer.textContent = debt.peer;
      // The paid list mixes both directions, so label them there.
      if (view.mode === "paid") peer.append(badge(direction(debt)));

      const comment = document.createElement("div");
      comment.className = "account-description";
      comment.textContent = debt.comment;

      const peerCell = document.createElement("div");
      peerCell.append(peer, comment);

      row.append(
        cell(peerCell),
        cell(debt.createdAt, "nowrap"),
        dueCell(debt.dueDate, debt.overdue),
        cell(withBase(debt, debt.amountCents, view.baseCurrency, debt.baseAmountCents), "num"),
        cell(withBase(debt, debt.paidCents, view.baseCurrency, debt.basePaidCents), "num"),
        cell(withBase(debt, debt.leftCents, view.baseCurrency, debt.baseLeftCents), debt.leftCents > 0 ? "num" : "num muted"),
      );
      clickableRow(row, () => openEdit(debt));

      return row;
    }),
  );
}

// --- add -------------------------------------------------------------------

function openAdd() {
  const form = el.addForm;
  form.reset();
  fillSelect(
    form.elements.currency,
    (state.view?.currencies ?? []).map((c) => c.name),
  );
  syncRate(form, state.view?.currencies ?? []);
  // Start with the direction of the list being looked at.
  form.elements.direction.value = state.mode === "incoming" ? "incoming" : "outgoing";
  form.elements.createdAt.value = localDate(new Date());
  formError(form, "");
  el.addDialog.showModal();
}

async function saveNew(event) {
  event.preventDefault();
  const form = el.addForm.elements;
  const input = {
    isOwedToUser: form.direction.value === "incoming",
    peer: form.peer.value,
    currency: form.currency.value,
    rate: rateInput(el.addForm),
    amount: form.amount.value,
    amountPaid: form.amountPaid.value,
    createdAt: form.createdAt.value,
    dueDate: form.dueDate.value,
    comment: form.comment.value,
  };

  try {
    const created = await busy(el.addForm, () => api.CreateDebt(input));
    el.addDialog.close();
    state.mode = created.mode;
    await show(`saved debt for ${input.peer.trim()}`);
  } catch (err) {
    formError(el.addForm, String(err));
  }
}

// --- edit, payments, delete ------------------------------------------------

function openEdit(debt) {
  state.current = debt;
  const form = el.editForm.elements;

  form.amount.value = formatAmount(debt.amountCents);
  form.amountPaid.value = formatAmount(debt.paidCents);
  form.createdAt.value = debt.createdAt;
  form.dueDate.value = debt.dueDate;
  form.comment.value = debt.comment;
  // Debts saved before rates were kept may have none: one can be added.
  showRate(el.editForm, state.view.baseCurrency, debt.currency, debt.rateToBase, debt.isBase);
  formError(el.editForm, "");

  el.paymentForm.reset();
  el.paymentForm.elements.date.value = localDate(new Date());
  // A payment on a debt owed to me is money in: an income.
  setupCashflow(el.paymentForm, { isIncome: debt.isOwedToUser, options: state.view.cashflow });
  formError(el.paymentForm, "");

  showDebtHeader(debt);
  el.logs.replaceChildren();
  el.logsEmpty.hidden = true;
  el.editDialog.showModal();
  loadLogs();
}

function showDebtHeader(debt) {
  el.editTitle.textContent = debt.peer;
  const base = state.view.baseCurrency;
  const left = debt.isBase ? "" : debt.hasRate ? ` · ${formatMoney(base, debt.baseLeftCents)} left at ${rateText(debt.rateToBase)}` : " · no rate to " + base;
  el.editSubtitle.textContent = `${direction(debt)} · ${progressText(debt.currency, debt.paidCents, debt.amountCents)}${left}`;
  setProgress(el.editProgress, debt.paidCents, debt.amountCents);
}

async function loadLogs() {
  const debt = state.current;

  try {
    renderPaymentLogs(el.logs, el.logsEmpty, await api.DebtLogs(debt.id), debt.currency, askDeletePayment);
  } catch (err) {
    formError(el.paymentForm, `Failed to load payments: ${err}`);
  }
}

async function saveEdit(event) {
  event.preventDefault();
  const debt = state.current;
  const form = el.editForm.elements;
  const input = {
    id: debt.id,
    rate: rateInput(el.editForm),
    amount: form.amount.value,
    amountPaid: form.amountPaid.value,
    createdAt: form.createdAt.value,
    dueDate: form.dueDate.value,
    comment: form.comment.value,
  };

  try {
    await busy(el.editForm, () => api.UpdateDebt(input));
    el.editDialog.close();
    await show(`updated debt for ${debt.peer}`);
  } catch (err) {
    formError(el.editForm, String(err));
  }
}

// A payment keeps the dialog open: it updates the header, the amount paid
// field and the log, and refreshes the list behind.
async function logPayment(event) {
  event.preventDefault();
  const form = el.paymentForm.elements;
  const input = { id: state.current.id, delta: form.delta.value, date: form.date.value, note: form.note.value, cashflow: cashflowInput(el.paymentForm) };

  try {
    const updated = await busy(el.paymentForm, () => api.AddDebtPayment(input));
    state.current = updated;
    showDebtHeader(updated);
    el.editForm.elements.amountPaid.value = formatAmount(updated.paidCents);
    form.delta.value = "";
    form.note.value = "";
    formError(el.paymentForm, "");
    await loadLogs();
    await show(input.cashflow.add ? `logged payment and added it as an ${state.current.isOwedToUser ? "income" : "expense"}` : "logged payment");
  } catch (err) {
    formError(el.paymentForm, String(err));
  }
}

// Deleting a payment undoes it, so say what amount paid goes back to.
function askDeletePayment(entry) {
  const debt = state.current;
  const money = (cents) => formatMoney(debt.currency, cents);
  const linked = entry.cashflowId ? ` Its ${debt.isOwedToUser ? "income" : "expense"} in Incomes & expenses is deleted too.` : "";
  const change = `Amount paid goes from ${money(debt.paidCents)} to ${money(debt.paidCents - entry.deltaCents)}.${linked}`;

  confirmDelete("Delete payment?", `The ${signedMoney(debt.currency, entry.deltaCents)} payment from ${entry.when} will be deleted. ${change}`, async () => {
    const updated = await api.DeleteDebtPayment(debt.id, entry.id);
    state.current = updated;
    showDebtHeader(updated);
    el.editForm.elements.amountPaid.value = formatAmount(updated.paidCents);
    await loadLogs();
    await show("deleted payment");
  });
}

function askDelete() {
  const debt = state.current;

  confirmDelete("Delete debt?", `The debt with “${debt.peer}” and its payment log will be deleted. This can't be undone.`, async () => {
    await api.DeleteDebt(debt.id);
    el.editDialog.close();
    await show(`deleted debt ${debt.peer}`);
  });
}
