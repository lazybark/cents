// First launch with a new database: pick the currencies in use from the
// built-in list and which one is the base. Rates are fetched for them.
import { api } from "./api.js";
import { describe, loadCatalog, match } from "./currencies.js";
import { busy, cell, formError, rowAction } from "./ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  screen: $("currency-setup"),
  form: $("currency-pick-form"),
  search: $("currency-search"),
  picked: $("currency-picked"),
  empty: $("currency-picked-empty"),
  error: $("currency-setup-error"),
  save: $("currency-save"),
  skip: $("currency-skip"),
};

const state = {
  codes: [],
  base: "",
  done: null,
};

el.form.addEventListener("submit", (event) => {
  event.preventDefault();
  add(el.search.value);
});

// Picking an option from the list adds it straight away.
el.search.addEventListener("change", () => {
  if (match(el.search.value) && el.search.value.includes("—")) add(el.search.value);
});

el.save.addEventListener("click", save);
el.skip.addEventListener("click", skip);

// showCurrencySetup shows the screen; done(status) runs once currencies are
// saved (status says how rates went) or the pick is skipped (no status).
export async function showCurrencySetup(done) {
  state.done = done;
  state.codes = [];
  state.base = "";
  el.screen.hidden = false;
  el.error.hidden = true;
  formError(el.form, "");
  render();

  try {
    await loadCatalog();
  } catch (err) {
    formError(el.form, `Failed to load the currency list: ${err}`);
  }

  el.search.focus();
}

function add(text) {
  const currency = match(text);
  if (!currency) {
    formError(el.form, "Pick a currency from the list.");
    return;
  }

  if (state.codes.includes(currency.code)) {
    formError(el.form, `${currency.code} is already picked.`);
    return;
  }

  state.codes.push(currency.code);
  // The first currency picked is the base until another one is marked.
  state.base ||= currency.code;
  el.search.value = "";
  formError(el.form, "");
  render();
  el.search.focus();
}

function remove(code) {
  state.codes = state.codes.filter((c) => c !== code);
  if (state.base === code) state.base = state.codes[0] ?? "";
  render();
}

function render() {
  el.empty.hidden = state.codes.length > 0;
  el.save.disabled = state.codes.length === 0;
  el.picked.replaceChildren(
    ...state.codes.map((code) => {
      const row = document.createElement("tr");

      const radio = document.createElement("input");
      radio.type = "radio";
      radio.name = "base-currency";
      radio.checked = code === state.base;
      radio.setAttribute("aria-label", `${code} is the base currency`);
      radio.addEventListener("change", () => {
        state.base = code;
        render();
      });

      const base = document.createElement("label");
      base.className = "check";
      base.append(radio, code === state.base ? "base" : "");

      row.append(cell(describe(code)), cell(base), cell(rowAction("Remove", () => remove(code)), "num"));
      return row;
    }),
  );
}

async function save() {
  el.error.hidden = true;

  try {
    const status = await busy(el.screen, () => api.SetupCurrencies({ codes: state.codes, base: state.base }));
    el.screen.hidden = true;
    state.done?.(status);
  } catch (err) {
    el.error.textContent = String(err);
    el.error.hidden = false;
  }
}

async function skip() {
  try {
    await busy(el.screen, () => api.SkipCurrencySetup());
    el.screen.hidden = true;
    state.done?.(null);
  } catch (err) {
    el.error.textContent = String(err);
    el.error.hidden = false;
  }
}
