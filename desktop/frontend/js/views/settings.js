// Settings: base currency plus every settings list the TUI can edit. Each
// list is described once in LISTS; one panel renderer and one dialog serve
// them all.
import { api } from "../api.js";
import { describe, loadCatalog, match } from "../currencies.js";
import { openMerge } from "../merge.js";
import { THEMES, savedTheme, setTheme } from "../theme.js";
import { badge, busy, cell, clickableRow, confirmDelete, formatUpdatedAt, formError, reloadApp, setStatus } from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  base: $("settings-base"),
  baseCode: $("settings-base-code"),
  editBase: $("edit-base"),
  ratesRefresh: $("rates-refresh"),
  ratesStatus: $("rates-status"),
  ratesAuto: $("rates-auto"),
  ratesNote: $("rates-note"),
  ratesError: $("rates-error"),
  db: $("settings-db"),
  dbOpen: $("db-open"),
  dbCreate: $("db-create"),
  dbError: $("db-error"),
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
      ["Linked to", (c) => (c.code ? `${c.code} · ${c.codeName}` : "not linked"), (c) => (c.code ? "" : "muted")],
      [`1 unit in ${v.baseCurrency}`, (c) => (c.rateToBase > 0 ? `${v.baseCurrency} ${Number(c.rateToBase.toPrecision(6))}` : "no rate yet"), "num"],
      ["Used by", usedBy, "num muted"],
    ],
    fields: (v) => [
      { name: "code", label: "Currency", placeholder: "pick from the list: EUR, euro, €…", list: "currency-catalog", currency: true },
      { name: "name", label: "Name records use (optional: the code)", placeholder: "EUR, €…", maxlength: 24 },
      {
        name: "rate",
        label: v.baseCode ? `Value of 1 unit in ${v.baseCurrency} (optional: fetched for you)` : `Value of 1 unit in ${v.baseCurrency}`,
        placeholder: "1.08",
        inputmode: "decimal",
      },
    ],
    values: (c) => ({ code: c.code ? describe(c.code) : "", name: c.name, rate: String(c.rateToBase) }),
    save: (id, f) => api.SaveCurrency({ id, code: f.code, name: f.name, rate: f.rate }),
    // The base is a currency records can use too.
    mergeTargets: (v, c) => [{ id: v.baseCurrencyId, label: `${v.baseCurrency} (base currency)` }, ...v.currencies.filter((o) => o.id !== c.id).map((o) => ({ id: o.id, label: o.name }))],
    mergeNote: "Only for two names of one currency, like “$” and “USD”: amounts are moved as they are, not converted.",
    warn: (c, action, v) => {
      const users = c.usedBy && action === "edit" ? `${c.usedBy} record(s) use ${c.name}; renaming it renames it there too.` : "";
      const auto = action === "edit" && c.linked && v.rates.auto ? "Its rate is updated once a day, so a typed rate is replaced on the next update." : "";
      return [users, auto].filter(Boolean).join(" ");
    },
  },
  {
    kind: "payment_method",
    title: "Payment methods",
    noun: "payment method",
    items: (v) => v.paymentMethods,
    label: (p) => p.name,
    columns: () => [
      ["Name", (p) => (p.isDefault ? withBadge(p.name, "default") : p.archived ? withBadge(p.name, "archived") : p.name)],
      ["Type", (p) => p.type],
      ["Currency", (p) => p.currency || "any", (p) => (p.currency ? "" : "muted")],
      ["Used by", usedBy, "num muted"],
    ],
    fields: (v) => {
      // Any currency in settings, or none ("any"); a method keeps its own
      // even if it left settings since.
      const currencies = [v.baseCurrency, ...v.currencies.map((c) => c.name)];
      const own = state.editing?.item?.currency;
      if (own && !currencies.includes(own)) currencies.push(own);

      return [
        { name: "name", label: "Name", placeholder: "Personal Visa", maxlength: 40 },
        { name: "type", label: "Type", options: v.paymentMethodTypes },
        { name: "currency", label: "Currency (fills in for payments made with it)", options: ["", ...currencies], labels: ["Any", ...currencies] },
        { name: "isDefault", label: "Default for new subscriptions", checkbox: true },
        { name: "archived", label: "Archived: not offered for new subscriptions", checkbox: true },
      ];
    },
    values: (p) => ({ name: p.name, type: p.type, currency: p.currency, isDefault: p.isDefault, archived: p.archived }),
    save: (id, f) => api.SavePaymentMethod({ id, name: f.name, type: f.type, currency: f.currency, isDefault: f.isDefault, archived: f.archived }),
    warn: (p, action, v) => {
      if (action === "edit") return p.usedBy ? `${p.usedBy} subscription(s) use ${p.name}; renaming it renames it there too.` : "";
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
      ["Used by", usedBy, "num muted"],
    ],
    fields: () => [
      { name: "country", label: "Country", placeholder: "Netherlands", maxlength: 60 },
      { name: "name", label: "Name", placeholder: "Income Tax", maxlength: 80 },
      { name: "description", label: "Description (optional)", maxlength: 140 },
      { name: "url", label: "URL (optional)", placeholder: "https://…", maxlength: 180 },
    ],
    values: (t) => ({ country: t.country, name: t.name, description: t.description, url: t.url }),
    save: (id, f) => api.SaveTaxType({ id, ...f }),
    warn: (t, action) => (t.usedBy && action === "edit" ? `${t.usedBy} tax record(s) use it; renaming it renames it there too.` : ""),
  },
  categoryList("income_category", "Income categories", "income category", true),
  categoryList("expense_category", "Expense categories", "expense category", false),
  {
    kind: "tag",
    title: "Tags",
    noun: "tag",
    items: (v) => v.tags,
    label: (t) => t.name,
    columns: () => [
      ["Name", (t) => (t.archived ? withBadge(t.name, "archived") : t.name)],
      ["Used by", usedBy, "num muted"],
    ],
    fields: () => [
      { name: "name", label: "Name", placeholder: "Trip to Japan", maxlength: 40 },
      { name: "archived", label: "Archived: not suggested for new entries", checkbox: true },
    ],
    values: (t) => ({ name: t.name, archived: t.archived }),
    save: (id, f) => api.SaveTag({ id, name: f.name, archived: f.archived }),
    warn: (t, action) => (t.usedBy && action === "edit" ? `${t.usedBy} entr(ies) have it; renaming it renames it there too.` : ""),
    // A tag can always go; merging is offered first, for typos and doubles.
    mergeNote: "Its entries get the other tag instead.",
    deleteAnyway: true,
  },
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
    warn: (c, action) => (c.usedBy && action === "edit" ? `${c.usedBy} record(s) use ${c.name}; renaming it renames it there too.` : ""),
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
  loadCatalog().catch((err) => setStatus(`Failed to load the currency list: ${err}`));
  renderThemes();
  $("security-form").addEventListener("submit", savePassword);
  $("security-remove").addEventListener("click", removePassword);
  el.ratesRefresh.addEventListener("click", refreshRates);
  el.dbOpen.addEventListener("click", () => switchDatabase(api.OpenDatabase, "opened"));
  el.dbCreate.addEventListener("click", () => switchDatabase(api.CreateDatabase, "created"));
  el.ratesAuto.addEventListener("change", setRatesAuto);
  el.editBase.addEventListener("click", openBase);
  el.form.addEventListener("submit", save);
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    state.view = await api.Settings();
    render(state.view);
    renderSecurity(await api.Status());
    setStatus(message ?? "settings");
  } catch (err) {
    setStatus(`Failed to load settings: ${err}`);
  }
}

