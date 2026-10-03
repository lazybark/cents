// Analytics: headline figures, net worth over time, where the money goes,
// and what's coming due. Everything is in the base currency.
import { api } from "../api.js";
import { chart, legend, monthLabel, monthRange } from "../charts.js";
import { badge, busy, cell, clickableRow, confirmDelete, formatAmount, formatMoney, formError, rowAction, setStatus, signClass, signedMoney, withBase } from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  savings: $("an-savings"),
  savingsNote: $("an-savings-note"),
  runway: $("an-runway"),
  runwayNote: $("an-runway-note"),
  fixed: $("an-fixed"),
  fixedNote: $("an-fixed-note"),
  change: $("an-change"),
  changeNote: $("an-change-note"),
  averages: $("an-averages"),

  netWorthChart: $("an-networth-chart"),
  netWorthLegend: $("an-networth-legend"),
  netWorthNote: $("an-networth-note"),

  range: $("an-range"),
  monthlyChart: $("an-monthly-chart"),
  monthlyLegend: $("an-monthly-legend"),
  spending: $("an-spending"),
  spendingEmpty: $("an-spending-empty"),

  forecastTitle: $("an-forecast-title"),
  out: $("an-out"),
  in: $("an-in"),
  typical: $("an-typical"),
  forecastTable: $("an-forecast-table"),
  forecastRows: $("an-forecast-rows"),
  forecastEmpty: $("an-forecast-empty"),
  forecastMissing: $("an-forecast-missing"),

  editHistory: $("an-networth-edit"),
  historyDialog: $("networth-dialog"),
  historyForm: $("networth-form"),
  historyBase: $("networth-base"),
  historyRows: $("networth-rows"),
};

const state = {
  // The analytics last shown, for the history dialog.
  data: null,
};

const SOURCE_LABELS = { entered: "Entered", saved: "Kept by the app", estimated: "Estimated" };

// The categories listed by name; the rest are summed up as one.
const TOP_CATEGORIES = 10;

// Where each kind of forecast item is listed.
const KIND_VIEWS = { subscription: "subscriptions", obligation: "obligations", debt: "debts", credit: "credits", tax: "taxes", invoice: "invoices" };

export const title = "Analytics";

export function init() {
  el.range.addEventListener("change", () => show());
  el.editHistory.addEventListener("click", openHistory);
  el.historyForm.addEventListener("submit", saveHistory);

  const months = Array.from({ length: 12 }, (_, i) => new Date(2000, i, 1).toLocaleString(undefined, { month: "long" }));
  el.historyForm.elements.month.replaceChildren(...months.map((name, i) => new Option(name, String(i + 1).padStart(2, "0"))));
}

export async function show(message) {
  try {
    const data = await api.Analytics(Number(el.range.value));
    state.data = data;
    render(data);
    if (el.historyDialog.open) renderHistory(data);
    setStatus(message ?? `analytics for ${el.range.selectedOptions[0].textContent.toLowerCase()}`);
  } catch (err) {
    setStatus(`Failed to load analytics: ${err}`);
  }
}

function render(a) {
  const money = (cents) => formatMoney(a.baseCurrency, cents);
  renderFigures(a, money);
  renderNetWorth(a, money);
  renderMonthly(a, money);
  renderSpending(a, money);
  renderForecast(a, money);
}

// --- headline figures --------------------------------------------------------

function figure(node, text, className = "") {
  node.textContent = text;
  node.className = `figure ${className}`.trim();
}

function renderFigures(a, money) {
  const months = a.averagedMonths === 1 ? "the last full month" : `the last ${a.averagedMonths} full months`;

  if (a.hasSavingsRate) {
    figure(el.savings, `${a.savingsRate.toFixed(0)}%`, a.savingsRate < 0 ? "negative" : "positive");
    el.savingsNote.textContent = "of income kept";
  } else {
    figure(el.savings, "—", "muted");
    el.savingsNote.textContent = "needs a month with income";
  }

  if (a.hasRunway && a.accountsCents === 0) {
    figure(el.runway, "—", "muted");
    el.runwayNote.textContent = "no money in accounts counted in summaries";
  } else if (a.hasRunway) {
    figure(el.runway, runwayText(a.runwayMonths), a.runwayMonths < 3 ? "negative" : "");
    el.runwayNote.textContent = `${money(a.accountsCents)} in accounts at the usual spending`;
  } else {
    figure(el.runway, "—", "muted");
    el.runwayNote.textContent = "needs a month with expenses";
  }

  figure(el.fixed, money(a.committedCents));
  el.fixedNote.textContent = a.hasFixedShare
    ? `a month, ${a.fixedShare.toFixed(0)}% of spending, in subscriptions and obligations`
    : "a month in subscriptions and obligations";

  if (a.hasChange) {
    figure(el.change, signedMoney(a.baseCurrency, a.changeCents), signClass(a.changeCents));
    const since = a.changeSince.endsWith("-12") ? `since the end of ${a.changeSince.slice(0, 4)}` : `since ${monthLabel(a.changeSince, true)}`;
    el.changeNote.textContent = a.changeEstimated ? `in accounts, property and investments ${since}` : since;
  } else {
    figure(el.change, "—", "muted");
    el.changeNote.textContent = "shows from next month on";
  }

  el.averages.textContent =
    a.averagedMonths > 0
      ? `Going by ${months}: on average ${money(a.averageIncomeCents)} in and ${money(a.averageExpenseCents)} out a month.`
      : "Figures go by the last 12 full months of incomes and expenses; there are none yet.";
}

