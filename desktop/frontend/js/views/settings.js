// Settings: base currency plus every settings list the TUI can edit. Each
// list is described once in LISTS; one panel renderer and one dialog serve
// them all.
import { api } from "../api.js";
import { badge, busy, cell, clickableRow, confirmDelete, formError, setStatus } from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  base: $("settings-base"),
  editBase: $("edit-base"),
  db: $("settings-db"),
  lists: $("settings-lists"),

  dialog: $("setting-dialog"),
  form: $("setting-form"),
  title: $("setting-title"),
  fields: $("setting-fields"),
  note: $("setting-note"),
  delete: $("setting-delete"),
};

// Each list: where its rows come from, how they show, which fields the
// dialog has, and how to save. kind matches DeleteSetting on the Go side.
// warn() explains what a rename or delete does to records using the item.
const LISTS = [
  {
    kind: "currency",
    title: "Currencies",
    noun: "currency",
    items: (v) => v.currencies,
    label: (c) => c.name,
    columns: (v) => [
      ["Currency", (c) => c.name],
      [`1 unit in ${v.baseCurrency}`, (c) => `${v.baseCurrency} ${c.rateToBase}`, "num"],
      ["Used by", usedBy, "num muted"],
    ],
    fields: (v) => [
      { name: "name", label: "Name", placeholder: "EUR", maxlength: 24 },
      { name: "rate", label: `Value of 1 unit in ${v.baseCurrency}`, placeholder: "1.08", inputmode: "decimal" },
    ],
    values: (c) => ({ name: c.name, rate: String(c.rateToBase) }),
    save: (id, f) => api.SaveCurrency({ id, name: f.name, rate: f.rate }),
    warn: (c, action) =>
      c.usedBy === 0
        ? ""
        : action === "edit"
          ? `${c.usedBy} record(s) use ${c.name}. Renaming doesn't update them, so they would lose their conversion rate.`
          : `${c.usedBy} record(s) use ${c.name}. They will show “no rate” and drop out of totals.`,
  },
  {
    kind: "payment_method",
    title: "Payment methods",
    noun: "payment method",
    items: (v) => v.paymentMethods,
    label: (p) => p.name,
    columns: () => [
      ["Name", (p) => (p.isDefault ? withBadge(p.name, "default") : p.name)],
      ["Type", (p) => p.type],
      ["Used by", usedBy, "num muted"],
    ],
    fields: (v) => [
      { name: "name", label: "Name", placeholder: "Personal Visa", maxlength: 40 },
      { name: "type", label: "Type", options: v.paymentMethodTypes },
      { name: "isDefault", label: "Default for new subscriptions", checkbox: true },
    ],
    values: (p) => ({ name: p.name, type: p.type, isDefault: p.isDefault }),
    save: (id, f) => api.SavePaymentMethod({ id, name: f.name, type: f.type, isDefault: f.isDefault }),
    warn: (p, action, v) => {
      if (action === "edit") return p.usedBy ? `${p.usedBy} subscription(s) use ${p.name} and keep that name if you rename it.` : "";
      if (v.paymentMethods.length === 1) return "It's the only payment method, so “Other” will be created in its place.";
      return p.isDefault ? "Another payment method will become the default." : "";
    },
  },
  {
    kind: "tax_type",
    title: "Tax types",
    noun: "tax type",
    items: (v) => v.taxTypes,
    label: (t) => `${t.country} / ${t.name}`,
    columns: () => [
      ["Country", (t) => t.country],
      ["Name", (t) => t.name],
      ["Description", (t) => t.description || "—", "muted wrap"],
      ["URL", (t) => t.url || "—", "muted wrap"],
    ],
    fields: () => [
      { name: "country", label: "Country", placeholder: "Netherlands", maxlength: 60 },
      { name: "name", label: "Name", placeholder: "Income Tax", maxlength: 80 },
      { name: "description", label: "Description (optional)", maxlength: 140 },
      { name: "url", label: "URL (optional)", placeholder: "https://…", maxlength: 180 },
    ],
    values: (t) => ({ country: t.country, name: t.name, description: t.description, url: t.url }),
    save: (id, f) => api.SaveTaxType({ id, ...f }),
    warn: (t, action) => (t.usedBy && action === "delete" ? `${t.usedBy} tax record(s) use it and keep their stored country and name.` : ""),
  },
  categoryList("income_category", "Income categories", "income category", true),
  categoryList("expense_category", "Expense categories", "expense category", false),
];

