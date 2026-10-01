// Property and investments: what the user owns besides accounts. Both kinds
// work the same way, so one module builds both views; they share the asset
// dialog. Values count towards net worth at today's rates, like accounts.
import { api } from "../api.js";
import {
  badge,
  busy,
  cell,
  clickableRow,
  confirmDelete,
  fillSelect,
  formatAmount,
  formatMoney,
  formError,
  localDate,
  rowAction,
  setStatus,
  signClass,
  withBase,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const dialog = {
  dialog: $("asset-dialog"),
  title: $("asset-dialog-title"),
  subtitle: $("asset-subtitle"),
  form: $("asset-form"),
  costLabel: $("asset-cost-label"),
  acquiredLabel: $("asset-acquired-label"),
  currencyHint: $("asset-currency-hint"),
  types: $("asset-types"),
  history: $("asset-history"),
  logForm: $("asset-log-form"),
  logs: $("asset-logs"),
  logsEmpty: $("asset-logs-empty"),
  delete: $("asset-delete"),
  close: document.querySelector("#asset-dialog .dialog-actions [data-close]"),
};

// The dialog belongs to whichever view opened it.
const editing = {
  view: null,
  // The asset being edited, or null when adding one.
  asset: null,
};

let dialogWired = false;

function wireDialog() {
  if (dialogWired) return;
  dialogWired = true;

  dialog.form.addEventListener("submit", save);
  dialog.logForm.addEventListener("submit", logValue);
  dialog.delete.addEventListener("click", askDelete);
}

// assetsView builds the view of one kind of asset ("property" or
// "investment"), with the words it uses.
export function assetsView(config) {
  const section = $(`view-${config.view}`);
  const q = (selector) => section.querySelector(selector);
  const el = {
    add: $(config.addButton),
    total: q(".asset-total"),
    gain: q(".asset-gain"),
    hint: q(".asset-hint"),
    typesPanel: q(".asset-types-panel"),
    types: q(".asset-types"),
    rows: q(".asset-rows"),
    empty: q(".asset-empty"),
    clickHint: q(".asset-click-hint"),
  };

  const view = { config, el, data: null };

  view.init = () => {
    wireDialog();
    q(".asset-acquired-heading").textContent = config.acquiredHeading;
    q(".asset-cost-heading").textContent = config.costHeading;
    el.empty.textContent = config.empty;
    el.add.addEventListener("click", () => openDialog(view, null));
  };

  view.show = async (message) => {
    try {
      view.data = await api.Assets(config.kind);
      render(view);
      setStatus(message ?? `${view.data.assets.length} ${config.countNoun}`);
    } catch (err) {
      setStatus(`Failed to load ${config.title.toLowerCase()}: ${err}`);
    }
  };

  return view;
}

// gainText reads like "+$ 500.00 (+12.5%)".
function gainText(currency, gain, cost) {
  const sign = gain > 0 ? "+" : "";
  const percent = cost > 0 ? ` (${sign}${((gain / cost) * 100).toFixed(1)}%)` : "";

  return `${sign}${formatMoney(currency, gain)}${percent}`;
}

function render(view) {
  const { data, el, config } = view;
  const base = data.baseCurrency;

  el.total.textContent = formatMoney(base, data.totalCents);

  const hasCost = data.costCents > 0;
  el.gain.textContent = hasCost ? gainText(base, data.gainCents, data.costCents) : "—";
  const tone = hasCost ? signClass(data.gainCents) : "muted";
  for (const name of ["positive", "negative", "muted"]) el.gain.classList.toggle(name, name === tone);

  const notes = [`In ${base} at today's rates; counted in net worth.`];
  if (hasCost) notes.push(`Gain compares the ${formatMoney(base, data.withCostCents)} of those with ${config.costWord} to the ${formatMoney(base, data.costCents)} they cost.`);
  if (data.missingRates > 0) notes.push(`${data.missingRates} without a conversion rate aren't counted.`);
  if (data.ignored > 0) notes.push(`${data.ignored} left out of net worth.`);
  el.hint.textContent = notes.join(" ");

  el.typesPanel.hidden = data.byType.length < 2;
  el.types.replaceChildren(
    ...data.byType.flatMap((t) => {
      const dt = document.createElement("dt");
      dt.textContent = `${t.type} (${t.count})`;
      const dd = document.createElement("dd");
      const share = data.totalCents > 0 ? ` · ${((t.baseCents / data.totalCents) * 100).toFixed(1)}%` : "";
      dd.textContent = `${formatMoney(base, t.baseCents)}${share}`;
      return [dt, dd];
    }),
  );

  el.empty.hidden = data.assets.length > 0;
  el.clickHint.hidden = data.assets.length === 0;
  el.rows.replaceChildren(
    ...data.assets.map((item) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = item.name;
      if (item.ignoreInNetWorth) name.append(badge("not in net worth"));

      const description = document.createElement("div");
      description.className = "account-description";
      description.textContent = item.description;

      const nameCell = document.createElement("div");
      nameCell.append(name, description);

      row.append(
        cell(nameCell),
        cell(item.type),
        cell(item.acquiredAt || "—", item.acquiredAt ? "nowrap" : "muted"),
        item.costCents > 0 ? cell(formatMoney(item.currency, item.costCents), "num") : cell("—", "num muted"),
        cell(withBase(item, item.valueCents, base, item.baseCents), "num"),
        item.hasGain ? cell(gainText(item.currency, item.gainCents, item.costCents), `num ${signClass(item.gainCents)}`) : cell("—", "num muted"),
      );
      clickableRow(row, () => openDialog(view, item));

      return row;
    }),
  );
}