// runwayText reads like "8.5 months" or "3 years".
function runwayText(months) {
  if (months <= 0) return "0 months";
  if (months >= 24) return `${(months / 12).toFixed(1).replace(/\.0$/, "")} years`;
  return `${months.toFixed(1).replace(/\.0$/, "")} months`;
}

// --- net worth over time -----------------------------------------------------

function renderNetWorth(a, money) {
  const history = a.netWorthHistory;
  // Every month from the first to this one; months with nothing kept or
  // estimated stay empty.
  const months = history.length > 0 ? monthRange(history[0].month, history.at(-1).month) : [];
  const byMonth = new Map(history.map((p) => [p.month, p]));
  const points = months.map((m) => byMonth.get(m));

  // Estimated months dashed, kept months solid. They aren't joined: an
  // estimate leaves out debts and the like, so the two can be far apart.
  const estimated = points.map((p) => (p?.estimated ? p.cents : null));
  const kept = points.map((p) => (p && !p.estimated ? p.cents : null));
  const anyEstimated = history.some((p) => p.estimated);

  chart(el.netWorthChart, {
    months,
    zero: false,
    label: "Net worth by month",
    series: [
      { type: "line", values: anyEstimated ? estimated : [], className: "line-estimated" },
      { type: "line", values: kept, className: "line-value" },
    ],
    tooltip: (i) => (points[i] ? [monthLabel(months[i], true), money(points[i].cents), TOOLTIP_SOURCES[points[i].source]] : [monthLabel(months[i], true), "nothing kept"]),
  });

  const items = [["line-value", "Net worth"]];
  if (anyEstimated) items.push(["line-estimated", "Estimated: accounts, property and investments only"]);
  legend(el.netWorthLegend, items);

  el.netWorthNote.textContent = anyEstimated
    ? "Net worth is kept once a month from now on. Earlier months are estimated from the value history of accounts, property and investments at today's rates, leaving out debts, credits, taxes and invoices; accounts ignored in summaries aren't counted. Use Edit history to enter the real values."
    : "Net worth is kept once a month, whenever the app is opened. Use Edit history to enter earlier months or correct one.";
}

const TOOLTIP_SOURCES = {
  entered: "entered by you",
  saved: "net worth",
  estimated: "estimated from account and asset values",
};

// --- net worth history dialog --------------------------------------------------

function openHistory() {
  const data = state.data;
  if (!data) return;

  el.historyForm.reset();
  formError(el.historyForm, "");
  // Start on the month before the oldest one, the next to backfill.
  const oldest = data.netWorthHistory[0]?.month;
  fillMonth(oldest ? shiftMonth(oldest, -1) : data.netWorthHistory.at(-1)?.month, "");
  el.historyBase.textContent = data.baseCurrency;
  renderHistory(data);
  el.historyDialog.showModal();
  el.historyForm.elements.amount.focus();
}

