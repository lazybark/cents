// Analytics, in tabs: Summary (headline figures, net worth over time and
// what's coming due), Spending, Currencies, and Assets & goals. Money is in
// the base currency unless said otherwise. The range picker applies to the
// month-by-month charts outside Summary.
import { api } from "../api.js";
import { chart, legend, monthLabel, monthRange } from "../charts.js";
import { noteLine } from "../notes.js";
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

  tabs: document.querySelectorAll("#an-tabs .tab"),
  rangeField: $("an-range-field"),
  commitments: $("an-commitments"),
  commitmentsNote: $("an-commitments-note"),
  rateChart: $("an-rate-chart"),
  rateLegend: $("an-rate-legend"),
  usualTabs: document.querySelectorAll("#an-usual-tabs .tab"),
  usualSummary: $("an-usual-summary"),
  usualNote: $("an-usual-note"),
  usualTable: $("an-usual-table"),
  usualMonthHead: $("an-usual-month-head"),
  usualRows: $("an-usual-rows"),
  usualEmpty: $("an-usual-empty"),
  exposure: $("an-exposure"),
  exposureTable: $("an-exposure-table"),
  exposureRows: $("an-exposure-rows"),
  exposureEmpty: $("an-exposure-empty"),
  exposureMissing: $("an-exposure-missing"),
  fxTotal: $("an-fx-total"),
  fxSince: $("an-fx-since"),
  fxChart: $("an-fx-chart"),
  fxCurrencies: $("an-fx-currencies"),
  fxMissing: $("an-fx-missing"),
  fxNone: $("an-fx-none"),
  goalsTable: $("an-goals-table"),
  goalsRows: $("an-goals-rows"),
  goalsEmpty: $("an-goals-empty"),

  editHistory: $("an-networth-edit"),
  historyDialog: $("networth-dialog"),
  historyForm: $("networth-form"),
  historyBase: $("networth-base"),
  historyRows: $("networth-rows"),
};

const state = {
  // The analytics last shown, for the history dialog.
  data: null,
  tab: "summary",
  // Which month "Compared with your usual" shows: this or last.
  usual: "this",
};

// The Summary tab doesn't go by the range.
const RANGED_TABS = new Set(["spending", "currencies", "assets"]);

const SOURCE_LABELS = { entered: "Entered", saved: "Kept by the app", estimated: "Estimated" };

// The categories listed by name; the rest are summed up as one.
const TOP_CATEGORIES = 10;

// Where each kind of forecast item is listed.
const KIND_VIEWS = { subscription: "subscriptions", obligation: "obligations", debt: "debts", credit: "credits", tax: "taxes", invoice: "invoices" };

export const title = "Analytics";

export function init() {
  el.range.addEventListener("change", () => show());
  for (const tab of el.tabs) {
    tab.addEventListener("click", () => {
      state.tab = tab.dataset.tab;
      showTab();
    });
  }

  for (const tab of el.usualTabs) {
    tab.addEventListener("click", () => {
      state.usual = tab.dataset.which;
      if (state.data) renderUsual(state.data, (cents) => formatMoney(state.data.baseCurrency, cents));
    });
  }

  showTab();
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
  renderForecast(a, money);
  renderSavingsRate(a, money);
  renderMonthly(a, money);
  renderSpending(a, money);
  renderUsual(a, money);
  renderExposure(a, money);
  renderFX(a, money);
  renderAssets(a.investments, "investments", money);
  renderAssets(a.property, "property", money);
  renderGoals(a);
}

