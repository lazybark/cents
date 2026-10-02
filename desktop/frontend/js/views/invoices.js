// Invoices: unpaid ones paid to me, unpaid ones I pay, and paid ones, with
// the unpaid totals. Every field can be edited, like in the TUI; one dialog
// adds and edits.
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
  rateInput,
  setStatus,
  showRate,
  syncRate,
  withBase,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  add: $("add-invoice"),
  tabs: document.querySelectorAll("#view-invoices .tab"),
  toMe: $("invoices-to-me"),
  byMe: $("invoices-by-me"),
  notCounted: $("invoices-not-counted"),
  rows: $("invoices-rows"),
  empty: $("invoices-empty"),
  hint: $("invoices-hint"),
  accounts: $("invoice-accounts"),

  dialog: $("invoice-dialog"),
  dialogTitle: $("invoice-dialog-title"),
  form: $("invoice-form"),
  openURL: $("invoice-open-url"),
  linked: $("invoice-linked"),
  delete: $("invoice-delete"),
};

const TAB_LABELS = { outgoing: "They pay me", incoming: "I pay", paid: "Paid" };

const state = {
  // Like the TUI menu, the list starts with invoices paid to me.
  mode: "outgoing",
  view: null,
  // The invoice being edited, or null when adding one.
  current: null,
};

export const title = "Invoices";

export function init() {
  for (const tab of el.tabs) {
    tab.addEventListener("click", () => {
      state.mode = tab.dataset.mode;
      show();
    });
  }

  el.add.addEventListener("click", () => openDialog(null));
  el.form.addEventListener("submit", save);
  el.form.elements.currency.addEventListener("change", currencyPicked);
  el.form.elements.paid.addEventListener("change", syncPaid);
  el.form.elements.direction.addEventListener("change", () => {
    setupPaidBox();
    syncPaid();
  });
  el.openURL.addEventListener("click", openURL);
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    const view = await api.Invoices(state.mode);
    state.view = view;
    state.mode = view.mode;
    render(view);
    setStatus(message ?? `${view.invoices.length} invoice(s)`);
  } catch (err) {
    setStatus(`Failed to load invoices: ${err}`);
  }
}

function direction(invoice) {
  return invoice.isIncoming ? "I pay" : "they pay me";
}

function render(view) {
  for (const tab of el.tabs) {
    tab.setAttribute("aria-selected", String(tab.dataset.mode === view.mode));
    tab.textContent = `${TAB_LABELS[tab.dataset.mode]} (${view.counts[tab.dataset.mode] ?? 0})`;
  }

  el.toMe.textContent = formatMoney(view.baseCurrency, view.unpaidToMeCents);
  el.byMe.textContent = formatMoney(view.baseCurrency, view.unpaidByMeCents);
  el.byMe.classList.toggle("negative", view.unpaidByMeCents > 0);
  el.notCounted.textContent =
    view.notCounted === 1
      ? "One without a currency or conversion rate isn't counted."
      : view.notCounted > 1
        ? `${view.notCounted} without a currency or conversion rate aren't counted.`
        : "";

  el.empty.hidden = view.invoices.length > 0;
  el.hint.hidden = view.invoices.length === 0;
  el.rows.replaceChildren(
    ...view.invoices.map((invoice) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = invoice.title;
      // The paid list mixes both directions, so label them there.
      if (view.mode === "paid") name.append(badge(direction(invoice)));

      const details = document.createElement("div");
      details.className = "account-description";
      details.textContent = [invoice.peer, invoice.description].filter(Boolean).join(" · ");

      const nameCell = document.createElement("div");
      nameCell.append(name, details);

      row.append(
        cell(nameCell),
        cell(invoice.invoiceDate || "—", invoice.invoiceDate ? "nowrap" : "muted"),
        dueCell(invoice.dueDate, invoice.overdue),
        cell(invoice.targetAccount || "—", invoice.targetAccount ? "" : "muted"),
        invoice.amountCents !== 0 ? cell(withBase(invoice, invoice.amountCents, view.baseCurrency, invoice.baseCents), "num") : cell("—", "num muted"),
      );
      clickableRow(row, () => openDialog(invoice));

      return row;
    }),
  );
}

// --- add and edit ----------------------------------------------------------

// setupPaidBox readies "Also add as …" for the invoice's direction: an
// expense when I pay it, an income when it's paid to me, from the target
// account when that's one of the accounts.
function setupPaidBox() {
  const form = el.form;
  const options = state.view?.cashflow;
  setupCashflow(form, { isIncome: form.elements.direction.value === "outgoing", options });

  const target = form.elements.targetAccount.value.trim().toLowerCase();
  const match = (options?.accounts ?? []).find((a) => a.toLowerCase() === target);
  if (match) form.elements.cashflowAccount.value = match;
}

