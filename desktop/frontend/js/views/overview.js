// Overview: the same figures as the TUI home screen, in the base currency.
import { api } from "../api.js";
import { formatMoney, loadedStatus, setStatus, signClass } from "../ui.js";

const el = {
  netWorth: document.getElementById("net-worth"),
  financial: document.getElementById("financial"),
  obligations: document.getElementById("obligations"),
};

export const title = "Overview";

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
  ]);

  // Like the TUI, amounts the user owes turn red once they are above zero.
  const owed = (cents) => (cents > 0 ? "negative" : "");
  renderStats(el.obligations, [
    ["Debts owed to me", money(o.debtsToMeCents)],
    ["Debts I owe", money(o.debtsByMeCents), owed(o.debtsByMeCents)],
    ["Unpaid taxes", money(o.unpaidTaxesCents), owed(o.unpaidTaxesCents)],
    ["Invoices owed to me", money(o.invoicesToMeCents)],
    ["Invoices I owe", money(o.invoicesByMeCents), owed(o.invoicesByMeCents)],
    ["Goals progress", goalsProgress(o, money), "wide", progressBar(o)],
  ]);
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
