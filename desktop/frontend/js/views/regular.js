// Regular payments, in two kinds listed apart: subscriptions (minor ones:
// streaming, apps) and obligations (serious ones: rent, insurance, bills).
// One module builds both views; they share the payment dialog, where a
// payment can be moved to the other kind. Every field can be edited; a next
// payment date repeats every period, payments due soon are marked, and one
// can be marked paid early to show the next.
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
  rowAction,
  setStatus,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const dialog = {
  dialog: $("subscription-dialog"),
  form: $("subscription-form"),
  title: $("subscription-title"),
  due: $("subscription-due"),
  dueText: $("subscription-due-text"),
  paid: $("subscription-paid"),
  delete: $("subscription-delete"),
  types: $("subscription-types"),
  paymentMethods: $("payment-methods"),
  payments: $("subscription-payments"),
  paymentRows: $("subscription-payment-rows"),
  paymentsEmpty: $("subscription-payments-empty"),
};

// Both views by kind, so a payment can be opened from elsewhere (the
// overview) in the right one.
const views = {};

// The dialog belongs to whichever view opened it.
const editing = {
  view: null,
  // The payment open in the dialog, or null when adding one.
  current: null,
  // Opened from elsewhere: called after a change, to show it there too.
  onChange: null,
};

let dialogWired = false;

function wireDialog() {
  if (dialogWired) return;
  dialogWired = true;

  dialog.form.addEventListener("submit", save);
  // A payment method with a currency (a card's) fills it in.
  dialog.form.elements.paymentMethod.addEventListener("change", methodPicked);
  dialog.paid.addEventListener("click", markPaid);
  dialog.delete.addEventListener("click", askDelete);
}

// regularView builds the view of one kind ("subscription" or
// "obligation"), with the words it uses.
export function regularView(config) {
  const section = $(`view-${config.view}`);
  const q = (selector) => section.querySelector(selector);
  const el = {
    add: $(config.addButton),
    tabs: section.querySelectorAll(".tab"),
    activeMonthly: q(".regular-active-monthly"),
    activeYearly: q(".regular-active-yearly"),
    inactiveMonthly: q(".regular-inactive-monthly"),
    inactiveYearly: q(".regular-inactive-yearly"),
    inactiveTotals: section.querySelectorAll(".inactive-total"),
    baseHeading: q(".regular-base-heading"),
    rows: q(".regular-rows"),
    empty: q(".regular-empty"),
    hint: q(".regular-hint"),
  };

  const view = {
    config,
    el,
    // Like the TUI menu, lists start with active payments.
    mode: "active",
    data: null,
  };

  view.init = () => {
    wireDialog();
    el.empty.textContent = config.empty;
    for (const tab of el.tabs) {
      tab.addEventListener("click", () => {
        view.mode = tab.dataset.mode;
        view.show();
      });
    }

    el.add.addEventListener("click", () => openDialog(view, null));
  };

  view.load = async () => {
    view.data = await api.Subscriptions(view.mode, config.kind);
  };

  view.show = async (message) => {
    for (const tab of el.tabs) tab.setAttribute("aria-selected", String(tab.dataset.mode === view.mode));

    try {
      await view.load();
      render(view);
      const list = view.data.subscriptions;
      const soon = list.filter((s) => s.dueSoon).length;
      setStatus(message ?? `${list.length} ${view.mode === "all" ? "" : "active "}${config.plural}${soon ? `, ${soon} due soon` : ""}`);
    } catch (err) {
      setStatus(`Failed to load ${config.plural}: ${err}`);
    }
  };

  views[config.kind] = view;
  return view;
}

// openRegularPayment opens a subscription or obligation (a row from the
// API) in the dialog from elsewhere, like the overview; onChange runs after
// it changes there.
export async function openRegularPayment(sub, onChange) {
  const view = views[sub.isObligation ? "obligation" : "subscription"];
  if (!view.data) await view.load();

  openDialog(view, sub);
  editing.onChange = onChange;
}

