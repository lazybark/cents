// Backup & export: a full copy of the database where the user picks, and
// CSV files of chosen records for a period, for spreadsheets.
import { api } from "../api.js";
import { busy, formError, localDate, setStatus } from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  backupNow: $("backup-now"),
  backupLast: $("backup-last"),
  backupDB: $("backup-db"),

  form: $("export-form"),
  timed: $("export-timed"),
  lists: $("export-lists"),
  all: $("export-all"),
  none: $("export-none"),
  result: $("export-result"),
  resultTitle: $("export-result-title"),
  resultFiles: $("export-result-files"),
};

const state = {
  view: null,
  // Picks survive switching views; everything is picked to start with.
  picked: null,
};

export const title = "Backup & export";

export function init() {
  el.backupNow.addEventListener("click", backup);
  el.form.addEventListener("submit", exportCSV);
  el.form.elements.preset.addEventListener("change", applyPreset);
  for (const name of ["from", "to"]) {
    el.form.elements[name].addEventListener("change", () => {
      el.form.elements.preset.value = "custom";
    });
  }

  el.all.addEventListener("click", () => pickAll(true));
  el.none.addEventListener("click", () => pickAll(false));
}

export async function show(message) {
  try {
    const view = await api.ExportOptions();
    state.view = view;
    render(view);
    if (!el.form.elements.from.value && !el.form.elements.to.value) applyPreset();
    setStatus(message ?? "backup & export");
  } catch (err) {
    setStatus(`Failed to load backup & export: ${err}`);
  }
}

function render(view) {
  el.backupLast.textContent = view.lastBackupAt ? `Last backup: ${view.lastBackupAt}, to ${view.lastBackupPath}` : "No backup made yet.";
  el.backupLast.className = view.lastBackupAt ? "" : "muted";
  el.backupDB.textContent = view.dbPath;

  state.picked ??= new Set(view.datasets.map((ds) => ds.key));
  const box = (ds) => {
    const label = document.createElement("label");
    label.className = "check";
    const input = document.createElement("input");
    input.type = "checkbox";
    input.value = ds.key;
    input.checked = state.picked.has(ds.key);
    input.addEventListener("change", () => (input.checked ? state.picked.add(ds.key) : state.picked.delete(ds.key)));
    const text = document.createElement("span");
    text.textContent = ds.label;
    label.append(input, text);
    return label;
  };

  el.timed.replaceChildren(...view.datasets.filter((ds) => ds.timed).map(box));
  el.lists.replaceChildren(...view.datasets.filter((ds) => !ds.timed).map(box));
}

function pickAll(on) {
  for (const input of el.form.querySelectorAll(".check-list input")) {
    input.checked = on;
    if (on) state.picked.add(input.value);
    else state.picked.delete(input.value);
  }
}

// applyPreset fills From and To for the picked period.
function applyPreset() {
  const form = el.form.elements;
  const now = new Date();
  const year = now.getFullYear();
  const month = now.getMonth();
  const day = (y, m, d) => localDate(new Date(y, m, d));
  const today = localDate(now);

  const ranges = {
    "this-month": [day(year, month, 1), today],
    "last-month": [day(year, month - 1, 1), day(year, month, 0)],
    "this-year": [day(year, 0, 1), today],
    "last-year": [day(year - 1, 0, 1), day(year - 1, 11, 31)],
    "last-12": [day(year, month - 11, 1), today],
    all: [state.view?.firstDate ?? "", today],
  };

  const range = ranges[form.preset.value];
  if (!range) return;
  [form.from.value, form.to.value] = range;
}

async function backup() {
  try {
    const result = await busy(el.backupNow.closest(".panel"), () => api.BackupDatabase());
    if (!result.path) return;
    await show(`backed up to ${result.path}`);
  } catch (err) {
    setStatus(`Backup failed: ${err}`);
    el.backupLast.textContent = `Backup failed: ${err}`;
    el.backupLast.className = "negative";
  }
}

async function exportCSV(event) {
  event.preventDefault();
  const form = el.form.elements;
  const input = { from: form.from.value, to: form.to.value, datasets: [...state.picked] };

  try {
    const result = await busy(el.form, () => api.ExportCSV(input));
    formError(el.form, "");
    if (!result.folder) return;

    el.result.hidden = false;
    el.resultTitle.textContent = `Saved ${result.files.length} file${result.files.length === 1 ? "" : "s"} to ${result.folder}:`;
    el.resultFiles.replaceChildren(
      ...result.files.map((file) => {
        const item = document.createElement("li");
        item.textContent = `${file.name} — ${file.rows} row${file.rows === 1 ? "" : "s"}`;
        return item;
      }),
    );
    setStatus(`exported ${result.files.length} CSV file(s) to ${result.folder}`);
  } catch (err) {
    formError(el.form, String(err));
  }
}
