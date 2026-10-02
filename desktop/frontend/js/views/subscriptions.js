// Subscriptions: any regular payment (software, rent, insurance…), active
// or all, with monthly and yearly totals. Every field can be edited; a
// next payment date repeats every period, payments due soon are marked,
// and one can be marked paid early to show the next.
import { api } from "../api.js";
import {
  badge,
  busy,
  cell,
  clickableRow,
  confirmDelete,
  dueText,
  fillSelect,
  formatAmount,
  formatMoney,
  formError,
  nextPaymentCell,
  periodLabel,
  setStatus,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  add: $("add-subscription"),
  tabs: document.querySelectorAll("#view-subscriptions .tab"),
  activeMonthly: $("subs-active-monthly"),
  activeYearly: $("subs-active-yearly"),
  inactiveMonthly: $("subs-inactive-monthly"),
  inactiveYearly: $("subs-inactive-yearly"),
  inactiveTotals: document.querySelectorAll("#view-subscriptions .inactive-total"),
  baseHeading: $("subs-base-heading"),
  rows: $("subs-rows"),
  empty: $("subs-empty"),
  hint: $("subs-hint"),
  paymentMethods: $("payment-methods"),
  types: $("subscription-types"),

  dialog: $("subscription-dialog"),
  form: $("subscription-form"),
  title: $("subscription-title"),
  due: $("subscription-due"),
  dueText: $("subscription-due-text"),
  paid: $("subscription-paid"),
  delete: $("subscription-delete"),
};

const state = {
  // Like the TUI menu, the list starts with active subscriptions.
  mode: "active",
  view: null,
  // The subscription open in the dialog, or null when adding one.
  current: null,
};

export const title = "Subscriptions";

export function init() {
  for (const tab of el.tabs) {
    tab.addEventListener("click", () => {
      state.mode = tab.dataset.mode;
      show();
    });
  }

  el.add.addEventListener("click", () => openDialog(null));
  el.form.addEventListener("submit", save);
  el.paid.addEventListener("click", markPaid);
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  for (const tab of el.tabs) tab.setAttribute("aria-selected", String(tab.dataset.mode === state.mode));

  try {
    const view = await api.Subscriptions(state.mode);
    state.view = view;
    render(view);
    const soon = view.subscriptions.filter((s) => s.dueSoon).length;
    setStatus(message ?? `${view.subscriptions.length} ${state.mode === "all" ? "" : "active "}subscription(s)${soon ? `, ${soon} due soon` : ""}`);
  } catch (err) {
    setStatus(`Failed to load subscriptions: ${err}`);
  }
}

function render(view) {
  const money = (cents) => formatMoney(view.baseCurrency, cents);

  el.activeMonthly.textContent = money(view.active.monthlyCents);
  el.activeYearly.textContent = money(view.active.yearlyCents);
  el.inactiveMonthly.textContent = money(view.inactive.monthlyCents);
  el.inactiveYearly.textContent = money(view.inactive.yearlyCents);
  for (const total of el.inactiveTotals) total.hidden = view.mode !== "all";

  el.baseHeading.textContent = `In ${view.baseCurrency}`;
  el.empty.hidden = view.subscriptions.length > 0;
  el.hint.hidden = view.subscriptions.length === 0;
  el.paymentMethods.replaceChildren(...view.options.paymentMethods.map((name) => new Option(name)));
  el.types.replaceChildren(...view.options.types.map((name) => new Option(name)));

  el.rows.replaceChildren(
    ...view.subscriptions.map((sub) => {
      const row = document.createElement("tr");
      if (!sub.isActive) row.className = "ignored";

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = sub.name;
      if (!sub.isActive) name.append(badge("inactive"));

      const type = document.createElement("div");
      type.className = "account-description";
      type.textContent = sub.type;

      const nameCell = document.createElement("div");
      nameCell.append(name, type);

      row.append(
        cell(nameCell),
        nextPaymentCell(sub),
        cell(periodLabel(sub.period)),
        cell(sub.paymentMethod),
        cell(formatMoney(sub.currency, sub.amountCents), "num"),
        cell(sub.hasRate ? money(sub.baseCents) : "no rate", sub.hasRate ? "num" : "num muted"),
      );
      clickableRow(row, () => openDialog(sub));

      return row;
    }),
  );
}

// --- add and edit ----------------------------------------------------------

function openDialog(sub) {
  const options = state.view?.options;
  if (!options) return;

  state.current = sub;
  const form = el.form;
  const fields = form.elements;
  form.reset();

  // An edited subscription keeps its currency even if it left settings.
  const currencies = sub && !options.currencies.includes(sub.currency) ? [...options.currencies, sub.currency] : options.currencies;
  fillSelect(fields.currency, currencies);
  fillSelect(fields.period, options.periods, options.periods.map(periodLabel));

  if (sub) {
    el.title.textContent = sub.name;
    fields.name.value = sub.name;
    fields.type.value = sub.type;
    fields.amount.value = formatAmount(sub.amountCents);
    fields.currency.value = sub.currency;
    fields.period.value = sub.period;
    fields.nextPayment.value = sub.anchor;
    fields.paymentMethod.value = sub.paymentMethod;
    fields.isActive.checked = sub.isActive;
  } else {
    el.title.textContent = "Add subscription";
    fields.period.value = "month";
    fields.paymentMethod.value = options.paymentMethods[0] ?? "";
  }

  showDue(sub);
  el.delete.hidden = !sub;
  formError(form, "");
  el.dialog.showModal();
}

// showDue says when an edited subscription is paid next, with a way to
// mark that payment paid.
function showDue(sub) {
  el.due.hidden = !sub?.nextPayment;
  if (!sub?.nextPayment) return;

  el.dueText.replaceChildren(`Next payment ${sub.nextPayment}`);
  const label = badge(dueText(sub.dueInDays));
  if (sub.dueSoon) label.classList.add("badge-soon");
  el.dueText.append(label);
}

function formInput() {
  const fields = el.form.elements;

  return {
    id: state.current?.id ?? 0,
    name: fields.name.value,
    type: fields.type.value,
    currency: fields.currency.value,
    amount: fields.amount.value,
    period: fields.period.value,
    paymentMethod: fields.paymentMethod.value,
    nextPayment: fields.nextPayment.value,
    isActive: fields.isActive.checked,
  };
}

async function save(event) {
  event.preventDefault();
  const editing = state.current;
  const input = formInput();

  try {
    await busy(el.form, () => (editing ? api.UpdateSubscription(input) : api.CreateSubscription(input)));
    el.dialog.close();
    // A new inactive one would vanish from the active list, so like the TUI
    // every subscription shows after adding one.
    if (!editing) state.mode = "all";
    await show(`${editing ? "updated" : "saved"} subscription ${input.name.trim()}`);
  } catch (err) {
    formError(el.form, String(err));
  }
}

// markPaid marks the next payment paid; the dialog stays open showing the
// one after it.
async function markPaid() {
  const sub = state.current;

  try {
    const updated = await busy(el.form, () => api.MarkSubscriptionPaid(sub.id));
    state.current = updated;
    showDue(updated);
    formError(el.form, "");
    await show(`marked ${sub.name} paid; next payment ${updated.nextPayment}`);
  } catch (err) {
    formError(el.form, String(err));
  }
}

function askDelete() {
  const sub = state.current;

  confirmDelete("Delete subscription?", `“${sub.name}” will be deleted. This can't be undone.`, async () => {
    await api.DeleteSubscription(sub.id);
    el.dialog.close();
    await show(`deleted subscription ${sub.name}`);
  });
}
