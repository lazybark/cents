// Formatting and DOM helpers shared by every view.

// Decimal fields (inputmode="decimal") use a dot as the decimal separator,
// so a typed or pasted comma turns into one as it's entered: "12,5" reads
// "12.5" right away. It's done here, visibly, rather than guessed when
// saving, where "1,000" could mean a thousand or one.
document.addEventListener("input", (event) => {
  const input = event.target;
  if (!(input instanceof HTMLInputElement) || input.inputMode !== "decimal" || !input.value.includes(",")) return;

  // Same length, so the caret stays where it was.
  const start = input.selectionStart;
  const end = input.selectionEnd;
  input.value = input.value.replaceAll(",", ".");
  input.setSelectionRange(start, end);
});

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

// signedMoney reads like "+$ 10.00" or "-$ 10.00".
export function signedMoney(currency, cents) {
  return (cents > 0 ? "+" : "") + formatMoney(currency, cents);
}

export function percent(part, total) {
  return total > 0 ? Math.min(100, Math.max(0, (part / total) * 100)) : 0;
}

// setProgress fills a .progress-fill bar to paid/total.
export function setProgress(fill, paid, total) {
  fill.style.width = `${percent(paid, total)}%`;
}

// progressText reads like "$ 300.00 of $ 1000.00 (30.0%)".
export function progressText(currency, paid, total) {
  const text = `${formatMoney(currency, paid)} of ${formatMoney(currency, total)}`;
  return total > 0 ? `${text} (${percent(paid, total).toFixed(1)}%)` : text;
}

// rowAction is the small text button at the end of a table row.
export function rowAction(label, onClick) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "row-action";
  button.textContent = label;
  button.addEventListener("click", onClick);
  return button;
}

// renderPaymentLogs fills a When / Note / Change table of logged payments.
// With onDelete, each row also gets a Delete button calling onDelete(entry).
export function renderPaymentLogs(tbody, empty, logs, currency, onDelete) {
  empty.hidden = logs.length > 0;
  tbody.replaceChildren(
    ...logs.map((entry) => {
      const row = document.createElement("tr");
      row.append(
        cell(entry.when, "nowrap"),
        cell(entry.note, "muted wrap"),
        cell(signedMoney(currency, entry.deltaCents), `num ${signClass(entry.deltaCents)}`),
      );
      if (onDelete) row.append(cell(rowAction("Delete", () => onDelete(entry)), "num"));
      return row;
    }),
  );
}

// dueCell shows a due date, flagged when it has passed on an unpaid item.
export function dueCell(dueDate, isOverdue) {
  if (!dueDate) return cell("—", "muted");

  const content = document.createElement("span");
  content.append(dueDate);
  if (isOverdue) content.append(badge("overdue"));

  return cell(content, isOverdue ? "nowrap negative" : "nowrap");
}

// --- recorded rates --------------------------------------------------------
// Dated records keep the rate to the base currency they were entered with.
// A form's rate field is a .rate-field label holding a .rate-label and an
// input named rate; it is hidden for the base currency, whose rate is 1.

// rateText shows a rate without float noise, like "1.08"; empty for none.
export function rateText(rate) {
  return rate > 0 ? String(Number(rate.toFixed(6))) : "";
}

function sameCurrency(a, b) {
  return (a ?? "").trim().toLowerCase() === (b ?? "").trim().toLowerCase();
}

// showRate shows form's rate field for currency with rate filled in, or
// hides it when there is nothing to convert.
export function showRate(form, baseCurrency, currency, rate, hide) {
  const field = form.querySelector(".rate-field");
  field.hidden = Boolean(hide);
  form.querySelector(".rate-label").textContent = `Rate: 1 ${currency} in ${baseCurrency}`;
  form.elements.rate.value = rateText(rate);
}

// syncRate shows form's rate field for the currency picked in it, starting
// with that currency's rate in settings now. rates lists {name, rate} with
// the base currency first.
export function syncRate(form, rates) {
  const currency = form.elements.currency.value;
  const option = rates.find((r) => sameCurrency(r.name, currency));
  const base = rates[0]?.name ?? "";

  showRate(form, base, currency, option?.rate ?? 0, !currency || sameCurrency(currency, base));
}

// rateInput is what to send as a form's rate: nothing while it's hidden.
export function rateInput(form) {
  return form.querySelector(".rate-field").hidden ? "" : form.elements.rate.value;
}

// withBase shows an amount in its currency and, for another currency, the
// amount in the base currency at the record's rate below it (or "no rate").
// format turns (currency, cents) into text; it defaults to formatMoney.
export function withBase(record, cents, baseCurrency, baseCents, format = formatMoney) {
  const content = document.createElement("div");
  content.append(format(record.currency, cents));

  if (!record.isBase && record.currency) {
    const base = document.createElement("div");
    base.className = "account-description";
    base.textContent = record.hasRate === false ? "no rate" : format(baseCurrency, baseCents);
    content.append(base);
  }

  return content;
}