// syncPaid shows the box while Paid is ticked on an invoice not yet in
// incomes and expenses, and says what happens to one that is.
function syncPaid() {
  const form = el.form;
  const invoice = state.current;
  const paid = form.elements.paid.checked;
  const linked = Boolean(invoice?.cashflowId);

  showCashflow(form, paid && !linked);
  el.linked.hidden = !linked;
  el.linked.textContent = paid
    ? "It's in Incomes & expenses. Unticking Paid or deleting the invoice deletes it there too."
    : "Saving deletes its entry in Incomes & expenses.";
}

// currencyPicked shows the rate for the picked currency: the invoice's own
// while it stays in its recorded currency, else the one in settings now.
function currencyPicked() {
  const invoice = state.current;
  const form = el.form;
  const currency = form.elements.currency.value;

  if (invoice?.currency && currency && currency.toLowerCase() === invoice.currency.toLowerCase()) {
    showRate(form, state.view.baseCurrency, currency, invoice.rateToBase, invoice.isBase);
    return;
  }

  syncRate(form, state.view?.currencies ?? []);
}

function openDialog(invoice) {
  state.current = invoice;
  const view = state.view;
  const form = el.form.elements;
  el.form.reset();

  const currencies = (view?.currencies ?? []).map((c) => c.name);
  // Keep an invoice's currency selectable even if it left settings since.
  if (invoice?.currency && !currencies.some((c) => c.toLowerCase() === invoice.currency.toLowerCase())) {
    currencies.push(invoice.currency);
  }
  fillSelect(form.currency, ["", ...currencies], ["(none)", ...currencies]);
  el.accounts.replaceChildren(...(view?.accounts ?? []).map((name) => new Option(name)));

  if (invoice) {
    el.dialogTitle.textContent = "Edit invoice";
    form.title.value = invoice.title;
    form.direction.value = invoice.isIncoming ? "incoming" : "outgoing";
    form.peer.value = invoice.peer;
    form.amount.value = invoice.amountCents !== 0 ? formatAmount(invoice.amountCents) : "";
    form.currency.value = currencies.find((c) => c.toLowerCase() === invoice.currency.toLowerCase()) ?? "";
    form.paid.checked = invoice.paid;
    form.invoiceDate.value = invoice.invoiceDate;
    form.dueDate.value = invoice.dueDate;
    form.targetAccount.value = invoice.targetAccount;
    form.url.value = invoice.url;
    form.description.value = invoice.description;
    currencyPicked();
  } else {
    el.dialogTitle.textContent = "Add invoice";
    // Start with the direction of the list being looked at, in the base
    // currency and issued today.
    form.direction.value = state.mode === "incoming" ? "incoming" : "outgoing";
    form.currency.value = currencies[0] ?? "";
    form.invoiceDate.value = localDate(new Date());
    currencyPicked();
  }

  form.paidOn.value = localDate(new Date());
  setupPaidBox();
  syncPaid();

  el.openURL.hidden = !invoice?.url;
  el.delete.hidden = !invoice;
  formError(el.form, "");
  el.dialog.showModal();
}

async function save(event) {
  event.preventDefault();
  const form = el.form.elements;
  const editing = state.current;
  const input = {
    id: editing?.id ?? 0,
    title: form.title.value,
    isIncoming: form.direction.value === "incoming",
    currency: form.currency.value,
    rate: rateInput(el.form),
    amount: form.amount.value,
    paid: form.paid.checked,
    peer: form.peer.value,
    invoiceDate: form.invoiceDate.value,
    dueDate: form.dueDate.value,
    targetAccount: form.targetAccount.value,
    url: form.url.value,
    description: form.description.value,
    cashflow: cashflowInput(el.form),
    paidOn: form.paidOn.value,
  };

  try {
    const saved = await busy(el.form, () => (editing ? api.UpdateInvoice(input) : api.CreateInvoice(input)));
    el.dialog.close();
    const name = input.title.trim();
    const added = input.cashflow.add ? ` and added it as an ${input.isIncoming ? "expense" : "income"}` : "";

    if (saved.mode === state.mode) {
      await show(`${editing ? "updated" : "saved"} invoice ${name}${added}`);
      return;
    }

    // It belongs to another list now (new, marked paid, or flipped): go there.
    state.mode = saved.mode;
    await show(`${editing ? "updated" : "saved"} invoice ${name}${added}, now under “${TAB_LABELS[saved.mode]}”`);
  } catch (err) {
    formError(el.form, String(err));
  }
}

async function openURL() {
  try {
    await api.OpenInvoiceURL(state.current.id);
    formError(el.form, "");
  } catch (err) {
    formError(el.form, String(err));
  }
}

function askDelete() {
  const invoice = state.current;

  const linked = invoice.cashflowId ? ` Its ${invoice.isIncoming ? "expense" : "income"} in Incomes & expenses is deleted too.` : "";
  confirmDelete("Delete invoice?", `The invoice “${invoice.title}” will be deleted.${linked} This can't be undone.`, async () => {
    await api.DeleteInvoice(invoice.id);
    el.dialog.close();
    await show(`deleted invoice ${invoice.title}`);
  });
}