function categoryList(kind, title, noun, isIncome) {
  const items = (v) => (isIncome ? v.incomeCategories : v.expenseCategories);

  return {
    kind,
    title,
    noun,
    items,
    label: (c) => c.name,
    columns: () => [
      ["Name", (c) => (c.archived ? withBadge(c.name, "archived") : c.name)],
      ["Used by", usedBy, "num muted"],
    ],
    fields: () => [
      { name: "name", label: "Name", placeholder: isIncome ? "Salary" : "Groceries", maxlength: 80 },
      // For something temporary, like a side gig or buying a car.
      { name: "archived", label: "Archived: not offered for new entries and hidden in the Categories chart", checkbox: true },
    ],
    values: (c) => ({ name: c.name, archived: c.archived }),
    save: (id, f) => api.SaveCategory({ isIncome, id, name: f.name, archived: f.archived }),
    warn: (c, action) =>
      c.usedBy === 0
        ? ""
        : action === "edit"
          ? `${c.usedBy} entr(ies) use ${c.name} and keep that name if you rename it.`
          : `${c.usedBy} entr(ies) use ${c.name} and keep the name, but new entries can't pick it.`,
  };
}

function usedBy(item) {
  return item.usedBy ? String(item.usedBy) : "—";
}

function withBadge(text, label) {
  const span = document.createElement("span");
  span.append(text, badge(label));
  return span;
}

const state = {
  view: null,
  // What the dialog is editing: { list, item } for a list, or { base: true }.
  editing: null,
};

export const title = "Settings";

export function init() {
  el.editBase.addEventListener("click", openBase);
  el.form.addEventListener("submit", save);
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    state.view = await api.Settings();
    render(state.view);
    setStatus(message ?? "settings");
  } catch (err) {
    setStatus(`Failed to load settings: ${err}`);
  }
}

// --- rendering -------------------------------------------------------------

function render(view) {
  el.base.textContent = view.baseCurrency;
  el.db.textContent = view.dbPath;

  // Categories pair up side by side; the other lists take the full width.
  const panels = LISTS.map((list) => listPanel(list, view));
  const categories = document.createElement("div");
  categories.className = "columns";
  categories.append(panels[3], panels[4]);
  el.lists.replaceChildren(panels[0], panels[1], panels[2], categories);
}

function listPanel(list, view) {
  const panel = document.createElement("section");
  panel.className = "panel";
  panel.dataset.kind = list.kind;

  const head = document.createElement("div");
  head.className = "panel-head";
  const heading = document.createElement("h3");
  heading.className = "section-title";
  heading.textContent = list.title;
  const add = document.createElement("button");
  add.type = "button";
  add.className = "button";
  add.textContent = "Add";
  add.setAttribute("aria-label", `Add ${list.noun}`);
  add.addEventListener("click", () => openItem(list, null));
  head.append(heading, add);

  const items = list.items(view);
  const columns = list.columns(view);
  panel.append(head);

  if (items.length === 0) {
    const empty = document.createElement("p");
    empty.className = "muted";
    empty.textContent = `No ${list.title.toLowerCase()} yet.`;
    panel.append(empty);
    return panel;
  }

  const table = document.createElement("table");
  table.className = "table clickable";
  const headRow = document.createElement("tr");
  for (const [label, , className] of columns) {
    const th = document.createElement("th");
    th.textContent = label;
    if (className?.includes("num")) th.className = "num";
    headRow.append(th);
  }

  const body = document.createElement("tbody");
  for (const item of items) {
    const row = document.createElement("tr");
    for (const [, value, className] of columns) row.append(cell(value(item), className));
    clickableRow(row, () => openItem(list, item));
    body.append(row);
  }

  const thead = document.createElement("thead");
  thead.append(headRow);
  table.append(thead, body);
  panel.append(table);

  return panel;
}