// --- rendering -------------------------------------------------------------

function render(view) {
  el.base.textContent = view.baseCurrency;
  el.baseCode.textContent = view.baseCode
    ? `Linked to ${describe(view.baseCode)}`
    : "Not linked to a known currency, so rates can't be fetched. Use Change to link it.";
  renderRates(view.rates, view);
  el.db.textContent = view.dbPath;

  // Categories pair up side by side; the other lists take the full width.
  const panels = LISTS.map((list) => listPanel(list, view));
  const byKind = (kind) => panels[LISTS.findIndex((list) => list.kind === kind)];
  const categories = document.createElement("div");
  categories.className = "columns";
  categories.append(byKind("income_category"), byKind("expense_category"));
  el.lists.replaceChildren(byKind("currency"), byKind("payment_method"), byKind("tax_type"), categories, byKind("tag"));
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
    if (typeof className === "string" && className.includes("num")) th.className = "num";
    headRow.append(th);
  }

  const body = document.createElement("tbody");
  for (const item of items) {
    const row = document.createElement("tr");
    for (const [, value, className] of columns) row.append(cell(value(item), typeof className === "function" ? className(item) : className));
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
        input.replaceChildren(...field.options.map((option, i) => new Option(field.labels?.[i] ?? option, option)));
      } else {
        input = document.createElement("input");
        input.autocomplete = "off";
        if (field.checkbox) input.type = "checkbox";
        if (field.placeholder) input.placeholder = field.placeholder;
        if (field.maxlength) input.maxLength = field.maxlength;
        if (field.inputmode) input.inputMode = field.inputmode;
      }

      if (field.list) input.setAttribute("list", field.list);
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

  const users = view.baseUsedBy
    ? `${view.baseUsedBy} record(s) are in ${view.baseCurrency}. A new name for the same currency (like € → EUR) renames it on them; a different currency keeps them in ${view.baseCurrency}, which stays as an ordinary currency that needs a rate. `
    : "";
  const rates = view.rates.auto ? "Linked currencies get new rates for the new base right away; others keep theirs." : "Existing rates aren't converted: they stay relative to whatever the base currency is.";

  openDialog(
    "Change base currency",
    [
      { name: "code", label: "Currency", placeholder: "pick from the list: EUR, euro, €…", list: "currency-catalog", currency: true },
      { name: "value", label: "Name records use (optional: the code)", placeholder: "€", maxlength: 24 },
    ],
    { code: view.baseCode ? describe(view.baseCode) : "", value: view.baseCurrency },
    `${users}${rates}`,
    false,
  );
}