// shiftMonth moves a "YYYY-MM" month by delta months.
function shiftMonth(month, delta) {
  const [year, index] = month.split("-").map(Number);
  const date = new Date(year, index - 1 + delta, 1);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function fillMonth(month, amount) {
  const form = el.historyForm.elements;
  const now = new Date();
  const [year, index] = (month ?? `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`).split("-");
  form.month.value = index;
  form.year.value = year;
  form.amount.value = amount;
}

// renderHistory lists every month, newest first: what's kept for it and
// where that comes from. A row fills the form in to change it.
function renderHistory(data) {
  const money = (cents) => formatMoney(data.baseCurrency, cents);
  const thisMonth = data.netWorthHistory.at(-1)?.month;

  el.historyRows.replaceChildren(
    ...data.netWorthHistory
      .slice()
      .reverse()
      .map((point) => {
        const row = document.createElement("tr");
        // The app keeps this month on every load, so deleting it does nothing.
        const deletable = point.source === "entered" || (point.source === "saved" && point.month !== thisMonth);
        const source = point.source === "saved" && point.month === thisMonth ? "Kept by the app, this month" : SOURCE_LABELS[point.source];

        row.append(
          cell(monthLabel(point.month, true), "nowrap"),
          cell(money(point.cents), `num ${signClass(point.cents)}`),
          cell(source, point.source === "estimated" ? "muted" : ""),
          cell(deletable ? rowAction("Delete", (event) => {
            event.stopPropagation();
            askDeleteMonth(point);
          }) : "", "num"),
        );
        clickableRow(row, () => {
          fillMonth(point.month, formatAmount(point.cents));
          formError(el.historyForm, "");
          el.historyForm.elements.amount.focus();
          el.historyForm.elements.amount.select();
        });
        return row;
      }),
  );
}

async function saveHistory(event) {
  event.preventDefault();
  const form = el.historyForm.elements;
  const input = { month: `${form.year.value.trim()}-${form.month.value}`, amount: form.amount.value };

  try {
    await busy(el.historyForm, () => api.SetNetWorth(input));
    formError(el.historyForm, "");
    await show(`saved net worth for ${monthLabel(input.month, true)}`);
    // Next, the month before: backfilling goes back one month at a time.
    fillMonth(shiftMonth(input.month, -1), "");
    form.amount.focus();
  } catch (err) {
    formError(el.historyForm, String(err));
  }
}

function askDeleteMonth(point) {
  const label = monthLabel(point.month, true);
  const after = point.month === state.data.netWorthHistory.at(-1)?.month ? "the value the app keeps" : "an estimate, or nothing if there's none";

  confirmDelete("Delete this month's net worth?", `${formatMoney(state.data.baseCurrency, point.cents)} for ${label} will be deleted, and the month goes back to ${after}.`, async () => {
    await api.DeleteNetWorth(point.month);
    await show(`deleted net worth for ${label}`);
  });
}

// --- where the money goes ----------------------------------------------------

function renderMonthly(a, money) {
  const months = a.monthly.map((m) => m.month);
  chart(el.monthlyChart, {
    months,
    label: "Expenses by month",
    series: [
      { type: "bar", values: a.monthly.map((m) => m.expenseCents), className: "bar-negative" },
      { type: "line", values: a.monthly.map((m) => m.incomeCents), className: "line-income" },
      { type: "line", values: months.map(() => a.committedCents), className: "line-committed" },
    ],
    tooltip: (i) => {
      const m = a.monthly[i];
      return [monthLabel(m.month, true), `out ${money(m.expenseCents)}`, `in ${money(m.incomeCents)}`, `subscriptions and obligations ${money(a.committedCents)}`];
    },
  });

  legend(el.monthlyLegend, [
    ["bar-negative", "Expenses"],
    ["line-income", "Income"],
    ["line-committed", "Subscriptions and obligations a month"],
  ]);
}

function renderSpending(a, money) {
  const top = a.spending.slice(0, TOP_CATEGORIES);
  const rest = a.spending.slice(TOP_CATEGORIES);
  if (rest.length > 0) {
    top.push({
      category: `${rest.length} more`,
      cents: rest.reduce((sum, c) => sum + c.cents, 0),
      share: rest.reduce((sum, c) => sum + c.share, 0),
    });
  }

  el.spendingEmpty.hidden = top.length > 0;
  el.spending.hidden = top.length === 0;

  // Bars are scaled to the largest category, so small ones stay visible.
  const largest = Math.max(...top.map((c) => c.share), 1);
  el.spending.replaceChildren(
    ...top.flatMap((c) => {
      const name = document.createElement("div");
      name.textContent = c.category || "(no category)";

      const bar = document.createElement("div");
      bar.className = "share-bar";
      const fill = document.createElement("div");
      fill.style.width = `${(c.share / largest) * 100}%`;
      bar.append(fill);

      const amount = document.createElement("div");
      amount.className = "num";
      amount.textContent = money(c.cents);

      const share = document.createElement("div");
      share.className = "num muted";
      share.textContent = `${c.share.toFixed(c.share < 10 ? 1 : 0)}%`;

      return [name, bar, amount, share];
    }),
  );
}

// --- coming up -------------------------------------------------------------

function renderForecast(a, money) {
  el.forecastTitle.textContent = `Next ${a.forecastDays} days`;
  figure(el.out, money(a.forecastOutCents), a.forecastOutCents > 0 ? "negative" : "");
  figure(el.in, money(a.forecastInCents), a.forecastInCents > 0 ? "positive" : "");
  figure(el.typical, money(a.typicalIncomeCents));

  el.forecastTable.hidden = a.forecast.length === 0;
  el.forecastEmpty.hidden = a.forecast.length > 0;
  el.forecastMissing.hidden = !a.forecastMissing;
  el.forecastMissing.textContent =
    a.forecastMissing === 1 ? "One has no rate to the base currency and isn't in the totals." : `${a.forecastMissing} have no rate to the base currency and aren't in the totals.`;

  el.forecastRows.replaceChildren(
    ...a.forecast.map((item) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = item.name;
      name.append(badge(item.kind));
      if (item.overdue) name.append(badge("overdue"));

      const record = { currency: item.currency, isBase: item.isBase, hasRate: item.hasRate };
      const signed = (currency, cents) => (item.incoming ? signedMoney(currency, cents) : formatMoney(currency, -cents));

      row.append(
        cell(item.date, item.overdue ? "nowrap negative" : "nowrap"),
        cell(name),
        cell(withBase(record, item.cents, a.baseCurrency, item.baseCents, signed), `num ${item.incoming ? "positive" : ""}`),
      );
      clickableRow(row, () => window.dispatchEvent(new CustomEvent("cents:open-view", { detail: KIND_VIEWS[item.kind] })));
      return row;
    }),
  );
}
