// Settings: read-only for now.
import { api } from "../api.js";
import { cell, setStatus } from "../ui.js";

const el = {
  db: document.getElementById("settings-db"),
  base: document.getElementById("settings-base"),
  currencies: document.getElementById("settings-currencies"),
  currenciesEmpty: document.getElementById("settings-currencies-empty"),
};

export const title = "Settings";

export async function show() {
  try {
    const view = await api.Settings();

    el.db.textContent = view.dbPath;
    el.base.textContent = view.baseCurrency;
    el.currencies.replaceChildren(
      ...view.currencies.map((currency) => {
        const row = document.createElement("tr");
        row.append(cell(currency.name), cell(String(currency.rateToBase), "num"));
        return row;
      }),
    );
    el.currenciesEmpty.hidden = view.currencies.length > 0;
    setStatus("settings");
  } catch (err) {
    setStatus(`Failed to load settings: ${err}`);
  }
}