// --- exchange rates --------------------------------------------------------

function renderRates(rates, view) {
  el.ratesAuto.checked = rates.auto;
  el.ratesRefresh.disabled = !rates.baseCode;
  el.ratesStatus.textContent = rates.updatedAt
    ? `Updated ${formatUpdatedAt(rates.updatedAt)} from ${rates.source} (rates of ${rates.date}).`
    : "Not updated yet.";

  const notes = [];
  if (!rates.baseCode) notes.push(`Link the base currency ${view.baseCurrency} to a known currency to fetch rates.`);
  else notes.push(`${rates.linked} currenc${rates.linked === 1 ? "y gets its rate" : "ies get their rates"} fetched in ${rates.baseCode}.`);
  if (rates.unlinked.length) notes.push(`Not linked, so they keep the rates you typed: ${rates.unlinked.join(", ")}.`);
  el.ratesNote.textContent = notes.join(" ");

  el.ratesError.hidden = !rates.lastError;
  el.ratesError.textContent = rates.lastError ? `Last update failed: ${rates.lastError}` : "";
}

async function refreshRates() {
  try {
    await busy(el.ratesRefresh.closest(".panel"), () => api.RefreshRates());
    await show("rates updated");
  } catch (err) {
    await show();
    setStatus(`Rates not updated: ${err}`);
  }
}

async function setRatesAuto() {
  try {
    await api.SetRatesAuto(el.ratesAuto.checked);
    await show(el.ratesAuto.checked ? "rates update daily" : "rates kept as they are");
  } catch (err) {
    setStatus(`Failed to change rate updates: ${err}`);
  }
}

// formValues reads the dialog; currency pickers give the picked code (or
// the text as typed, for the Go side to reject).
function formValues() {
  const values = {};
  for (const input of el.fields.querySelectorAll("input, select")) {
    if (input.type === "checkbox") values[input.name] = input.checked;
    else if (input.getAttribute("list") === "currency-catalog") values[input.name] = input.value.trim() ? (match(input.value)?.code ?? input.value.trim()) : "";
    else values[input.name] = input.value;
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
      await busy(el.form, () => api.SaveBaseCurrency({ value: values.value, code: values.code }));
      message = "saved base currency";
    } else {
      const { list, item } = editing;
      await busy(el.form, () => list.save(item?.id ?? 0, values));
      message = `saved ${list.noun} ${(values.name || values.code || values.country || "").trim()}`;
    }

    el.dialog.close();
    await show(message);
  } catch (err) {
    formError(el.form, String(err));
  }
}

// Lists records link to: what's in use is merged rather than deleted.
const MERGEABLE = new Set(["currency", "payment_method", "tax_type", "income_category", "expense_category", "tag"]);

