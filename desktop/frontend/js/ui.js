// Formatting and DOM helpers shared by every view.

// Mirrors renderMoneyWithCurrency in app/render.go: "-$ 12.34".
export function formatMoney(currency, cents) {
  const sign = cents < 0 ? "-" : "";
  const amount = formatAmount(Math.abs(cents));

  return currency ? `${sign}${currency} ${amount}` : `${sign}${amount}`;
}

// Mirrors formatAmount in app/format.go, for prefilling inputs.
export function formatAmount(cents) {
  return (cents / 100).toFixed(2);
}

// Mirrors formatUpdatedAt in app/format.go: records never updated carry a
// 1970 placeholder date.
export function formatUpdatedAt(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 1971) return "—";

  return `${localDate(date)} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// localDate formats as YYYY-MM-DD, the value format of <input type="date">.
export function localDate(date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function pad(value) {
  return String(value).padStart(2, "0");
}

export function signClass(cents) {
  if (cents > 0) return "positive";
  if (cents < 0) return "negative";
  return "";
}

export function cell(content, className) {
  const td = document.createElement("td");
  if (content instanceof Node) td.append(content);
  else td.textContent = content;
  if (className) td.className = className;
  return td;
}

export function formError(form, message) {
  const error = form.querySelector(".form-error");
  error.textContent = message ?? "";
  error.hidden = !message;
}

// busy disables a container's buttons while an API call is in flight.
export async function busy(container, work) {
  const buttons = container.querySelectorAll("button");
  buttons.forEach((button) => (button.disabled = true));

  try {
    return await work();
  } finally {
    buttons.forEach((button) => (button.disabled = false));
  }
}

const status = document.getElementById("status");
let dbPath = "";

export function setDBPath(path) {
  dbPath = path;
}

export function setStatus(message) {
  status.textContent = dbPath ? `${dbPath} · ${message}` : message;
}

export function loadedStatus(loadedAt) {
  setStatus(`updated ${new Date(loadedAt).toLocaleString()}`);
}

// clickableRow makes a table row open something on click, Enter or Space.
export function clickableRow(row, open) {
  row.tabIndex = 0;
  row.addEventListener("click", open);
  row.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      open();
    }
  });
}

const confirm = {
  dialog: document.getElementById("confirm-dialog"),
  title: document.getElementById("confirm-title"),
  message: document.getElementById("confirm-message"),
  ok: document.getElementById("confirm-ok"),
  action: null,
};

confirm.ok.addEventListener("click", async () => {
  try {
    await busy(confirm.dialog, confirm.action);
    confirm.dialog.close();
  } catch (err) {
    formError(confirm.dialog, String(err));
  }
});

// confirmDelete asks before running action; the dialog closes once action
// succeeds and shows its error otherwise.
export function confirmDelete(title, message, action) {
  confirm.title.textContent = title;
  confirm.message.textContent = message;
  confirm.action = action;
  formError(confirm.dialog, "");
  confirm.dialog.showModal();
}

export function badge(text) {
  const span = document.createElement("span");
  span.className = "badge";
  span.textContent = text;
  return span;
}

// fillSelect replaces a <select>'s options with values (label = value).
export function fillSelect(select, values, labels = values) {
  select.replaceChildren(...values.map((value, index) => new Option(labels[index], value)));
}
