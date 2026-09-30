// Incomes & expenses → Search: entries across all months by text, kind,
// category, account, currency, dates and amount, with totals; several can
// be ticked to change their category or account together. Editing one uses
// the usual dialog (passed in by cashflow.js).
import { api } from "../api.js";
import { commentWithTags } from "../tags-input.js";
import { badge, busy, cell, fillSelect, formatMoney, formError, setStatus, signClass, signedMoney } from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  form: $("search-form"),
  clear: $("search-clear"),
  summary: $("search-summary"),
  minLabel: $("search-min-label"),
  maxLabel: $("search-max-label"),
  bulk: $("search-bulk"),
  selected: $("search-selected"),
  bulkCategory: $("bulk-category"),
  bulkCategoryApply: $("bulk-category-apply"),
  bulkAccount: $("bulk-account"),
  bulkAccountApply: $("bulk-account-apply"),
  unselect: $("search-unselect"),
  bulkNote: $("search-bulk-note"),
  bulkTag: $("bulk-tag"),
  bulkTagAdd: $("bulk-tag-add"),
  bulkTagRemove: $("bulk-tag-remove"),
  all: $("search-all"),
  rows: $("search-rows"),
  table: $("search-table"),
  empty: $("search-empty"),
  more: $("search-more"),
  baseHeading: $("search-base-heading"),
};

const state = {
  result: null,
  // Ticked entries, by id.
  picked: new Set(),
  // Filters to start the next search with (see presetSearch).
  preset: null,
  openEdit: null,
  timer: 0,
  // Each search gets a number; an answer to an older one is dropped.
  asked: 0,
};

export function initSearch({ openEdit }) {
  state.openEdit = openEdit;
  const elements = el.form.elements;

  // Typing searches after a pause; picking a filter right away.
  for (const input of [elements.text, elements.minBase, elements.maxBase]) {
    input.addEventListener("input", () => {
      clearTimeout(state.timer);
      state.timer = setTimeout(loadSearch, 250);
    });
  }

  for (const input of [elements.kind, elements.category, elements.account, elements.currency, elements.tag, elements.from, elements.to]) {
    input.addEventListener("change", loadSearch);
  }

  el.form.addEventListener("submit", (event) => {
    event.preventDefault();
    loadSearch();
  });

  el.clear.addEventListener("click", () => {
    el.form.reset();
    loadSearch();
    elements.text.focus();
  });

  el.all.addEventListener("change", () => {
    for (const entry of state.result?.entries ?? []) {
      if (el.all.checked) state.picked.add(entry.id);
      else state.picked.delete(entry.id);
    }

    render();
  });

  el.unselect.addEventListener("click", () => {
    state.picked.clear();
    render();
  });

  el.bulkCategoryApply.addEventListener("click", () => change({ setCategory: true, category: el.bulkCategory.value }, `category ${el.bulkCategory.value}`));
  el.bulkAccountApply.addEventListener("click", () => change({ setAccount: true, account: el.bulkAccount.value }, el.bulkAccount.value ? `account ${el.bulkAccount.value}` : "no account"));
  el.bulkTagAdd.addEventListener("click", () => el.bulkTag.value.trim() && change({ addTag: el.bulkTag.value }, `tag ${el.bulkTag.value.trim()}`, "tagged"));
  el.bulkTagRemove.addEventListener("click", () => el.bulkTag.value.trim() && change({ removeTag: el.bulkTag.value }, `tag ${el.bulkTag.value.trim()}`, "untagged"));
}

// presetSearch starts the next search from filters ({ tag }, say), the
// rest cleared.
export function presetSearch(filters) {
  el.form.reset();
  state.preset = filters;
}

// focusSearch puts the cursor in the search box.
export function focusSearch() {
  el.form.elements.text.focus();
  el.form.elements.text.select();
}

function input() {
  const f = el.form.elements;
  return {
    text: f.text.value,
    kind: f.kind.value,
    category: f.category.value,
    account: f.account.value,
    currency: f.currency.value,
    tag: f.tag.value,
    from: f.from.value,
    to: f.to.value,
    minBase: f.minBase.value,
    maxBase: f.maxBase.value,
  };
}

export async function loadSearch() {
  const asked = ++state.asked;
  // A preset goes in as asked even before the tag list is filled.
  const query = { ...input(), ...state.preset };
  try {
    const result = await api.CashflowSearch(query);
    if (asked !== state.asked) return;

    formError(el.form, "");
    state.result = result;
    fillOptions(result);
    if (state.preset) {
      for (const [name, value] of Object.entries(state.preset)) el.form.elements[name].value = value;
      state.preset = null;
    }

    // Ticks stay on entries still shown.
    const shown = new Set(result.entries.map((e) => e.id));
    for (const id of state.picked) if (!shown.has(id)) state.picked.delete(id);

    render();
  } catch (err) {
    if (asked === state.asked) formError(el.form, String(err));
  }
}

// fillOptions offers what can be picked, keeping what is.
function fillOptions(result) {
  const f = el.form.elements;
  const o = result.options;
  const picked = { category: f.category.value, account: f.account.value, currency: f.currency.value, tag: f.tag.value };
  const categories = [...new Set([...o.expenseCategories, ...o.incomeCategories])].sort((a, b) => a.localeCompare(b));
  fillSelect(f.category, ["", ...categories], ["Any", ...categories]);
  fillSelect(f.account, ["", ...o.accounts], ["Any", ...o.accounts]);
  fillSelect(f.currency, ["", ...o.currencies], ["Any", ...o.currencies]);
  fillSelect(f.tag, ["", ...o.tags], ["Any", ...o.tags]);
  for (const [name, value] of Object.entries(picked)) f[name].value = value;
  el.minLabel.textContent = `At least, ${result.baseCurrency}`;
  el.maxLabel.textContent = `At most, ${result.baseCurrency}`;
}

