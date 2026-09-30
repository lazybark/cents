// Month notes: a few words on why a month looks the way it does ("trip to
// Dubai"). Charts mark the months that have one and show it, and a click on
// a month adds or edits it; month headings show it too. The notes are kept
// here, loaded before each view shows (see main.js).
import { api } from "./api.js";
import { monthLabel } from "./charts.js";
import { busy, formError } from "./ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  dialog: $("note-dialog"),
  form: $("note-form"),
  title: $("note-title"),
  delete: $("note-delete"),
};

let notes = {};
let editing = "";

export async function loadNotes() {
  try {
    notes = (await api.MonthNotes()) ?? {};
  } catch {
    // Without notes, views still show; they just have none.
    notes = {};
  }
}

// noteFor is month's ("YYYY-MM") note, "" for none.
export function noteFor(month) {
  return notes[month] ?? "";
}

// editNote opens the dialog for month's note.
export function editNote(month) {
  editing = month;
  el.title.textContent = `Note for ${monthLabel(month, true)}`;
  el.form.elements.text.value = noteFor(month);
  el.delete.hidden = !noteFor(month);
  formError(el.form, "");
  el.dialog.showModal();
  el.form.elements.text.focus();
}

async function save(text) {
  await busy(el.form, () => api.SaveMonthNote({ month: editing, text }));
  el.dialog.close();
  await loadNotes();
  // The open view shows notes; it draws them again.
  window.dispatchEvent(new CustomEvent("cents:notes-changed", { detail: editing }));
}

el.form.addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await save(el.form.elements.text.value);
  } catch (err) {
    formError(el.form, String(err));
  }
});

el.delete.addEventListener("click", async () => {
  try {
    await save("");
  } catch (err) {
    formError(el.form, String(err));
  }
});

// noteLine is a month heading's note with an Edit button, or a button to
// add one.
export function noteLine(month) {
  const line = document.createElement("p");
  line.className = "month-note";
  const text = noteFor(month);
  const button = document.createElement("button");
  button.type = "button";
  button.className = "link-button";
  button.textContent = text ? "Edit note" : "Add a note for this month";
  button.addEventListener("click", () => editNote(month));

  if (text) line.append(text, button);
  else {
    line.classList.add("muted");
    line.append(button);
  }

  return line;
}