// --- dialog ----------------------------------------------------------------

function buildFields(fields, values) {
  el.fields.replaceChildren(
    ...fields.map((field) => {
      const label = document.createElement("label");
      label.className = field.checkbox ? "check" : "field";

      const text = document.createElement("span");
      text.textContent = field.label;

      let input;
      if (field.options) {
        input = document.createElement("select");
        input.replaceChildren(...field.options.map((option) => new Option(option, option)));
      } else {
        input = document.createElement("input");
        input.autocomplete = "off";
        if (field.checkbox) input.type = "checkbox";
        if (field.placeholder) input.placeholder = field.placeholder;
        if (field.maxlength) input.maxLength = field.maxlength;
        if (field.inputmode) input.inputMode = field.inputmode;
      }

      input.name = field.name;
      const value = values[field.name];
      if (field.checkbox) input.checked = Boolean(value);
      else if (value !== undefined) input.value = value;

      if (field.checkbox) {
        label.append(input, text);
      } else {
        text.className = "field-label";
        label.append(text, input);
      }

      return label;
    }),
  );
}

function openDialog(heading, fields, values, note, canDelete) {
  el.title.textContent = heading;
  buildFields(fields, values);
  el.note.hidden = !note;
  el.note.textContent = note;
  el.delete.hidden = !canDelete;
  formError(el.form, "");
  el.dialog.showModal();
  el.fields.querySelector("input, select")?.focus();
}

function openItem(list, item) {
  const view = state.view;
  state.editing = { list, item };

  const defaults = list.kind === "payment_method" ? { type: view.paymentMethodTypes[0], isDefault: false } : {};
  const values = item ? list.values(item) : defaults;
  const heading = item ? `Edit ${list.noun}` : `Add ${list.noun}`;
  el.delete.textContent = `Delete ${list.noun}`;

  openDialog(heading, list.fields(view), values, item ? list.warn(item, "edit", view) : "", Boolean(item));
}

function openBase() {
  const view = state.view;
  state.editing = { base: true };

  const users = view.baseUsedBy ? `${view.baseUsedBy} record(s) are in ${view.baseCurrency}; to keep them in totals, add ${view.baseCurrency} as a currency with a rate afterwards. ` : "";
  const note = `${users}Existing rates aren't converted: they stay relative to whatever the base currency is.`;

  openDialog("Change base currency", [{ name: "value", label: "Base currency", placeholder: "€", maxlength: 24 }], { value: view.baseCurrency }, note, false);
}

function formValues() {
  const values = {};
  for (const input of el.fields.querySelectorAll("input, select")) {
    values[input.name] = input.type === "checkbox" ? input.checked : input.value;
  }

  return values;
}

async function save(event) {
  event.preventDefault();
  const editing = state.editing;
  const values = formValues();

  try {
    let message;
    if (editing.base) {
      await busy(el.form, () => api.SaveBaseCurrency(values.value));
      message = "saved base currency";
    } else {
      const { list, item } = editing;
      await busy(el.form, () => list.save(item?.id ?? 0, values));
      message = `saved ${list.noun} ${(values.name ?? values.country ?? "").trim()}`;
    }

    el.dialog.close();
    await show(message);
  } catch (err) {
    formError(el.form, String(err));
  }
}

function askDelete() {
  const { list, item } = state.editing;
  const name = list.label(item);
  const warning = list.warn(item, "delete", state.view);

  confirmDelete(`Delete ${list.noun}?`, `“${name}” will be deleted. ${warning}`.trim(), async () => {
    await api.DeleteSetting(list.kind, item.id);
    el.dialog.close();
    await show(`deleted ${list.noun} ${name}`);
  });
}