function render() {
  const r = state.result;
  if (!r) return;
  const money = (cents) => formatMoney(r.baseCurrency, cents);

  const figures = [
    ["Found", String(r.total), ""],
    ["Income", money(r.incomeCents), r.incomeCents ? "positive" : ""],
    ["Expenses", money(r.expenseCents), ""],
    ["Net", money(r.netCents), signClass(r.netCents)],
  ];
  el.summary.replaceChildren(
    ...figures.map(([label, value, className]) => {
      const box = document.createElement("div");
      const l = document.createElement("div");
      l.className = "label";
      l.textContent = label;
      const v = document.createElement("div");
      v.className = `figure ${className}`.trim();
      v.textContent = value;
      box.append(l, v);
      return box;
    }),
  );

  el.baseHeading.textContent = `In ${r.baseCurrency}`;
  el.table.hidden = r.entries.length === 0;
  el.empty.hidden = r.entries.length > 0;
  const notes = [];
  if (r.total > r.entries.length) notes.push(`Showing the newest ${r.entries.length} of ${r.total}; the totals count them all.`);
  if (r.missingRates) notes.push(`${r.missingRates} match(es) have no rate and aren't in the totals.`);
  el.more.hidden = notes.length === 0;
  el.more.textContent = notes.join(" ");

  el.rows.replaceChildren(...r.entries.map((entry) => row(entry, r.baseCurrency)));
  el.all.checked = r.entries.length > 0 && r.entries.every((e) => state.picked.has(e.id));
  renderBulk();
}

function row(entry, baseCurrency) {
  const tr = document.createElement("tr");
  // Like the month's table: "+$ 10.00" in, "-$ 10.00" out.
  const money = (currency, cents) => (entry.isIncome ? signedMoney(currency, cents) : formatMoney(currency, -cents));

  const box = document.createElement("input");
  box.type = "checkbox";
  box.checked = state.picked.has(entry.id);
  box.setAttribute("aria-label", `Select ${entry.date} ${entry.category}`);
  box.addEventListener("click", (event) => event.stopPropagation());
  box.addEventListener("change", () => {
    if (box.checked) state.picked.add(entry.id);
    else state.picked.delete(entry.id);
    tr.classList.toggle("selected", box.checked);
    el.all.checked = state.result.entries.every((e) => state.picked.has(e.id));
    renderBulk();
  });

  const category = document.createElement("span");
  category.append(entry.category || "—");
  if (entry.categoryArchived) category.append(badge("archived"));

  tr.append(
    cell(box, "check-cell"),
    cell(entry.date, "nowrap"),
    cell(category),
    cell(entry.account || "—", entry.account ? "" : "muted"),
    cell(commentWithTags(entry), "muted"),
    cell(money(entry.currency, entry.amountCents), `num ${entry.isIncome ? "positive" : ""}`),
    cell(entry.hasRate ? money(baseCurrency, entry.baseCents) : "no rate", `num ${entry.hasRate ? (entry.isIncome ? "positive" : "") : "muted"}`),
  );
  tr.classList.toggle("selected", state.picked.has(entry.id));
  tr.addEventListener("click", () => state.openEdit(entry));
  return tr;
}

// renderBulk shows what can be done to the ticked entries: a category of
// their kind (one kind only), an account.
function renderBulk() {
  const r = state.result;
  const picked = (r?.entries ?? []).filter((e) => state.picked.has(e.id));
  el.bulk.hidden = picked.length === 0;
  if (picked.length === 0) return;

  el.selected.textContent = `${picked.length} selected`;
  const kinds = new Set(picked.map((e) => e.isIncome));
  const mixed = kinds.size > 1;
  const categories = mixed ? [] : picked[0].isIncome ? r.options.incomeCategories : r.options.expenseCategories;
  const keep = el.bulkCategory.value;
  fillSelect(el.bulkCategory, categories);
  if (categories.includes(keep)) el.bulkCategory.value = keep;
  el.bulkCategory.disabled = el.bulkCategoryApply.disabled = mixed || categories.length === 0;

  const account = el.bulkAccount.value;
  fillSelect(el.bulkAccount, ["", ...r.options.accounts], ["— none —", ...r.options.accounts]);
  el.bulkAccount.value = account;

  el.bulkNote.hidden = !mixed;
  el.bulkNote.textContent = "Incomes and expenses are both ticked; their categories differ, so change them one kind at a time.";
}

async function change(what, label, verb = "moved") {
  const ids = [...state.picked];
  try {
    const n = await busy(el.bulk, () => api.ChangeCashflows({ ids, ...what }));
    state.picked.clear();
    el.bulkTag.value = "";
    await loadSearch();
    const entries = `${n} entr${n === 1 ? "y" : "ies"}`;
    setStatus(verb === "moved" ? `moved ${entries} to ${label}` : `${verb} ${entries}: ${label}`);
  } catch (err) {
    setStatus(`Change failed: ${err}`);
    el.bulkNote.hidden = false;
    el.bulkNote.textContent = String(err);
  }
}
