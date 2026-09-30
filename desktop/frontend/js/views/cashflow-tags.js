// Incomes & expenses → Tags: what each tag adds up to across categories,
// most recently used first; a tag opens to its categories and a way to its
// entries in Search.
import { api } from "../api.js";
import { badge, cell, clickableRow, formatMoney, setStatus } from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  table: $("tags-table"),
  rows: $("tags-rows"),
  empty: $("tags-empty"),
  unused: $("tags-unused"),
  expenseHead: $("tags-expense-head"),
  incomeHead: $("tags-income-head"),
};

const state = {
  // The tag opened, by name.
  open: "",
  showEntries: null,
};

export function initTags({ showEntries }) {
  state.showEntries = showEntries;
}

export async function loadTags() {
  try {
    render(await api.CashflowTags());
  } catch (err) {
    setStatus(`Failed to load tags: ${err}`);
  }
}

function render(view) {
  const money = (cents) => formatMoney(view.baseCurrency, cents);
  el.expenseHead.textContent = `Spent, ${view.baseCurrency}`;
  el.incomeHead.textContent = `Received, ${view.baseCurrency}`;
  el.table.hidden = view.tags.length === 0;
  el.empty.hidden = view.tags.length > 0;
  el.unused.hidden = view.unused === 0;
  el.unused.textContent = `${view.unused} tag${view.unused === 1 ? " is" : "s are"} on no entries.`;

  el.rows.replaceChildren(
    ...view.tags.flatMap((t) => {
      const row = document.createElement("tr");
      const name = document.createElement("span");
      name.className = "tag";
      name.textContent = t.name;
      const nameCell = document.createElement("span");
      nameCell.append(name);
      if (t.archived) nameCell.append(badge("archived"));

      const when = t.first === t.last ? t.first : `${t.first} – ${t.last}`;
      row.append(
        cell(nameCell),
        cell(when, "nowrap muted"),
        cell(String(t.count), "num"),
        cell(t.expenseCents ? money(t.expenseCents) : "—", t.expenseCents ? "num" : "num muted"),
        cell(t.incomeCents ? money(t.incomeCents) : "—", t.incomeCents ? "num positive" : "num muted"),
      );
      clickableRow(row, () => {
        state.open = state.open === t.name ? "" : t.name;
        render(view);
      });

      if (state.open !== t.name) return [row];
      return [row, breakdown(t, money)];
    }),
  );
}

// breakdown is a tag's categories, its net, and a way to its entries.
function breakdown(t, money) {
  const row = document.createElement("tr");
  row.className = "tag-breakdown";
  const td = document.createElement("td");
  td.colSpan = 5;

  const list = document.createElement("dl");
  list.className = "stats";
  for (const c of t.categories) {
    const dt = document.createElement("dt");
    dt.textContent = c.category || "(no category)";
    const dd = document.createElement("dd");
    dd.textContent = c.isIncome ? `+${money(c.cents)}` : money(c.cents);
    if (c.isIncome) dd.className = "positive";
    list.append(dt, dd);
  }

  const dt = document.createElement("dt");
  dt.textContent = "Net";
  const dd = document.createElement("dd");
  dd.textContent = money(t.netCents);
  dd.className = t.netCents < 0 ? "negative" : t.netCents > 0 ? "positive" : "";
  list.append(dt, dd);

  const note = document.createElement("p");
  note.className = "hint";
  note.hidden = !t.missingRates;
  note.textContent = `${t.missingRates} entr${t.missingRates === 1 ? "y has" : "ies have"} no rate and ${t.missingRates === 1 ? "isn't" : "aren't"} counted.`;

  const show = document.createElement("button");
  show.type = "button";
  show.className = "button";
  show.textContent = `Show its ${t.count} entr${t.count === 1 ? "y" : "ies"}`;
  show.addEventListener("click", () => state.showEntries(t.name));

  td.append(list, note, show);
  row.append(td);
  return row;
}