function askDelete() {
  const { list, item } = state.editing;
  const name = list.label(item);
  const warning = list.warn(item, "delete", state.view);

  if (MERGEABLE.has(list.kind) && item.usedBy > 0) {
    const options = list.mergeTargets
      ? list.mergeTargets(state.view, item)
      : list
          .items(state.view)
          .filter((other) => other.id !== item.id)
          .map((other) => ({ id: other.id, label: list.label(other) + (other.archived ? " (archived)" : "") }));
    openMerge({
      title: `Merge ${list.noun}?`,
      message: list.deleteAnyway
        ? `“${name}” is on ${item.usedBy} entr${item.usedBy === 1 ? "y" : "ies"}. Merge it into another ${list.noun}, or delete it from ${item.usedBy === 1 ? "it" : "them"}.`
        : `“${name}” is used by ${item.usedBy} record(s), so it can't just be deleted. Merge it into another ${list.noun}: its records move there and “${name}” is deleted.`,
      note: list.mergeNote ?? (list.kind.endsWith("category") || list.kind === "payment_method" ? "To keep it but stop offering it, archive it instead." : ""),
      options,
      alternative: list.deleteAnyway
        ? {
            label: `Just delete it from ${item.usedBy} entr${item.usedBy === 1 ? "y" : "ies"}`,
            run: async () => {
              await api.DeleteSetting(list.kind, item.id);
              el.dialog.close();
              await show(`deleted ${list.noun} ${name}`);
            },
          }
        : null,
      merge: async (into) => {
        const result = await api.Merge({ kind: list.kind, fromId: item.id, intoId: into });
        el.dialog.close();
        const target = options.find((o) => o.id === into).label;
        await show(`merged ${list.noun} ${name} into ${target}: ${result.moved} record(s) moved`);
      },
    });
    return;
  }

  confirmDelete(`Delete ${list.noun}?`, `“${name}” will be deleted. ${warning}`.trim(), async () => {
    await api.DeleteSetting(list.kind, item.id);
    el.dialog.close();
    await show(`deleted ${list.noun} ${name}`);
  });
}

// --- database ----------------------------------------------------------------

// switchDatabase runs the Open or Create dialog; once the app has switched,
// it starts over on the new database (currency setup first, for a new one).
async function switchDatabase(choose, verb) {
  el.dbError.hidden = true;

  try {
    const chosen = await busy(el.dbOpen.closest(".panel"), choose);
    if (!chosen) return;

    const status = await api.Status();
    reloadApp(`${verb} ${status.dbPath}`);
  } catch (err) {
    el.dbError.textContent = String(err);
    el.dbError.hidden = false;
  }
}

// --- appearance --------------------------------------------------------------

// renderThemes shows a card per theme, the one in use checked; picking one
// switches to it right away.
function renderThemes() {
  const picker = $("theme-picker");
  const current = savedTheme();

  picker.replaceChildren(
    ...THEMES.map((theme) => {
      const card = document.createElement("button");
      card.type = "button";
      card.className = "theme-card";
      card.setAttribute("role", "radio");
      card.setAttribute("aria-checked", String(theme.id === current));
      card.dataset.theme = theme.id;

      const swatch = document.createElement("span");
      swatch.className = "theme-swatch";
      for (const color of theme.swatch) {
        const part = document.createElement("span");
        part.style.background = color;
        swatch.append(part);
      }

      const name = document.createElement("span");
      name.className = "theme-name";
      name.textContent = theme.name;
      const note = document.createElement("span");
      note.className = "theme-note";
      note.textContent = theme.note;

      card.append(swatch, name, note);
      card.addEventListener("click", () => {
        setTheme(theme.id);
        renderThemes();
        setStatus(`theme: ${theme.name}`);
      });
      return card;
    }),
  );
}

// --- password ------------------------------------------------------------------

const security = {
  status: $("security-status"),
  form: $("security-form"),
  current: $("security-current-field"),
  newLabel: $("security-new-label"),
  save: $("security-save"),
  remove: $("security-remove"),
  encrypted: false,
};

// renderSecurity shows whether the database has a password, and the form
// to set one, or to change or remove it.
function renderSecurity(status) {
  security.encrypted = status.encrypted;
  security.form.reset();
  formError(security.form, "");
  security.status.textContent = status.encrypted
    ? "This database is encrypted with a password."
    : "This database has no password: anyone with the file can read it.";
  security.status.className = status.encrypted ? "positive" : "muted";
  security.current.hidden = !status.encrypted;
  security.newLabel.textContent = status.encrypted ? "New password" : "Password";
  security.save.textContent = status.encrypted ? "Change the password" : "Encrypt with this password";
  security.remove.hidden = !status.encrypted;
}

async function savePassword(event) {
  event.preventDefault();
  const f = security.form.elements;
  if (f.next.value !== f.again.value) {
    formError(security.form, "The new passwords don't match.");
    return;
  }

  if (!f.next.value) {
    formError(security.form, "Type the new password twice.");
    return;
  }

  const encrypted = security.encrypted;
  try {
    await busy(security.form, () => api.SetPassword({ current: f.current.value, new: f.next.value }));
    await show(encrypted ? "password changed" : "the database is encrypted now");
  } catch (err) {
    formError(security.form, String(err));
  }
}

async function removePassword() {
  const f = security.form.elements;
  if (!f.current.value) {
    formError(security.form, "Type the current password to remove it.");
    return;
  }

  try {
    await busy(security.form, () => api.SetPassword({ current: f.current.value, new: "" }));
    await show("password removed: the database is a plain file again");
  } catch (err) {
    formError(security.form, String(err));
  }
}
