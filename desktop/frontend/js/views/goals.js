// Goals: active ones or reached ones, with saved progress. Each goal can be
// edited, take logged changes (which can be deleted again) and be deleted,
// like in the TUI.
import { api } from "../api.js";
import {
  busy,
  cell,
  clickableRow,
  confirmDelete,
  dueCell,
  fillSelect,
  formatAmount,
  formatMoney,
  formError,
  localDate,
  progressText,
  renderPaymentLogs,
  setProgress,
  setStatus,
  signedMoney,
} from "../ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  add: $("add-goal"),
  tabs: document.querySelectorAll("#view-goals .tab"),
  progressText: $("goals-progress-text"),
  progressFill: $("goals-progress-fill"),
  rows: $("goals-rows"),
  empty: $("goals-empty"),
  hint: $("goals-hint"),

  addDialog: $("goal-add-dialog"),
  addForm: $("goal-add-form"),

  editDialog: $("goal-edit-dialog"),
  editTitle: $("goal-title"),
  editSubtitle: $("goal-subtitle"),
  editProgress: $("goal-progress-fill"),
  editForm: $("goal-edit-form"),
  changeForm: $("goal-change-form"),
  logs: $("goal-logs"),
  logsEmpty: $("goal-logs-empty"),
  delete: $("goal-delete"),
};

const TAB_LABELS = { active: "Active", done: "Reached" };

const state = {
  mode: "active",
  view: null,
  current: null,
};

export const title = "Goals";

export function init() {
  for (const tab of el.tabs) {
    tab.addEventListener("click", () => {
      state.mode = tab.dataset.mode;
      show();
    });
  }

  el.add.addEventListener("click", openAdd);
  el.addForm.addEventListener("submit", saveNew);
  el.editForm.addEventListener("submit", saveEdit);
  el.changeForm.addEventListener("submit", logChange);
  el.delete.addEventListener("click", askDelete);
}

export async function show(message) {
  try {
    const view = await api.Goals(state.mode);
    state.view = view;
    state.mode = view.mode;
    render(view);
    setStatus(message ?? `${view.goals.length} goal(s)`);
  } catch (err) {
    setStatus(`Failed to load goals: ${err}`);
  }
}

function render(view) {
  for (const tab of el.tabs) {
    tab.setAttribute("aria-selected", String(tab.dataset.mode === view.mode));
    tab.textContent = `${TAB_LABELS[tab.dataset.mode]} (${view.counts[tab.dataset.mode] ?? 0})`;
  }

  el.progressText.textContent = progressText(view.baseCurrency, view.progress.paidCents, view.progress.totalCents);
  setProgress(el.progressFill, view.progress.paidCents, view.progress.totalCents);

  el.empty.hidden = view.goals.length > 0;
  el.hint.hidden = view.goals.length === 0;
  el.rows.replaceChildren(
    ...view.goals.map((goal) => {
      const row = document.createElement("tr");

      const name = document.createElement("div");
      name.className = "account-name";
      name.textContent = goal.name;

      const description = document.createElement("div");
      description.className = "account-description";
      description.textContent = goal.description;

      const nameCell = document.createElement("div");
      nameCell.append(name, description);

      row.append(
        cell(nameCell),
        cell(goal.startedAt, "nowrap"),
        dueCell(goal.targetDate, goal.overdue),
        cell(formatMoney(goal.currency, goal.targetCents), "num"),
        cell(formatMoney(goal.currency, goal.accumulatedCents), "num"),
        cell(formatMoney(goal.currency, goal.leftCents), goal.leftCents > 0 ? "num" : "num muted"),
        cell(`${goal.percent.toFixed(1)}%`, "num"),
      );
      clickableRow(row, () => openEdit(goal));

      return row;
    }),
  );
}

// --- add -------------------------------------------------------------------

function openAdd() {
  const form = el.addForm;
  form.reset();
  fillSelect(form.elements.currency, state.view?.currencies ?? []);
  form.elements.startedAt.value = localDate(new Date());
  formError(form, "");
  el.addDialog.showModal();
}