// --- add and edit ----------------------------------------------------------

function openDialog(view, item) {
  const { config, data } = view;
  const form = dialog.form;
  const fields = form.elements;

  editing.view = view;
  editing.asset = item;
  form.reset();

  dialog.title.textContent = item ? item.name : config.addTitle;
  dialog.subtitle.hidden = !item;
  dialog.subtitle.textContent = item ? `${item.type} · ${item.currency}` : "";
  // Its input's placeholder says it's optional; the label stays short.
  dialog.costLabel.textContent = config.costLabel;
  dialog.acquiredLabel.textContent = `${config.acquiredLabel} (optional)`;
  fields.name.placeholder = config.namePlaceholder;
  dialog.types.replaceChildren(...(data?.types ?? []).map((t) => new Option(t)));

  // The currency is picked once; an existing asset keeps its own.
  const currencies = item ? [item.currency] : (data?.currencies ?? []);
  fillSelect(fields.currency, currencies);
  fields.currency.disabled = Boolean(item);
  dialog.currencyHint.textContent = item
    ? "The currency can't change after creation."
    : `Values in another currency count in ${data?.baseCurrency ?? "the base currency"} at today's rate, like accounts.`;

  if (item) {
    fields.name.value = item.name;
    fields.type.value = item.type;
    fields.value.value = formatAmount(item.valueCents);
    fields.cost.value = item.costCents > 0 ? formatAmount(item.costCents) : "";
    fields.acquiredAt.value = item.acquiredAt;
    fields.description.value = item.description;
  }
  fields.countInNetWorth.checked = !item?.ignoreInNetWorth;

  dialog.history.hidden = !item;
  dialog.delete.hidden = !item;
  dialog.close.textContent = item ? "Close" : "Cancel";
  formError(form, "");

  if (item) {
    dialog.logForm.reset();
    dialog.logForm.elements.date.value = localDate(new Date());
    formError(dialog.logForm, "");
    dialog.logs.replaceChildren();
    dialog.logsEmpty.hidden = true;
    loadLogs();
  }

  dialog.dialog.showModal();
}

async function save(event) {
  event.preventDefault();
  const { view, asset: item } = editing;
  const fields = dialog.form.elements;
  const input = {
    id: item?.id ?? 0,
    kind: view.config.kind,
    currency: fields.currency.value,
    name: fields.name.value,
    type: fields.type.value,
    value: fields.value.value,
    cost: fields.cost.value,
    acquiredAt: fields.acquiredAt.value,
    description: fields.description.value,
    ignoreInNetWorth: !fields.countInNetWorth.checked,
  };

  try {
    const saved = await busy(dialog.form, () => (item ? api.UpdateAsset(input) : api.CreateAsset(input)));
    dialog.dialog.close();
    await view.show(`${item ? "updated" : "saved"} ${saved.name}`);
  } catch (err) {
    formError(dialog.form, String(err));
  }
}

function askDelete() {
  const { view, asset: item } = editing;

  confirmDelete(`Delete ${item.name}?`, `“${item.name}” and its value history will be deleted. This can't be undone.`, async () => {
    await api.DeleteAsset(item.id);
    dialog.dialog.close();
    await view.show(`deleted ${item.name}`);
  });
}

// --- value history ---------------------------------------------------------

async function loadLogs() {
  const item = editing.asset;

  try {
    const logs = await api.AssetValueLogs(item.id);
    dialog.logsEmpty.hidden = logs.length > 0;
    dialog.logs.replaceChildren(
      ...logs.map((entry) => {
        const row = document.createElement("tr");
        row.append(
          cell(entry.date, "nowrap"),
          cell(formatMoney(item.currency, entry.valueCents), "num"),
          cell(rowAction("Delete", () => askDeleteLog(entry)), "num"),
        );
        return row;
      }),
    );
  } catch (err) {
    formError(dialog.logForm, `Failed to load value history: ${err}`);
  }
}

async function logValue(event) {
  event.preventDefault();
  const fields = dialog.logForm.elements;
  const input = { assetId: editing.asset.id, date: fields.date.value, value: fields.value.value };

  try {
    await busy(dialog.logForm, () => api.SaveAssetValueLog(input));
    fields.value.value = "";
    formError(dialog.logForm, "");
    await loadLogs();
    setStatus(`logged value for ${input.date}`);
  } catch (err) {
    formError(dialog.logForm, String(err));
  }
}

function askDeleteLog(entry) {
  const item = editing.asset;

  confirmDelete("Delete value?", `The value ${formatMoney(item.currency, entry.valueCents)} for ${entry.date} will be deleted. The current value doesn't change.`, async () => {
    await api.DeleteAssetValueLog(item.id, entry.id);
    await loadLogs();
    setStatus(`deleted value for ${entry.date}`);
  });
}