// showTab shows the picked tab, with the range picker where it applies.
// Charts are drawn while their tab is hidden; they're sized by viewBox, so
// they show right away.
function showTab() {
  for (const tab of el.tabs) {
    tab.setAttribute("aria-selected", String(tab.dataset.tab === state.tab));
    $(`an-pane-${tab.dataset.tab}`).hidden = tab.dataset.tab !== state.tab;
  }

  el.rangeField.hidden = !RANGED_TABS.has(state.tab);
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

  const c = a.commitments;
  if (c.hasShare) {
    figure(el.commitments, `${c.share.toFixed(0)}%`, c.share >= 40 ? "negative" : "");
    el.commitmentsNote.textContent = `of income: ${money(c.obligationsCents)} obligations + ${money(c.creditPaymentsCents)} credit payments a month`;
  } else {
    figure(el.commitments, money(c.totalCents));
    el.commitmentsNote.textContent = "a month in obligations and credit payments; needs a month with income for the share";
  }
  el.commitmentsNote.title = "Credit payments are the average logged over the last 6 full months. A loan also kept as an obligation counts twice.";

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
    stacked: true,
    label: "Fixed and flexible spending by month",
    series: [
      { type: "bar", values: a.monthly.map((m) => m.fixedCents), className: "bar-fixed" },
      { type: "bar", values: a.monthly.map((m) => m.flexibleCents), className: "bar-negative" },
      { type: "line", values: a.monthly.map((m) => m.incomeCents), className: "line-income" },
    ],
    tooltip: (i) => {
      const m = a.monthly[i];
      return [monthLabel(m.month, true), `out ${money(m.expenseCents)}`, `fixed ${money(m.fixedCents)}`, `flexible ${money(m.flexibleCents)}`, `in ${money(m.incomeCents)}`];
    },
  });

  legend(el.monthlyLegend, [
    ["bar-fixed", "Fixed: subscriptions and obligations"],
    ["bar-negative", "Flexible"],
    ["line-income", "Income"],
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

// --- spending ----------------------------------------------------------------

// The savings rate chart stops here, so a month with little income doesn't
// squash the rest.
const RATE_FLOOR = -100;

function renderSavingsRate(a, money) {
  const months = a.monthly.map((m) => m.month);
  chart(el.rateChart, {
    months,
    label: "Savings rate by month",
    axis: (value) => `${value}%`,
    series: [{ type: "bar", values: a.monthly.map((m) => (m.hasSavingsRate ? Math.max(Math.round(m.savingsRate), RATE_FLOOR) : null)) }],
    tooltip: (i) => {
      const m = a.monthly[i];
      const rate = m.hasSavingsRate ? `${m.savingsRate.toFixed(0)}% kept` : "no income";
      return [monthLabel(m.month, true), rate, `in ${money(m.incomeCents)}`, `out ${money(m.expenseCents)}`];
    },
  });

  legend(el.rateLegend, [["bar-positive", "Kept (green) or overspent (red), as a part of income"]]);
}

function renderUsual(a, money) {
  const c = state.usual === "this" ? a.thisMonth : a.lastMonth;
  for (const tab of el.usualTabs) tab.setAttribute("aria-selected", String(tab.dataset.which === state.usual));
  el.usualMonthHead.textContent = state.usual === "this" ? "This month" : monthLabel(c.month, true);
  // An unusual month is where a note explains most.
  el.usualNote.replaceChildren(noteLine(c.month));

  const sofar = state.usual === "this" ? `, ${c.daysIn} of ${c.daysInMonth} days in (only spending above usual is marked until the month is over)` : "";
  const unusual = c.items.filter((item) => item.unusual).length;
  el.usualSummary.textContent =
    c.usualMonths === 0
      ? `${monthLabel(c.month, true)}: nothing before it to compare with yet.`
      : `${monthLabel(c.month, true)}${sofar}: ${money(c.totalCents)} spent against a usual ${money(c.usualTotalCents)}` +
        (unusual > 0 ? `; ${unusual === 1 ? "one category stands" : `${unusual} categories stand`} out.` : ".");

  el.usualTable.hidden = c.items.length === 0;
  el.usualEmpty.hidden = c.items.length > 0;
  el.usualEmpty.textContent = "No expenses in that month or the months before.";

  el.usualRows.replaceChildren(
    ...c.items.map((item) => {
      const row = document.createElement("tr");
      const name = document.createElement("span");
      name.textContent = item.category || "(no category)";
      if (item.unusual) name.append(badge(item.usualCents === 0 ? "new" : item.diffCents > 0 ? "more than usual" : "less than usual"));

      let diff = c.usualMonths === 0 ? "—" : signedMoney(a.baseCurrency, item.diffCents);
      if (item.usualCents > 0) diff += ` (${item.diffPercent > 0 ? "+" : ""}${item.diffPercent.toFixed(0)}%)`;
      // Spending more than usual is the one to watch.
      const tone = item.unusual ? (item.diffCents > 0 ? "negative" : "positive") : "";

      row.append(
        cell(name),
        cell(money(item.cents), "num"),
        cell(c.usualMonths === 0 ? "—" : money(item.usualCents), "num muted"),
        cell(diff, `num ${tone}`),
      );
      return row;
    }),
  );
}

// --- currencies --------------------------------------------------------------

function renderExposure(a, money) {
  const rows = a.exposure;
  const owned = rows.filter((e) => e.hasRate && e.ownedBaseCents > 0);
  el.exposureEmpty.hidden = rows.length > 0;
  el.exposureTable.hidden = rows.length === 0;
  el.exposure.hidden = owned.length === 0;

  const largest = Math.max(...owned.map((e) => e.share), 1);
  el.exposure.replaceChildren(
    ...owned.flatMap((e) => {
      const name = document.createElement("div");
      name.textContent = e.currency;

      const bar = document.createElement("div");
      bar.className = "share-bar share-bar-neutral";
      const fill = document.createElement("div");
      fill.style.width = `${(e.share / largest) * 100}%`;
      bar.append(fill);

      const amount = document.createElement("div");
      amount.className = "num";
      amount.textContent = money(e.ownedBaseCents);

      const share = document.createElement("div");
      share.className = "num muted";
      share.textContent = `${e.share.toFixed(e.share < 10 ? 1 : 0)}%`;

      return [name, bar, amount, share];
    }),
  );

  // Each amount in its currency, with the base amount below for others.
  const both = (e, cents, baseCents) => withBase({ currency: e.currency, isBase: e.isBase, hasRate: e.hasRate }, cents, a.baseCurrency, baseCents);
  el.exposureRows.replaceChildren(
    ...rows.map((e) => {
      const row = document.createElement("tr");
      row.append(
        cell(e.currency),
        cell(both(e, e.ownedCents, e.ownedBaseCents), "num"),
        cell(formatMoney(e.currency, e.owedToMeCents), `num ${e.owedToMeCents ? "" : "muted"}`),
        cell(formatMoney(e.currency, e.owedByMeCents), `num ${e.owedByMeCents ? "negative" : "muted"}`),
        cell(both(e, e.netCents, e.netBaseCents), `num ${signClass(e.netCents)}`),
      );
      return row;
    }),
  );

  const missing = rows.filter((e) => !e.hasRate).map((e) => e.currency);
  el.exposureMissing.hidden = missing.length === 0;
  el.exposureMissing.textContent = `${missing.join(", ")} ${missing.length === 1 ? "has" : "have"} no rate to the base currency, so ${missing.length === 1 ? "it isn't" : "they aren't"} in the shares.`;
}

function renderFX(a, money) {
  const fx = a.fx;
  const months = fx.months.map((m) => m.month);
  figure(el.fxTotal, signedMoney(a.baseCurrency, fx.totalCents), signClass(fx.totalCents));
  el.fxSince.textContent = months.length > 0 ? `since the start of ${monthLabel(months[0], true)}` : "";

  // Nothing moved: say so rather than draw a flat line.
  const moved = fx.months.some((m) => m.cents !== 0);
  el.fxChart.hidden = !moved;
  el.fxNone.hidden = moved;

  chart(el.fxChart, {
    months,
    label: "Effect of exchange rates by month",
    series: [{ type: "bar", values: fx.months.map((m) => m.cents) }],
    tooltip: (i) => [monthLabel(months[i], true), `${signedMoney(a.baseCurrency, fx.months[i].cents)} from rates`],
  });

  el.fxCurrencies.replaceChildren(
    ...fx.byCurrency.flatMap((c) => {
      const dt = document.createElement("dt");
      dt.textContent = c.currency;
      const dd = document.createElement("dd");
      dd.textContent = signedMoney(a.baseCurrency, c.cents);
      dd.className = signClass(c.cents);
      return [dt, dd];
    }),
  );

  el.fxMissing.hidden = fx.missing.length === 0;
  el.fxMissing.textContent = `No rate was known for ${fx.missing.join(", ")} in some of these months, so those months leave ${fx.missing.length === 1 ? "it" : "them"} out.`;
}

// --- assets and goals ----------------------------------------------------------

function renderAssets(view, kind, money) {
  const chartEl = $(`an-${kind}-chart`);
  const figures = $(`an-${kind}-figures`);
  const legendEl = $(`an-${kind}-legend`);
  const empty = view.count === 0;
  $(`an-${kind}-empty`).hidden = !empty;
  for (const node of [chartEl, figures, legendEl]) node.hidden = empty;
  if (empty) return;

  // The change needs a value in an earlier month to go from.
  const last = view.months.at(-1);
  const first = view.months.find((m) => m.valueCents > 0 && m !== last);
  const change = first ? view.valueCents - first.valueCents : 0;
  const gainPercent = view.costCents > 0 ? ` (${view.gainCents > 0 ? "+" : ""}${((view.gainCents / view.costCents) * 100).toFixed(1)}%)` : "";
  const items = [
    ["Value now", money(view.valueCents), ""],
    [first ? `Change since ${monthLabel(first.month)}` : "Change", first ? signedMoney(state.data.baseCurrency, change) : "—", first ? signClass(change) : "muted"],
    ["Gain over cost", view.hasCost ? signedMoney(state.data.baseCurrency, view.gainCents) + gainPercent : "no cost given", view.hasCost ? signClass(view.gainCents) : "muted"],
  ];
  figures.replaceChildren(
    ...items.map(([label, value, className]) => {
      const box = document.createElement("div");
      const labelEl = document.createElement("div");
      labelEl.className = "label";
      labelEl.textContent = label;
      const valueEl = document.createElement("div");
      valueEl.className = `figure ${className}`.trim();
      valueEl.textContent = value;
      box.append(labelEl, valueEl);
      return box;
    }),
  );

  const months = view.months.map((m) => m.month);
  const known = (m) => m.valueCents !== 0 || m.costCents !== 0;
  chart(chartEl, {
    months,
    zero: false,
    label: `${kind} value by month`,
    series: [
      { type: "line", values: view.months.map((m) => (known(m) ? m.valueCents : null)), className: "line-value" },
      { type: "line", values: view.months.map((m) => (known(m) && m.hasCost ? m.costCents : null)), className: "series-1" },
    ],
    tooltip: (i) => {
      const m = view.months[i];
      if (!known(m)) return [monthLabel(m.month, true), "no value logged yet"];
      const lines = [monthLabel(m.month, true), `value ${money(m.valueCents)}`];
      if (m.hasCost) lines.push(`paid ${money(m.costCents)}`, `gain ${signedMoney(state.data.baseCurrency, m.gainCents)}`);
      return lines;
    },
  });

  legend(legendEl, [
    ["line-value", "Value"],
    ["series-1", "What was paid (those with a cost given)"],
  ]);
}

const PACE_TEXT = {
  "on-track": (g) => `on track: ${g.projected}${g.monthsLate < 0 ? `, ${-g.monthsLate} month${g.monthsLate === -1 ? "" : "s"} early` : ""}`,
  behind: (g) => `${g.projected}, ${g.monthsLate} month${g.monthsLate === 1 ? "" : "s"} after the target`,
  "no-target": (g) => `reached around ${g.projected}`,
  stalled: () => "nothing added lately",
};

function renderGoals(a) {
  el.goalsTable.hidden = a.goals.length === 0;
  el.goalsEmpty.hidden = a.goals.length > 0;

  el.goalsRows.replaceChildren(
    ...a.goals.map((g) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = g.name;
      const details = document.createElement("div");
      details.className = "account-description";
      details.textContent = g.targetDate ? `target ${g.targetDate}` : "no target date";
      const nameCell = document.createElement("div");
      nameCell.append(name, details);

      const pace = document.createElement("div");
      pace.textContent = PACE_TEXT[g.status](g);
      if (g.status === "behind" && g.neededPerMonthCents > 0) {
        const needed = document.createElement("div");
        needed.className = "account-description";
        needed.textContent = `needs ${formatMoney(g.currency, g.neededPerMonthCents)} a month to make it`;
        pace.append(needed);
      }

      const tone = g.status === "behind" || g.status === "stalled" ? "negative" : g.status === "on-track" ? "positive" : "";
      row.append(
        cell(nameCell),
        cell(formatMoney(g.currency, g.leftCents), "num"),
        cell(formatMoney(g.currency, g.perMonthCents), `num ${g.perMonthCents > 0 ? "" : "muted"}`),
        cell(pace, tone),
      );
      clickableRow(row, () => window.dispatchEvent(new CustomEvent("cents:open-view", { detail: "goals" })));
      return row;
    }),
  );
}