async function saveNew(event) {
  event.preventDefault();
  const form = el.addForm.elements;
  const input = {
    name: form.name.value,
    currency: form.currency.value,
    target: form.target.value,
    accumulated: form.accumulated.value,
    startedAt: form.startedAt.value,
    targetDate: form.targetDate.value,
    description: form.description.value,
  };

  try {
    const created = await busy(el.addForm, () => api.CreateGoal(input));
    el.addDialog.close();
    state.mode = created.mode;
    await show(`saved goal ${input.name.trim()}`);
  } catch (err) {
    formError(el.addForm, String(err));
  }
}

// --- edit, changes, delete -------------------------------------------------

function openEdit(goal) {
  state.current = goal;
  const form = el.editForm.elements;

  form.target.value = formatAmount(goal.targetCents);
  form.accumulated.value = formatAmount(goal.accumulatedCents);
  form.startedAt.value = goal.startedAt;
  form.targetDate.value = goal.targetDate;
  form.description.value = goal.description;
  formError(el.editForm, "");

  el.changeForm.reset();
  el.changeForm.elements.date.value = localDate(new Date());
  formError(el.changeForm, "");

  showGoalHeader(goal);
  el.logs.replaceChildren();
  el.logsEmpty.hidden = true;
  el.editDialog.showModal();
  loadLogs();
}

function showGoalHeader(goal) {
  el.editTitle.textContent = goal.name;
  el.editSubtitle.textContent = `${goal.currency} · ${progressText(goal.currency, goal.accumulatedCents, goal.targetCents)}`;
  setProgress(el.editProgress, goal.accumulatedCents, goal.targetCents);
}

async function loadLogs() {
  const goal = state.current;

  try {
    renderPaymentLogs(el.logs, el.logsEmpty, await api.GoalLogs(goal.id), goal.currency, askDeleteChange);
  } catch (err) {
    formError(el.changeForm, `Failed to load changes: ${err}`);
  }
}

async function saveEdit(event) {
  event.preventDefault();
  const goal = state.current;
  const form = el.editForm.elements;
  const input = {
    id: goal.id,
    target: form.target.value,
    accumulated: form.accumulated.value,
    startedAt: form.startedAt.value,
    targetDate: form.targetDate.value,
    description: form.description.value,
  };

  try {
    await busy(el.editForm, () => api.UpdateGoal(input));
    el.editDialog.close();
    await show(`updated goal ${goal.name}`);
  } catch (err) {
    formError(el.editForm, String(err));
  }
}

// After a change (logged or deleted) the dialog stays open: it updates the
// header, the saved field and the log, and refreshes the list behind.
async function changed(updated, message) {
  state.current = updated;
  showGoalHeader(updated);
  el.editForm.elements.accumulated.value = formatAmount(updated.accumulatedCents);
  formError(el.changeForm, "");
  await loadLogs();
  await show(message);
}

async function logChange(event) {
  event.preventDefault();
  const form = el.changeForm.elements;
  const input = { id: state.current.id, delta: form.delta.value, date: form.date.value, note: form.note.value };

  try {
    const updated = await busy(el.changeForm, () => api.AddGoalChange(input));
    form.delta.value = "";
    form.note.value = "";
    await changed(updated, "logged goal change");
  } catch (err) {
    formError(el.changeForm, String(err));
  }
}

// Deleting a change undoes it, so say what saved goes back to.
function askDeleteChange(entry) {
  const goal = state.current;
  const money = (cents) => formatMoney(goal.currency, cents);
  const change = `Saved goes from ${money(goal.accumulatedCents)} to ${money(goal.accumulatedCents - entry.deltaCents)}.`;

  confirmDelete("Delete change?", `The ${signedMoney(goal.currency, entry.deltaCents)} change from ${entry.when} will be deleted. ${change}`, async () => {
    await changed(await api.DeleteGoalChange(goal.id, entry.id), "deleted goal change");
  });
}

function askDelete() {
  const goal = state.current;

  confirmDelete("Delete goal?", `The goal “${goal.name}” and its log will be deleted. This can't be undone.`, async () => {
    await api.DeleteGoal(goal.id);
    el.editDialog.close();
    await show(`deleted goal ${goal.name}`);
  });
}