// changed shows a change: in the view the dialog belongs to and, when it was
// opened from elsewhere, there too (with the message last).
async function changed(message) {
  const { view, onChange } = editing;
  await view.show(message);

  if (onChange) {
    await onChange();
    setStatus(message);
  }
}

function render(view) {
  const { data, el } = view;
  const money = (cents) => formatMoney(data.baseCurrency, cents);

  el.activeMonthly.textContent = money(data.active.monthlyCents);
  el.activeYearly.textContent = money(data.active.yearlyCents);
  el.inactiveMonthly.textContent = money(data.inactive.monthlyCents);
  el.inactiveYearly.textContent = money(data.inactive.yearlyCents);
  for (const total of el.inactiveTotals) total.hidden = data.mode !== "all";

  el.baseHeading.textContent = `In ${data.baseCurrency}`;
  el.empty.hidden = data.subscriptions.length > 0;
  el.hint.hidden = data.subscriptions.length === 0;

  el.rows.replaceChildren(
    ...data.subscriptions.map((sub) => {
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
      clickableRow(row, () => openDialog(view, sub));

      return row;
    }),
  );
}

// --- add and edit ----------------------------------------------------------

function openDialog(view, sub) {
  const options = view.data?.options;
  if (!options) return;

  editing.view = view;
  editing.current = sub;
  editing.onChange = null;
  const form = dialog.form;
  const fields = form.elements;
  form.reset();

  dialog.types.replaceChildren(...options.types.map((name) => new Option(name)));
  dialog.paymentMethods.replaceChildren(...options.paymentMethods.map((name) => new Option(name)));

  // An edited payment keeps its currency even if it left settings.
  const currencies = sub && !options.currencies.includes(sub.currency) ? [...options.currencies, sub.currency] : options.currencies;
  fillSelect(fields.currency, currencies);
  fillSelect(fields.period, options.periods, options.periods.map(periodLabel));
  fields.name.placeholder = view.config.namePlaceholder;

  if (sub) {
    dialog.title.textContent = sub.name;
    fields.name.value = sub.name;
    fields.type.value = sub.type;
    fields.amount.value = formatAmount(sub.amountCents);
    fields.currency.value = sub.currency;
    fields.period.value = sub.period;
    fields.nextPayment.value = sub.anchor;
    fields.paymentMethod.value = sub.paymentMethod;
    fields.isActive.checked = sub.isActive;
    fields.isObligation.checked = sub.isObligation;
    fields.paidManually.checked = sub.paidManually;
  } else {
    dialog.title.textContent = view.config.addTitle;
    fields.period.value = "month";
    fields.paymentMethod.value = options.paymentMethods[0] ?? "";
    fields.isObligation.checked = view.config.kind === "obligation";
    // Obligations (rent) are usually paid by hand; subscriptions charged.
    fields.paidManually.checked = view.config.kind === "obligation";
    methodPicked();
  }

  showDue(sub);
  showPayments(sub);
  dialog.delete.hidden = !sub;
  formError(form, "");
  dialog.dialog.showModal();
}

// methodPicked sets the currency of the picked payment method, if it has
// one that can be picked.
function methodPicked() {
  const fields = dialog.form.elements;
  const options = editing.view?.data?.options;
  const currency = options?.paymentMethodCurrencies?.[fields.paymentMethod.value.trim()];

  if (currency && [...fields.currency.options].some((o) => o.value === currency)) fields.currency.value = currency;
}

// showDue says when an edited payment is due next, with a way to mark that
// payment paid.
function showDue(sub) {
  dialog.due.hidden = !sub?.nextPayment;
  if (!sub?.nextPayment) return;

  dialog.dueText.replaceChildren(`Next payment ${sub.nextPayment}`);
  const label = badge(dueText(sub.dueInDays));
  if (sub.dueSoon) label.classList.add("badge-soon");
  dialog.dueText.append(label);
}

