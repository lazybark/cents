// The merge dialog: something records still use can't just be deleted, but
// it can be merged into another one of its kind, which moves its records
// there and deletes it.
import { busy, fillSelect, formError } from "./ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  dialog: $("merge-dialog"),
  form: $("merge-form"),
  title: $("merge-title"),
  message: $("merge-message"),
  note: $("merge-note"),
  alternative: $("merge-alternative"),
};

let pending = null;
let alternative = null;

el.alternative.addEventListener("click", async () => {
  try {
    await busy(el.form, () => alternative.run());
    el.dialog.close();
  } catch (err) {
    formError(el.form, String(err));
  }
});

el.form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const into = Number(el.form.elements.into.value);

  try {
    await busy(el.form, () => pending(into));
    el.dialog.close();
  } catch (err) {
    formError(el.form, String(err));
  }
});

// openMerge asks what to merge into.
//   title, message, note: what the dialog says.
//   options: [{ id, label }] to merge into.
//   merge(intoId): does it; the dialog closes once it resolves.
//   alternative: { label, run() } for another way out (deleting anyway).
export function openMerge({ title, message, note = "", options, merge, alternative: other = null }) {
  alternative = other;
  el.alternative.hidden = !other;
  el.alternative.textContent = other?.label ?? "";
  el.title.textContent = title;
  el.message.textContent = message;
  el.note.textContent = note;
  el.note.hidden = !note;
  fillSelect(
    el.form.elements.into,
    options.map((o) => String(o.id)),
    options.map((o) => o.label),
  );
  el.form.querySelector('[type="submit"]').disabled = options.length === 0;
  formError(el.form, options.length === 0 ? "There's nothing else to merge it into yet; add another one first, or archive this one." : "");
  pending = merge;
  el.dialog.showModal();
}
