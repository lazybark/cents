// Subscriptions: active or all, with monthly and yearly totals, plus add,
// edit (amount, payment method, active) and delete, like in the TUI.
import { api } from "../api.js";
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

  addDialog: $("subscription-add-dialog"),
  addForm: $("subscription-add-form"),

  editDialog: $("subscription-edit-dialog"),
  editForm: $("subscription-edit-form"),
  editTitle: $("subscription-title"),
  editSubtitle: $("subscription-subtitle"),
  editDelete: $("subscription-delete"),
};

const state = {
  // Like the TUI menu, the list starts with active subscriptions.
  mode: "active",
  view: null,
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

  el.add.addEventListener("click", openAdd);
  el.addForm.addEventListener("submit", saveNew);
  el.addForm.elements.period.addEventListener("change", syncPeriodFields);
  el.editForm.addEventListener("submit", saveEdit);
  el.editDelete.addEventListener("click", askDelete);
}

export async function show(message) {
  for (const tab of el.tabs) tab.setAttribute("aria-selected", String(tab.dataset.mode === state.mode));

  try {
    const view = await api.Subscriptions(state.mode);
    state.view = view;
    render(view);
    setStatus(message ?? `${view.subscriptions.length} ${state.mode === "all" ? "" : "active "}subscription(s)`);
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
        cell(billing(sub), "nowrap"),
        cell(sub.paymentMethod),
        cell(formatMoney(sub.currency, sub.amountCents), "num"),
        cell(sub.hasRate ? money(sub.baseCents) : "no rate", sub.hasRate ? "num" : "num muted"),
      );
      clickableRow(row, () => openEdit(sub));

      return row;
    }),
  );
}

// billing reads like "Monthly · day 18" or "Yearly · 2026-12-24".
function billing(sub) {
  const period = sub.period === "year" ? "Yearly" : sub.period === "month" ? "Monthly" : sub.period;
  const when = sub.period === "year" ? sub.paymentDate : sub.paymentDay && `day ${sub.paymentDay}`;

  return when ? `${period} · ${when}` : period;
}

// --- add -------------------------------------------------------------------

function openAdd() {
  const options = state.view?.options;
  if (!options) return;

  const form = el.addForm;
  form.reset();
  fillSelect(form.elements.currency, options.currencies);
  fillSelect(form.elements.period, options.periods, options.periods.map((p) => (p === "year" ? "Yearly" : "Monthly")));
  fillSelect(form.elements.type, options.types);
  form.elements.paymentMethod.value = options.paymentMethods[0] ?? "";
  syncPeriodFields();
  formError(form, "");
  el.addDialog.showModal();
}

// Only the payment day field matching the period is shown and sent.
function syncPeriodFields() {
  const period = el.addForm.elements.period.value;
  for (const field of el.addForm.querySelectorAll("[data-period]")) field.hidden = field.dataset.period !== period;
}

async function saveNew(event) {
  event.preventDefault();
  const form = el.addForm.elements;
  const yearly = form.period.value === "year";
  const input = {
    name: form.name.value,
    currency: form.currency.value,
    amount: form.amount.value,
    period: form.period.value,
    paymentMethod: form.paymentMethod.value,
    type: form.type.value,
    isActive: form.isActive.checked,
    paymentDate: yearly ? form.paymentDate.value : "",
    paymentDay: yearly ? "" : form.paymentDay.value,
  };

  try {
    await busy(el.addForm, () => api.CreateSubscription(input));
    el.addDialog.close();
    // The TUI shows every subscription after adding one, so a new inactive
    // one doesn't seem to vanish.
    state.mode = "all";
    await show(`saved subscription ${input.name.trim()}`);
  } catch (err) {
    formError(el.addForm, String(err));
  }
}

// --- edit & delete ---------------------------------------------------------

function openEdit(sub) {
  state.current = sub;
  const form = el.editForm.elements;

  el.editTitle.textContent = sub.name;
  el.editSubtitle.textContent = [sub.type, billing(sub), sub.currency].filter(Boolean).join(" · ");
  form.amount.value = formatAmount(sub.amountCents);
  form.paymentMethod.value = sub.paymentMethod;
  form.isActive.checked = sub.isActive;
  formError(el.editForm, "");
  el.editDialog.showModal();
}

async function saveEdit(event) {
  event.preventDefault();
  const sub = state.current;
  const form = el.editForm.elements;
  const input = { id: sub.id, amount: form.amount.value, paymentMethod: form.paymentMethod.value, isActive: form.isActive.checked };

  try {
    await busy(el.editForm, () => api.UpdateSubscription(input));
    el.editDialog.close();
    await show(`updated subscription ${sub.name}`);
  } catch (err) {
    formError(el.editForm, String(err));
  }
}

function askDelete() {
  const sub = state.current;

  confirmDelete("Delete subscription?", `“${sub.name}” will be deleted. This can't be undone.`, async () => {
    await api.DeleteSubscription(sub.id);
    el.editDialog.close();
    await show(`deleted subscription ${sub.name}`);
  });
}
