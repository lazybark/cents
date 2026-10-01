// Incomes & expenses by category: by default the largest categories of the
// chosen months as lines in one chart, or one picked category on its own
// with its figures. Part of the incomes & expenses view.
import { api } from "../api.js";
import { chart, legend, monthLabel } from "../charts.js";
import { cell, clickableRow, fillSelect, formatMoney } from "../ui.js";

const TOP = 10;

const $ = (id) => document.getElementById(id);

const el = {
  title: $("cat-title"),
  kind: $("cat-kind"),
  pick: $("cat-pick"),
  range: $("cat-range"),
  chart: $("cat-chart"),
  legend: $("cat-legend"),
  empty: $("cat-empty"),
  note: $("cat-note"),
  figures: $("cat-figures-panel"),
  total: $("cat-total"),
  average: $("cat-average"),
  high: $("cat-high"),
  highMonth: $("cat-high-month"),
  shareLabel: $("cat-share-label"),
  share: $("cat-share"),
  more: $("cat-more"),
  tablePanel: $("cat-table-panel"),
  rows: $("cat-rows"),
};

const state = {
  data: null,
  // The category shown on its own; empty for the largest ones together.
  picked: "",
};

export function initCategories() {
  el.kind.addEventListener("change", () => {
    state.picked = "";
    loadCategories();
  });
  el.range.addEventListener("change", () => render());
  el.pick.addEventListener("change", () => {
    state.picked = el.pick.value;
    render();
  });
}

export async function loadCategories() {
  state.data = await api.CashflowCategories(el.kind.value);
  render();
}

function pick(name) {
  state.picked = name;
  render();
  el.chart.scrollIntoView({ block: "nearest" });
}

// inRange cuts the months down to the chosen range and ranks categories by
// their total in it; categories with nothing in it are left out.
function inRange(data) {
  const range = Number(el.range.value);
  const start = range > 0 ? Math.max(0, data.months.length - range) : 0;
  const months = data.months.slice(start);
  const categories = data.categories
    .map((c) => {
      const values = c.values.slice(start);
      const entries = c.entries.slice(start);
      return { name: c.name, values, entries, total: values.reduce((a, b) => a + b, 0), count: entries.reduce((a, b) => a + b, 0) };
    })
    .filter((c) => c.count > 0)
    .sort((a, b) => b.total - a.total || a.name.localeCompare(b.name));

  return { months, categories };
}

function render() {
  const data = state.data;
  if (!data) return;

  const base = data.baseCurrency;
  const money = (cents) => formatMoney(base, cents);
  const kind = data.isIncome ? "incomes" : "expenses";
  const { months, categories } = inRange(data);
  const all = categories.reduce((sum, c) => sum + c.total, 0);
  const top = categories.slice(0, TOP);
  // Colors follow the ranking, so a category keeps its color in the table.
  const color = new Map(top.map((c, i) => [c.name, `series-${i}`]));

  if (state.picked && !categories.some((c) => c.name === state.picked)) state.picked = "";
  fillSelect(
    el.pick,
    ["", ...categories.map((c) => c.name)],
    [categories.length > TOP ? `Largest ${TOP} together` : "All categories", ...categories.map((c) => c.name)],
  );
  el.pick.value = state.picked;

  const empty = categories.length === 0;
  el.empty.hidden = !empty;
  el.empty.textContent = `No ${kind} in these months.`;
  el.tablePanel.hidden = empty;
  el.figures.hidden = empty || !state.picked;
  el.note.textContent = data.missingRates > 0 ? `${data.missingRates} record(s) have no rate and aren't counted. Open their month to add one.` : "";
  el.note.hidden = data.missingRates === 0;

  if (empty) {
    el.title.textContent = `${data.isIncome ? "Incomes" : "Expenses"} by category`;
    el.chart.replaceChildren();
    el.legend.replaceChildren();
    el.rows.replaceChildren();
    return;
  }

  if (state.picked) renderOne(categories.find((c) => c.name === state.picked), months, all, money, kind, data.isIncome);
  else renderTop(top, months, money, kind, color, base);

  el.rows.replaceChildren(
    ...categories.map((c) => {
      const row = document.createElement("tr");
      const name = document.createElement("span");
      if (color.has(c.name)) {
        const dot = document.createElement("span");
        dot.className = `series-dot ${color.get(c.name)}`;
        name.append(dot);
      }
      name.append(c.name);

      row.append(
        cell(name),
        cell(money(c.total), "num"),
        cell(all > 0 ? `${((c.total / all) * 100).toFixed(1)}%` : "—", "num"),
        cell(money(Math.round(c.total / months.length)), "num"),
        cell(String(c.count), "num"),
      );
      if (c.name === state.picked) row.classList.add("selected");
      clickableRow(row, () => pick(c.name === state.picked ? "" : c.name));

      return row;
    }),
  );
}

function renderTop(top, months, money, kind, color, base) {
  el.title.textContent = `Largest ${kind} by month`;

  chart(el.chart, {
    months,
    series: top.map((c) => ({ type: "line", values: c.values, className: color.get(c.name) })),
    tooltip: (i) => [
      monthLabel(months[i], true),
      ...top
        .filter((c) => c.values[i] > 0)
        .sort((a, b) => b.values[i] - a.values[i])
        .map((c) => `${c.name}: ${money(c.values[i])}`),
    ],
    label: `Largest ${kind} by month in ${base}`,
  });
  legend(el.legend, top.map((c) => [color.get(c.name), c.name]));
}

function renderOne(category, months, all, money, kind, isIncome) {
  el.title.textContent = category.name;

  chart(el.chart, {
    months,
    series: [{ type: "bar", values: category.values, className: isIncome ? "bar-positive" : "bar-negative" }],
    tooltip: (i) => [
      monthLabel(months[i], true),
      money(category.values[i]),
      `${category.entries[i]} entr${category.entries[i] === 1 ? "y" : "ies"}`,
    ],
    label: `${category.name} by month`,
  });
  legend(el.legend, []);

  const high = category.values.reduce((best, value, i) => (value > category.values[best] ? i : best), 0);
  const active = category.entries.filter((n) => n > 0).length;
  const last = months.length - 1;

  el.total.textContent = money(category.total);
  el.average.textContent = money(Math.round(category.total / months.length));
  el.high.textContent = money(category.values[high]);
  el.highMonth.textContent = monthLabel(months[high], true);
  el.shareLabel.textContent = `Share of all ${kind}`;
  el.share.textContent = all > 0 ? `${((category.total / all) * 100).toFixed(1)}%` : "—";

  const more = [
    ["Months with entries", `${active} of ${months.length}`],
    ["Entries", String(category.count)],
    ["Average entry", money(Math.round(category.total / category.count))],
    [`In ${monthLabel(months[last], true)}`, money(category.values[last])],
  ];
  el.more.replaceChildren(
    ...more.flatMap(([term, value]) => {
      const dt = document.createElement("dt");
      dt.textContent = term;
      const dd = document.createElement("dd");
      dd.textContent = value;
      return [dt, dd];
    }),
  );
}