function formInput() {
  const fields = dialog.form.elements;

  return {
    id: editing.current?.id ?? 0,
    name: fields.name.value,
    type: fields.type.value,
    currency: fields.currency.value,
    amount: fields.amount.value,
    period: fields.period.value,
    paymentMethod: fields.paymentMethod.value,
    nextPayment: fields.nextPayment.value,
    isActive: fields.isActive.checked,
    isObligation: fields.isObligation.checked,
    paidManually: fields.paidManually.checked,
  };
}

async function save(event) {
  event.preventDefault();
  const { view, current } = editing;
  const input = formInput();

  try {
    await busy(dialog.form, () => (current ? api.UpdateSubscription(input) : api.CreateSubscription(input)));
    dialog.dialog.close();
    // A new inactive one would vanish from the active list, so like the TUI
    // every payment shows after adding one.
    if (!current) view.mode = "all";

    const name = input.name.trim();
    const moved = input.isObligation !== (view.config.kind === "obligation");
    const where = moved ? `, moved to ${input.isObligation ? "Obligations" : "Subscriptions"}` : "";
    await changed(`${current ? "updated" : "saved"} ${name}${where}`);
  } catch (err) {
    formError(dialog.form, String(err));
  }
}

// markPaid marks the next payment paid and records it; the dialog stays
// open showing the one after it, and the record can be deleted again.
async function markPaid() {
  const { current } = editing;

  try {
    const updated = await busy(dialog.form, () => api.MarkSubscriptionPaid(current.id));
    await afterPayments(updated, `marked ${current.name} paid; next payment ${updated.nextPayment}`);
  } catch (err) {
    formError(dialog.form, String(err));
  }
}

async function afterPayments(updated, message) {
  editing.current = updated;
  showDue(updated);
  formError(dialog.form, "");
  await showPayments(updated);
  await changed(message);
}

// showPayments lists the payments marked paid, latest first. Each opens its
// month in incomes and expenses; deleting one rolls the schedule back.
async function showPayments(sub) {
  dialog.payments.hidden = !sub;
  dialog.paymentRows.replaceChildren();
  dialog.paymentsEmpty.hidden = true;
  if (!sub) return;

  try {
    const payments = await api.SubscriptionPayments(sub.id);
    dialog.paymentsEmpty.hidden = payments.length > 0;
    dialog.paymentRows.replaceChildren(
      ...payments.map((payment) => {
        const row = document.createElement("tr");
        const remove = rowAction("Delete", (event) => {
          event.stopPropagation();
          askDeletePayment(payment);
        });

        row.append(cell(payment.paidFor, "nowrap"), cell(payment.markedAt, "muted nowrap"), cell(remove, "num"));
        clickableRow(row, () => openMonth(payment.paidFor));
        return row;
      }),
    );
  } catch (err) {
    formError(dialog.form, `Failed to load payments: ${err}`);
  }
}

// openMonth shows the payment's month in incomes and expenses.
function openMonth(day) {
  dialog.dialog.close();
  window.dispatchEvent(new CustomEvent("cents:open-month", { detail: day.slice(0, 7) }));
}

function askDeletePayment(payment) {
  const { current } = editing;

  confirmDelete("Delete payment?", `The payment for ${payment.paidFor} won't count as paid anymore, so the schedule rolls back.`, async () => {
    const updated = await api.DeleteSubscriptionPayment(current.id, payment.id);
    await afterPayments(updated, `unmarked ${current.name}'s payment for ${payment.paidFor}; next payment ${updated.nextPayment}`);
  });
}

function askDelete() {
  const { current } = editing;

  confirmDelete(`Delete ${current.name}?`, `“${current.name}” will be deleted. This can't be undone.`, async () => {
    await api.DeleteSubscription(current.id);
    dialog.dialog.close();
    await changed(`deleted ${current.name}`);
  });
}
