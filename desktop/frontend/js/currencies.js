// The built-in list of known currencies, for pickers: a datalist offers
// them as "EUR — Euro (€)" while typing, and match() turns what was typed
// (a code, a name, a symbol or a picked option) into a code.
import { api } from "./api.js";

const datalist = document.getElementById("currency-catalog");

let catalog = [];
let loading = null;

// loadCatalog fetches the list once and fills the datalist.
export function loadCatalog() {
  loading ??= api.CurrencyCatalog().then((list) => {
    catalog = list;
    datalist.replaceChildren(...list.map((c) => new Option(optionText(c))));
    return list;
  });

  return loading;
}

export function optionText(c) {
  return `${c.code} — ${c.name} (${c.symbol})`;
}

export function find(code) {
  return catalog.find((c) => c.code === code);
}

// describe reads like "EUR — Euro (€)", or the code alone when unknown.
export function describe(code) {
  const c = find(code);
  return c ? optionText(c) : code;
}

// match finds the currency meant by text: a picked option, a code, an exact
// name or symbol, or the only currency whose name contains it. It returns
// null when that isn't clear.
export function match(text) {
  const raw = (text ?? "").trim();
  if (!raw) return null;

  const code = raw.split(/\s/)[0].toUpperCase();
  const byCode = find(code);
  if (byCode) return byCode;

  const lower = raw.toLowerCase();
  const exact = catalog.filter((c) => c.name.toLowerCase() === lower || c.symbol === raw);
  if (exact.length === 1) return exact[0];

  const partial = catalog.filter((c) => c.name.toLowerCase().includes(lower));
  return partial.length === 1 ? partial[0] : null;
}
