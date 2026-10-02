// "Also add as an expense" in a debt or credit payment form: a checkbox
// (addCashflow) that, ticked, shows a category and an optional account
// (cashflowCategory, cashflowAccount) for the entry made with the payment.
// Choices are remembered per kind, so a run of payments is one click each.
import { fillSelect } from "./ui.js";

const STORAGE_KEY = "cents.paymentCashflow";

function remembered() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "{}") ?? {};
  } catch {
    return {};
  }
}

function remember(choice) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...remembered(), ...choice }));
  } catch {
    // Not remembered; the box just starts unticked next time.
  }
}

function parts(form) {
  return {
    box: form.querySelector(".cashflow-box"),
    check: form.elements.addCashflow,
    label: form.querySelector(".cashflow-label"),
    fields: form.querySelector(".cashflow-fields"),
    none: form.querySelector(".cashflow-none"),
    category: form.elements.cashflowCategory,
    account: form.elements.cashflowAccount,
  };
}

// setupCashflow fills the box for a payment that is an income (isIncome)
// or an expense, from options (CashflowOptions from the Go side).
export function setupCashflow(form, { isIncome, options }) {
  const p = parts(form);
  const kind = isIncome ? "income" : "expense";
  const categories = (isIncome ? options?.incomeCategories : options?.expenseCategories) ?? [];
  const accounts = options?.accounts ?? [];
  const saved = remembered();

  p.box.dataset.kind = kind;
  p.label.textContent = `Also add as an ${kind}`;
  fillSelect(p.category, categories);
  fillSelect(p.account, ["", ...accounts], ["— none —", ...accounts]);

  if (categories.includes(saved[kind])) p.category.value = saved[kind];
  if (accounts.includes(saved.account)) p.account.value = saved.account;

  // An entry needs a category, so without any the box can't be ticked.
  const usable = categories.length > 0;
  p.check.disabled = !usable;
  p.check.checked = usable && Boolean(saved[`add-${kind}`]);
  p.none.hidden = usable;
  p.none.textContent = `Add an ${kind} category in Settings to add payments as ${kind}s.`;

  if (!p.check.dataset.wired) {
    p.check.dataset.wired = "1";
    p.check.addEventListener("change", () => sync(form));
  }

  sync(form);
}

function sync(form) {
  const p = parts(form);
  p.fields.hidden = !p.check.checked;
}

// showCashflow shows or hides the whole box (a credit addition isn't a
// payment, say).
export function showCashflow(form, visible) {
  parts(form).box.hidden = !visible;
}

// cashflowInput is what to send for the box, remembering the choice.
export function cashflowInput(form) {
  const p = parts(form);
  const kind = p.box.dataset.kind;
  const add = !p.box.hidden && p.check.checked;

  remember({ [`add-${kind}`]: p.check.checked, [kind]: p.category.value, account: p.account.value });

  return { add, category: add ? p.category.value : "", account: add ? p.account.value : "" };
}
