// Overview: the same figures as the TUI home screen, in the base currency.
import { api } from "../api.js";
import { budgetRow } from "./budgets.js";
import { openRegularPayment } from "./regular.js";
import { badge, cell, clickableRow, formatMoney, loadedStatus, nextPaymentCell, periodLabel, setStatus, signClass, withBase } from "../ui.js";

const el = {
  netWorth: document.getElementById("net-worth"),
  financial: document.getElementById("financial"),
  obligations: document.getElementById("obligations"),
  upcoming: document.getElementById("upcoming-rows"),
  upcomingTable: document.getElementById("upcoming-table"),
  upcomingEmpty: document.getElementById("upcoming-empty"),
  unscheduled: document.getElementById("upcoming-unscheduled"),
  upcomingHint: document.getElementById("upcoming-hint"),
  budgets: document.getElementById("overview-budgets"),
  budgetsList: document.getElementById("overview-budgets-list"),
  budgetsNote: document.getElementById("overview-budgets-note"),
  budgetsSetUp: document.getElementById("overview-budgets-setup"),
};

export const title = "Overview";

export function init() {
  el.budgetsSetUp.addEventListener("click", () => document.querySelector('.nav-item[data-view="budgets"]').click());
}

export async function show() {
  try {
    const overview = await api.Overview();
    render(overview);
    loadedStatus(overview.loadedAt);
  } catch (err) {
    setStatus(`Failed to load overview: ${err}`);
  }
}

function render(o) {
  const money = (cents) => formatMoney(o.baseCurrency, cents);
  const month = new Date(o.month).toLocaleString(undefined, { month: "long" });

  el.netWorth.textContent = money(o.netWorthCents);
  el.netWorth.className = `amount ${signClass(o.netWorthCents)}`;

  renderStats(el.financial, [
    [`Monthly net (${month})`, money(o.monthlyNetCents), signClass(o.monthlyNetCents)],
    ["Monthly subscriptions", money(o.monthlySubscriptionsCents)],
    ["Yearly subscriptions", money(o.yearlySubscriptionsCents)],
    ["Total accounts", money(o.accountsCents)],
    ["Property", money(o.propertyCents)],
    ["Investments", money(o.investmentsCents)],
  ]);

  // Like the TUI, amounts the user owes turn red once they are above zero.
  const owed = (cents) => (cents > 0 ? "negative" : "");
  renderStats(el.obligations, [
    ["Regular obligations a month", money(o.monthlyObligationsCents)],
    ["Regular obligations a year", money(o.yearlyObligationsCents)],
    ["Debts owed to me", money(o.debtsToMeCents)],
    ["Debts I owe", money(o.debtsByMeCents), owed(o.debtsByMeCents)],
    ["Unpaid taxes", money(o.unpaidTaxesCents), owed(o.unpaidTaxesCents)],
    ["Credits left to pay", money(o.creditsCents), owed(o.creditsCents)],
    ["Invoices owed to me", money(o.invoicesToMeCents)],
    ["Invoices I owe", money(o.invoicesByMeCents), owed(o.invoicesByMeCents)],
    ["Goals progress", goalsProgress(o, money), "wide", progressBar(o)],
  ]);

  renderUpcoming(o);
  renderBudgets(o);
}

// renderBudgets shows this month's budgets, the ones over or close to
// their limit first; without budgets, how to set them.
function renderBudgets(o) {
  const order = { over: 0, close: 1, ok: 2 };
  const rows = [...o.budgets].sort((a, b) => order[a.state] - order[b.state]);
  const open = () => document.querySelector('.nav-item[data-view="budgets"]').click();

  el.budgetsList.replaceChildren(...rows.map((row) => budgetRow(row, o.baseCurrency, { open, compact: true })));
  el.budgetsList.hidden = rows.length === 0;
  el.budgetsSetUp.hidden = rows.length > 0;

  const over = rows.filter((r) => r.state === "over").length;
  const close = rows.filter((r) => r.state === "close").length;
  el.budgetsNote.textContent =
    rows.length === 0
      ? "No budgets yet. Set monthly limits for categories, or for all your spending, to see here how the month is going."
      : over + close === 0
        ? `${rows.length === 1 ? "The budget is" : `All ${rows.length} budgets are`} within ${rows.length === 1 ? "its limit" : "their limits"} so far. Click one to see more.`
        : "Click one to see more.";
}

// renderUpcoming lists the regular payments due soon, soonest first.
function renderUpcoming(o) {
  el.upcomingTable.hidden = o.upcoming.length === 0;
  el.upcomingEmpty.hidden = o.upcoming.length > 0;

  el.upcomingHint.hidden = o.upcoming.length === 0;

  // Obligations without a date can't come up; say which (each opens to set
  // one), so they get one.
  const missing = o.unscheduled ?? [];
  el.unscheduled.hidden = missing.length === 0;
  const names = missing.flatMap((sub, i) => {
    const link = document.createElement("button");
    link.type = "button";
    link.className = "link-button";
    link.textContent = sub.name;
    link.addEventListener("click", () => openRegularPayment(sub, show));
    return i === 0 ? [link] : [", ", link];
  });
  el.unscheduled.replaceChildren(
    ...names,
    missing.length === 1 ? " has no next payment date, so it can't show here. Click it to set one." : " have no next payment date, so they can't show here. Click one to set it.",
  );
  el.upcoming.replaceChildren(
    ...o.upcoming.map((sub) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = sub.name;
      name.append(badge(periodLabel(sub.period).toLowerCase()));
      if (sub.isObligation) name.append(badge("obligation"));

      const details = document.createElement("div");
      details.className = "account-description";
      details.textContent = [sub.type, sub.paymentMethod].filter(Boolean).join(" · ");

      const nameCell = document.createElement("div");
      nameCell.append(name, details);

      row.append(cell(nameCell), nextPaymentCell(sub), cell(withBase(sub, sub.amountCents, o.baseCurrency, sub.baseCents), "num"));
      // The same dialog as in its list; changes show here too.
      clickableRow(row, () => openRegularPayment(sub, show));
      return row;
    }),
  );
}

function goalsProgress(o, money) {
  const text = `${money(o.goalsAccumulatedCents)} / ${money(o.goalsTargetCents)}`;
  if (o.goalsTargetCents <= 0) return text;

  const percent = (o.goalsAccumulatedCents / o.goalsTargetCents) * 100;
  return `${text} (${percent.toFixed(1)}%)`;
}

function progressBar(o) {
  if (o.goalsTargetCents <= 0) return null;

  const bar = document.createElement("div");
  bar.className = "progress";
  const fill = document.createElement("div");
  fill.className = "progress-fill";
  fill.style.width = `${Math.min(100, (o.goalsAccumulatedCents / o.goalsTargetCents) * 100)}%`;
  bar.append(fill);
  return bar;
}

function renderStats(list, rows) {
  list.replaceChildren(
    ...rows.flatMap(([label, value, className, extra]) => {
      const dt = document.createElement("dt");
      dt.textContent = label;
      const dd = document.createElement("dd");
      dd.textContent = value;
      if (className) dd.className = className;
      if (extra) dd.append(extra);
      return [dt, dd];
    }),
  );
}
