// cents Mini App: this month, budgets and payments coming up; adding,
// editing and deleting entries; accounts and their balances; budgets,
// subscriptions, obligations and investments. It calls the cents API (the same as every
// client's) through the bot, signed in by Telegram.
const tg = window.Telegram?.WebApp;
const $ = (id) => document.getElementById(id);

const state = {
  screen: "home",
  // The month shown in Entries ("YYYY-MM"), and its data.
  month: "",
  options: null,
  base: "",
  // The entry being edited, or null when adding.
  editing: null,
  // Accounts as last loaded; the one open; whether archived ones show.
  accounts: null,
  account: null,
  showArchived: false,
  // Subscriptions or obligations: which kind, active or all, as last
  // loaded, and the one open (null for a new one).
  payKind: "subscription",
  payMode: "active",
  payments: null,
  payment: null,
  // Budgets: the month shown, as last loaded, and the one open.
  budgetMonth: "",
  budgets: null,
  budget: null,
  // Investments as last loaded, and the one open.
  investments: null,
  investment: null,
};

// Screens reached from another: where Back goes (and so which tab they
// belong to).
const parents = {
  account: "accounts",
  "new-account": "accounts",
  payments: "more",
  payment: "payments",
  budgets: "more",
  budget: "budgets",
  investments: "more",
  investment: "investments",
};
const screens = ["home", "add", "entries", "accounts", "account", "new-account", "more", "payments", "payment", "budgets", "budget", "investments", "investment"];

function tabOf(screen) {
  while (parents[screen]) screen = parents[screen];
  return screen;
}

// --- the API -----------------------------------------------------------------

async function api(method, path, body) {
  const res = await fetch(`api/v1${path}`, {
    method,
    headers: { Authorization: `tma ${tg?.initData ?? ""}`, ...(body ? { "Content-Type": "application/json" } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return null;
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new Error(data?.error?.message ?? `the server answered ${res.status}`);
  return data;
}

// --- formatting ------------------------------------------------------------------

// money formats cents like the desktop app: "-€ 12.34".
function money(currency, cents) {
  const sign = cents < 0 ? "-" : "";
  const amount = (Math.abs(cents) / 100).toFixed(2);
  return currency ? `${sign}${currency} ${amount}` : `${sign}${amount}`;
}

function monthName(month) {
  const [year, index] = month.split("-").map(Number);
  return new Date(year, index - 1, 1).toLocaleString(undefined, { month: "long", year: "numeric" });
}

function shiftMonth(month, delta) {
  const now = new Date();
  const [year, index] = month ? month.split("-").map(Number) : [now.getFullYear(), now.getMonth() + 1];
  const date = new Date(year, index - 1 + delta, 1);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function today() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function dayName(date) {
  const [year, month, day] = date.split("-").map(Number);
  const d = new Date(year, month - 1, day);
  if (date === today()) return "Today";
  return d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });
}

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function item(name, amount, amountClass = "") {
  const row = element("div", "item");
  row.append(element("span", "item-name", name), element("span", `item-amount ${amountClass}`.trim(), amount));
  return row;
}

function haptic(kind) {
  try {
    tg?.HapticFeedback?.notificationOccurred(kind);
  } catch {
    // Not in Telegram.
  }
}

function showMessage(text) {
  $("message").textContent = text;
  $("message").hidden = !text;
}

// --- screens -----------------------------------------------------------------------

async function show(screen) {
  state.screen = screen;
  for (const name of screens) $(name).hidden = name !== screen;
  const tab = tabOf(screen);
  for (const button of document.querySelectorAll(".tabs button")) {
    if (button.dataset.screen === tab) button.setAttribute("aria-current", "page");
    else button.removeAttribute("aria-current");
  }

  // Telegram's back button leaves an edit, an account, a new account.
  if ((state.editing && screen === "add") || parents[screen]) tg?.BackButton?.show();
  else tg?.BackButton?.hide();

  showMessage("");
  window.scrollTo(0, 0);
  try {
    if (screen === "home") await showHome();
    if (screen === "entries") await showEntries();
    if (screen === "add") await showForm();
    if (screen === "accounts") await showAccounts();
    if (screen === "account") await showAccount();
    if (screen === "new-account") await showNewAccount();
    if (screen === "more") await showMore();
    if (screen === "payments") await showPayments();
    if (screen === "payment") await showPayment();
    if (screen === "budgets") await showBudgets();
    if (screen === "budget") await showBudget();
    if (screen === "investments") await showInvestments();
    if (screen === "investment") await showInvestment();
  } catch (err) {
    showMessage(String(err.message ?? err));
  }
}

async function showHome() {
  const [month, overview] = await Promise.all([api("GET", "/cashflow/months/current"), api("GET", "/overview")]);
  const base = month.baseCurrency;
  remember(month);

  $("home-month").textContent = monthName(month.month);
  $("home-net").textContent = money(base, month.netCents);
  $("home-net").className = `figure ${month.netCents < 0 ? "negative" : ""}`;
  $("home-income").textContent = money(base, month.incomeCents);
  $("home-expenses").textContent = money(base, month.expenseCents);
  $("home-missing").hidden = !month.missingRates;
  $("home-missing").textContent = `${month.missingRates} entries have no rate to ${base} and aren't counted.`;
  $("home-worth").textContent = money(overview.baseCurrency, overview.netWorthCents);

  const budgets = overview.budgets ?? [];
  $("home-budgets").hidden = budgets.length === 0;
  $("budget-list").replaceChildren(
    ...budgets.map((row) => {
      const li = element("li");
      const bar = element("div", `bar ${row.state}`);
      const fill = element("span");
      fill.style.width = `${Math.min(100, row.percent)}%`;
      bar.append(fill);
      const button = element("button", "entry");
      button.type = "button";
      button.append(item(row.name, `${money(base, row.spentCents)} / ${money(base, row.limitCents)}`, row.state === "over" ? "negative" : ""), bar);
      button.addEventListener("click", () => {
        state.budgetMonth = "";
        state.budgets = null;
        state.budget = row;
        show("budget");
      });
      li.append(button);
      return li;
    }),
  );

  const upcoming = (overview.upcoming ?? []).slice(0, 5);
  $("home-upcoming").hidden = upcoming.length === 0;
  $("upcoming-list").replaceChildren(
    ...upcoming.map((sub) => {
      const li = element("li");
      const when = sub.dueInDays === 0 ? "today" : sub.dueInDays === 1 ? "tomorrow" : sub.dueInDays < 0 ? `${-sub.dueInDays} days overdue` : `in ${sub.dueInDays} days`;
      const button = element("button", "entry");
      button.type = "button";
      button.append(item(sub.name, money(sub.currency, sub.amountCents)), element("div", "item-note", when));
      button.addEventListener("click", () => openPayment(sub));
      li.append(button);
      return li;
    }),
  );
}

// remember keeps what the form offers (currencies, categories, accounts).
function remember(month) {
  state.options = month.options;
  state.base = month.baseCurrency;
}

async function showEntries() {
  const data = await api("GET", `/cashflow/months/${state.month || "current"}`);
  state.month = data.month;
  remember(data);

  const base = data.baseCurrency;
  $("entries-month").textContent = monthName(data.month);
  $("entries-totals").textContent = `Income ${money(base, data.incomeCents)} · Expenses ${money(base, data.expenseCents)}`;

  const entries = [...data.entries].sort((a, b) => b.date.localeCompare(a.date) || b.id - a.id);
  $("entries-empty").hidden = entries.length > 0;

  const days = [];
  for (const entry of entries) {
    let day = days.find((d) => d.date === entry.date);
    if (!day) {
      day = { date: entry.date, entries: [] };
      days.push(day);
    }
    day.entries.push(entry);
  }

  $("entry-days").replaceChildren(
    ...days.flatMap((day) => {
      const list = element("ul", "list card");
      for (const entry of day.entries) {
        const li = element("li");
        const button = element("button", "entry");
        button.type = "button";
        const amount = money(entry.currency, entry.amountCents);
        button.append(item(entry.category || "No category", entry.isIncome ? `+${amount}` : amount, entry.isIncome ? "positive" : ""));
        const note = [entry.comment, ...(entry.tags ?? []).map((t) => `#${t}`), entry.account].filter(Boolean).join(" · ");
        if (note) button.append(element("div", "item-note", note));
        button.addEventListener("click", () => edit(entry));
        li.append(button);
        list.append(li);
      }
      return [element("h3", "day", dayName(day.date)), list];
    }),
  );
}

// --- the form ----------------------------------------------------------------------

const form = $("entry-form");

function kind() {
  return form.elements.kind.value;
}

async function showForm() {
  if (!state.options) remember(await api("GET", "/cashflow/months/current"));

  const entry = state.editing;
  $("form-title").textContent = entry ? "Edit entry" : "Add";
  $("delete").hidden = !entry;
  $("save").textContent = entry ? "Save changes" : "Save";
  disarmDelete();
  formError("");

  if (entry) {
    form.elements.kind.value = entry.isIncome ? "income" : "expense";
    form.elements.amount.value = (entry.amountCents / 100).toFixed(2);
    form.elements.comment.value = entry.comment;
    form.elements.date.value = entry.date;
    form.elements.tags.value = (entry.tags ?? []).join(", ");
  }

  fillSelect(form.elements.currency, state.options.currencies, entry ? entry.currency : state.base);
  fillSelect(form.elements.account, ["", ...state.options.accounts], entry ? entry.account : "", "No account");
  renderCategories(entry?.category ?? "");
  if (!entry) form.elements.amount.focus();
}

function fillSelect(select, values, selected, emptyLabel = "") {
  // An edited entry may keep a value no longer offered.
  const all = selected && !values.includes(selected) ? [...values, selected] : values;
  select.replaceChildren(
    ...all.map((value) => {
      const option = element("option", "", value || emptyLabel);
      option.value = value;
      option.selected = value === selected;
      return option;
    }),
  );
}

function renderCategories(selected) {
  const names = kind() === "income" ? state.options.incomeCategories : state.options.expenseCategories;
  const all = selected && !names.includes(selected) ? [...names, selected] : names;
  $("categories").replaceChildren(
    ...all.map((name) => {
      const label = element("label");
      const input = element("input");
      input.type = "radio";
      input.name = "category";
      input.value = name;
      input.checked = name === selected;
      label.append(input, element("span", "", name));
      return label;
    }),
  );
}

function resetForm() {
  form.reset();
  form.elements.date.value = today();
}

function formError(text) {
  $("form-error").textContent = text;
  $("form-error").hidden = !text;
}

function edit(entry) {
  state.editing = entry;
  show("add");
}

async function save(event) {
  event.preventDefault();
  formError("");
  const f = form.elements;
  const category = form.querySelector('input[name="category"]:checked')?.value ?? "";
  if (!category) {
    formError("Pick a category.");
    return;
  }

  const input = {
    isIncome: kind() === "income",
    currency: f.currency.value,
    rate: "",
    amount: f.amount.value.trim().replace(",", "."),
    date: f.date.value,
    category,
    account: f.account.value,
    comment: f.comment.value.trim(),
    tags: f.tags.value
      .split(/[\s,]+/)
      .map((t) => t.replace(/^#/, ""))
      .filter(Boolean),
  };

  $("save").disabled = true;
  try {
    const result = state.editing
      ? await api("PUT", `/cashflow/entries/${state.editing.id}`, input)
      : await api("POST", "/cashflow/entries", input);
    haptic("success");
    state.editing = null;
    state.month = result.month;
    resetForm();
    await show("entries");
  } catch (err) {
    haptic("error");
    formError(String(err.message ?? err));
  } finally {
    $("save").disabled = false;
  }
}

// twoTaps makes button run action on a second tap within a few seconds,
// so a stray tap deletes nothing. It returns a function that disarms it.
function twoTaps(button, action) {
  const label = button.textContent;
  let armed = false;
  let timer = 0;
  const disarm = () => {
    armed = false;
    clearTimeout(timer);
    button.textContent = label;
  };

  button.addEventListener("click", () => {
    if (!armed) {
      armed = true;
      button.textContent = "Tap again to delete";
      timer = setTimeout(disarm, 4000);
      return;
    }

    disarm();
    action();
  });

  return disarm;
}

const disarmDelete = twoTaps($("delete"), remove);

async function remove() {
  try {
    await api("DELETE", `/cashflow/entries/${state.editing.id}`);
    haptic("success");
    state.editing = null;
    resetForm();
    await show("entries");
  } catch (err) {
    formError(String(err.message ?? err));
  }
}

// --- accounts ------------------------------------------------------------------------

// ago says how long ago an ISO time was: "today", "3 days ago", or the
// date. Records never updated carry a 1970 placeholder: "".
function ago(iso) {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime()) || at.getFullYear() < 1971) return "";
  const days = Math.floor((Date.now() - at.getTime()) / 86400000);
  if (days <= 0) return "updated today";
  if (days === 1) return "updated yesterday";
  if (days < 60) return `updated ${days} days ago`;
  return `updated ${at.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" })}`;
}

async function loadAccounts() {
  state.accounts = await api("GET", "/accounts?sort=0");
  return state.accounts;
}

function accountRow(account, base) {
  const li = element("li");
  const button = element("button", "entry");
  button.type = "button";
  button.append(item(account.name, money(account.currency, account.balanceCents)));
  const note = [account.description];
  if (account.currency !== base && account.hasRate) note.push(`≈ ${money(base, account.baseCents)}`);
  if (account.ignoreInSummaries) note.push("not in totals");
  note.push(ago(account.lastUpdatedAt));
  button.append(element("div", "item-note", note.filter(Boolean).join(" · ")));
  button.addEventListener("click", () => {
    state.account = account;
    show("account");
  });
  li.append(button);
  return li;
}

async function showAccounts() {
  const view = await loadAccounts();
  const base = view.baseCurrency;
  $("accounts-total").textContent = money(base, view.totalCents);

  // The base currency's total is the main figure's part: others show theirs.
  $("currency-totals").replaceChildren(
    ...view.currencyTotals.map((total) => {
      const li = element("li");
      li.append(item(money(total.currency, total.cents), total.hasRate ? `≈ ${money(base, total.baseCents)}` : "no rate", "item-note"));
      return li;
    }),
  );

  const notes = [];
  if (view.ignoredCount) notes.push(`${view.ignoredCount} not counted in the total`);
  if (view.missingRates) notes.push(`${view.missingRates} without a rate to ${base}`);
  $("accounts-note").textContent = notes.join("; ");
  $("accounts-note").hidden = notes.length === 0;

  const active = view.accounts.filter((a) => !a.archived);
  const archived = view.accounts.filter((a) => a.archived);
  $("account-list").replaceChildren(...active.map((a) => accountRow(a, base)));
  $("account-list").hidden = active.length === 0;
  $("archived-toggle").hidden = archived.length === 0;
  $("archived-toggle").textContent = `${state.showArchived ? "Hide" : "Show"} archived (${archived.length})`;
  $("archived-list").replaceChildren(...archived.map((a) => accountRow(a, base)));
  $("archived-list").hidden = !state.showArchived || archived.length === 0;
}

async function showAccount() {
  // Fresh from the server, with its history.
  const [view, history] = await Promise.all([loadAccounts(), api("GET", `/accounts/${state.account.id}/values`)]);
  const account = view.accounts.find((a) => a.id === state.account.id);
  if (!account) {
    show("accounts");
    return;
  }

  state.account = account;
  const base = view.baseCurrency;
  $("account-name").textContent = account.name;
  $("account-description").textContent = account.description;
  $("account-balance").textContent = money(account.currency, account.balanceCents);
  $("account-base").textContent = account.currency !== base && account.hasRate ? `≈ ${money(base, account.baseCents)}` : "";
  $("account-updated").textContent = ago(account.lastUpdatedAt);
  $("account-counted").checked = !account.ignoreInSummaries;
  $("account-archived").checked = account.archived;

  const balance = $("balance-form");
  balance.elements.amount.value = (account.balanceCents / 100).toFixed(2);
  balance.elements.log.checked = true;
  formErrorIn(balance, "");
  accountError("");
  disarmAccountDelete();

  renderHistory($("history-card"), $("history-list"), history, account.currency);
}

// renderHistory lists values newest first, each with how it changed from
// the one before; card hides when there are none.
function renderHistory(card, list, history, currency) {
  card.hidden = history.length === 0;
  list.replaceChildren(
    ...history.slice(0, 12).map((log, i) => {
      const li = element("li");
      li.append(item(dayName(log.date), money(currency, log.valueCents)));
      const before = history[i + 1];
      if (before) {
        const delta = log.valueCents - before.valueCents;
        li.append(element("div", `item-note ${delta < 0 ? "negative" : delta > 0 ? "positive" : ""}`.trim(), `${delta > 0 ? "+" : ""}${money(currency, delta)}`));
      }
      return li;
    }),
  );
}

function formErrorIn(f, text) {
  const error = f.querySelector(".error");
  error.textContent = text;
  error.hidden = !text;
}

function accountError(text) {
  $("account-error").textContent = text;
  $("account-error").hidden = !text;
}

// updateAccount saves the account's balance and switches; fields change
// some of them, the rest stay as they are.
async function updateAccount(fields) {
  const account = state.account;
  const result = await api("PUT", `/accounts/${account.id}`, {
    amount: (account.balanceCents / 100).toFixed(2),
    ignoreInSummaries: account.ignoreInSummaries,
    archived: account.archived,
    updateLog: false,
    ...fields,
  });
  haptic("success");
  await showAccount();
  if (result?.warning) accountError(result.warning);
}

async function saveBalance(event) {
  event.preventDefault();
  const f = $("balance-form").elements;
  formErrorIn($("balance-form"), "");
  try {
    await updateAccount({ amount: f.amount.value.trim().replace(",", "."), updateLog: f.log.checked });
  } catch (err) {
    haptic("error");
    formErrorIn($("balance-form"), String(err.message ?? err));
  }
}

async function toggleAccount(fields) {
  accountError("");
  try {
    await updateAccount(fields);
  } catch (err) {
    haptic("error");
    accountError(String(err.message ?? err));
    await showAccount();
  }
}

const disarmAccountDelete = twoTaps($("account-delete"), deleteAccount);

async function deleteAccount() {
  try {
    await api("DELETE", `/accounts/${state.account.id}`);
    haptic("success");
    state.account = null;
    await show("accounts");
  } catch (err) {
    haptic("error");
    accountError(String(err.message ?? err));
  }
}

async function showNewAccount() {
  const view = state.accounts ?? (await loadAccounts());
  const f = $("account-form");
  f.reset();
  formErrorIn(f, "");
  fillSelect(f.elements.currency, view.currencies, view.baseCurrency);
  f.elements.name.focus();
}

async function addAccount(event) {
  event.preventDefault();
  const f = $("account-form");
  formErrorIn(f, "");
  try {
    await api("POST", "/accounts", {
      name: f.elements.name.value.trim(),
      description: f.elements.description.value.trim(),
      currency: f.elements.currency.value,
      amount: f.elements.amount.value.trim().replace(",", ".") || "0",
      ignoreInSummaries: !f.elements.counted.checked,
    });
    haptic("success");
    await show("accounts");
  } catch (err) {
    haptic("error");
    formErrorIn(f, String(err.message ?? err));
  }
}

// --- subscriptions and obligations ---------------------------------------------------

const kinds = {
  subscription: { title: "Subscriptions", one: "subscription" },
  obligation: { title: "Obligations", one: "obligation" },
};

const periodNames = { week: "a week", month: "a month", quarter: "a quarter", year: "a year" };

// due says when a payment is due: "tomorrow", "in 5 days", "3 days
// overdue", or that it has no date.
function due(row) {
  if (!row.nextPayment) return "no date set";
  const days = row.dueInDays;
  if (days === 0) return "today";
  if (days === 1) return "tomorrow";
  if (days < 0) return `${-days} days overdue`;
  return `in ${days} days`;
}

async function showMore() {
  const [subscriptions, obligations, budgets, investments] = await Promise.all([
    api("GET", "/subscriptions?mode=active&kind=subscription"),
    api("GET", "/subscriptions?mode=active&kind=obligation"),
    api("GET", "/budgets"),
    api("GET", "/assets?kind=investment"),
  ]);

  for (const [kind, view] of [["subscription", subscriptions], ["obligation", obligations]]) {
    $(`more-${kind}`).textContent = `${money(view.baseCurrency, view.active.monthlyCents)} a month`;
  }

  // Budgets: how many are over, or else how all spending stands.
  const over = budgets.rows.filter((row) => row.state === "over").length;
  const total = budgets.rows.find((row) => row.isTotal);
  $("more-budgets").textContent = over
    ? `${over} over`
    : total
      ? `${Math.round(total.percent)}% used`
      : budgets.rows.length
        ? `${budgets.rows.length} set`
        : "";
  $("more-budgets").className = `item-note ${over ? "negative" : ""}`.trim();
  $("more-investments").textContent = investments.assets.length ? money(investments.baseCurrency, investments.totalCents) : "";
}

async function loadPayments(mode = state.payMode) {
  state.payments = await api("GET", `/subscriptions?mode=${mode}&kind=${state.payKind}`);
  return state.payments;
}

async function showPayments() {
  const view = await loadPayments();
  const base = view.baseCurrency;
  const kind = kinds[state.payKind];
  $("payments-title").textContent = kind.title;
  $("payments-monthly").textContent = money(base, view.active.monthlyCents);
  $("payments-yearly").textContent = money(base, view.active.yearlyCents);
  $("payment-new").textContent = `New ${kind.one}`;
  for (const button of document.querySelectorAll(".filter button")) button.setAttribute("aria-pressed", String(button.dataset.mode === state.payMode));

  $("payments-empty").hidden = view.subscriptions.length > 0;
  $("payment-list").hidden = view.subscriptions.length === 0;
  $("payment-list").replaceChildren(
    ...view.subscriptions.map((row) => {
      const li = element("li", row.isActive ? "" : "inactive");
      const button = element("button", "entry");
      button.type = "button";
      button.append(item(row.name, `${money(row.currency, row.amountCents)} ${periodNames[row.period] ?? ""}`.trim()));
      const note = [row.isActive ? due(row) : "inactive", row.type, row.paymentMethod];
      if (row.paidManually && row.isActive) note.push("paid by hand");
      button.append(element("div", `item-note ${row.isActive && row.nextPayment && row.dueInDays < 0 ? "negative" : ""}`.trim(), note.filter(Boolean).join(" · ")));
      button.addEventListener("click", () => openPayment(row));
      li.append(button);
      return li;
    }),
  );
}

function openPayment(row) {
  state.payKind = row.isObligation ? "obligation" : "subscription";
  state.payment = row;
  show("payment");
}

async function showPayment() {
  // Every one of the kind, to find this one fresh (and the options).
  const view = await loadPayments("all");
  if (state.payment) {
    state.payment = view.subscriptions.find((row) => row.id === state.payment.id) ?? null;
    if (!state.payment) {
      show("payments");
      return;
    }
  }

  const row = state.payment;
  const f = $("payment-form");
  const options = view.options;
  f.reset();
  formErrorIn(f, "");
  disarmPaymentDelete();
  $("payment-title").textContent = row ? row.name : `New ${kinds[state.payKind].one}`;
  $("payment-delete").hidden = !row;

  f.elements.name.value = row?.name ?? "";
  f.elements.amount.value = row ? (row.amountCents / 100).toFixed(2) : "";
  f.elements.type.value = row?.type ?? "";
  f.elements.paymentMethod.value = row?.paymentMethod ?? "";
  f.elements.nextPayment.value = row?.anchor || row?.nextPayment || "";
  f.elements.isActive.checked = row ? row.isActive : true;
  f.elements.paidManually.checked = row ? row.paidManually : state.payKind === "obligation";
  f.elements.isObligation.checked = state.payKind === "obligation";
  fillSelect(f.elements.currency, options.currencies, row?.currency ?? view.baseCurrency);
  f.elements.period.replaceChildren(
    ...options.periods.map((period) => {
      const option = element("option", "", `Every ${period}`);
      option.value = period;
      option.selected = period === (row?.period ?? "month");
      return option;
    }),
  );
  $("payment-types").replaceChildren(...options.types.map((type) => Object.assign(element("option"), { value: type })));
  $("payment-methods").replaceChildren(...options.paymentMethods.map((method) => Object.assign(element("option"), { value: method })));

  // Marking paid, for one with a next payment.
  $("paid-card").hidden = !row || !row.isActive || !row.nextPayment;
  if (!$("paid-card").hidden) {
    $("paid-next").textContent = `${dayName(row.nextPayment)}, ${due(row)}`;
    $("paid-next").className = row.dueInDays < 0 ? "negative" : "";
  }

  $("paid-list").replaceChildren();
  if (row) {
    const paid = await api("GET", `/subscriptions/${row.id}/payments`);
    $("paid-list").replaceChildren(
      ...paid.slice(0, 5).map((payment, i) => {
        const li = element("li");
        const line = item(`Paid for ${dayName(payment.paidFor)}`, "");
        // The latest can be taken back, if marked by mistake.
        if (i === 0) {
          const undo = element("button", "link inline", "Undo");
          undo.type = "button";
          undo.addEventListener("click", () => unmarkPaid(payment));
          line.lastChild.replaceChildren(undo);
        }
        li.append(line);
        return li;
      }),
    );
  }
}

// A payment method paid in one currency sets it.
$("payment-form").elements.paymentMethod.addEventListener("change", (event) => {
  const currency = state.payments?.options.paymentMethodCurrencies?.[event.target.value];
  if (currency) $("payment-form").elements.currency.value = currency;
});

async function savePayment(event) {
  event.preventDefault();
  const f = $("payment-form");
  formErrorIn(f, "");
  const e = f.elements;
  const input = {
    name: e.name.value.trim(),
    type: e.type.value.trim(),
    currency: e.currency.value,
    amount: e.amount.value.trim().replace(",", "."),
    period: e.period.value,
    paymentMethod: e.paymentMethod.value.trim(),
    nextPayment: e.nextPayment.value,
    isActive: e.isActive.checked,
    isObligation: e.isObligation.checked,
    paidManually: e.paidManually.checked,
  };

  try {
    if (state.payment) await api("PUT", `/subscriptions/${state.payment.id}`, input);
    else await api("POST", "/subscriptions", input);
    haptic("success");
    state.payKind = input.isObligation ? "obligation" : "subscription";
    state.payment = null;
    await show("payments");
  } catch (err) {
    haptic("error");
    formErrorIn(f, String(err.message ?? err));
  }
}

async function markPaid() {
  try {
    const row = await api("POST", `/subscriptions/${state.payment.id}/payments`);
    haptic("success");
    state.payment = row;
    await showPayment();
  } catch (err) {
    haptic("error");
    formErrorIn($("payment-form"), String(err.message ?? err));
  }
}

async function unmarkPaid(payment) {
  try {
    await api("DELETE", `/subscriptions/${state.payment.id}/payments/${payment.id}`);
    haptic("success");
    await showPayment();
  } catch (err) {
    formErrorIn($("payment-form"), String(err.message ?? err));
  }
}

const disarmPaymentDelete = twoTaps($("payment-delete"), async () => {
  try {
    await api("DELETE", `/subscriptions/${state.payment.id}`);
    haptic("success");
    state.payment = null;
    await show("payments");
  } catch (err) {
    formErrorIn($("payment-form"), String(err.message ?? err));
  }
});

// --- budgets --------------------------------------------------------------------------

async function showBudgets() {
  const view = await api("GET", `/budgets${state.budgetMonth ? `?month=${state.budgetMonth}` : ""}`);
  state.budgets = view;
  state.budgetMonth = view.month;
  const base = view.baseCurrency;

  $("budgets-month").textContent = monthName(view.month);
  $("budgets-days").textContent = view.isCurrent ? `Day ${view.daysIn} of ${view.daysInMonth}` : "Compared with today's limits.";
  $("budgets-empty").hidden = view.rows.length > 0;
  $("budgets-list").hidden = view.rows.length === 0;
  $("budgets-missing").hidden = !view.missingRates;
  $("budgets-missing").textContent = `${view.missingRates} entries have no rate to ${base} and aren't counted.`;
  $("budget-new").hidden = view.options.categories.length === 0 && view.options.hasTotal;

  $("budgets-list").replaceChildren(
    ...view.rows.map((row) => {
      const li = element("li");
      const button = element("button", "entry");
      button.type = "button";
      button.append(item(row.name, `${money(base, row.spentCents)} / ${money(base, row.limitCents)}`, row.state === "over" ? "negative" : ""));
      const bar = element("div", `bar ${row.state}`);
      const fill = element("span");
      fill.style.width = `${Math.min(100, row.percent)}%`;
      bar.append(fill);
      const note = [row.leftCents >= 0 ? `${money(base, row.leftCents)} left` : `${money(base, -row.leftCents)} over`];
      if (row.usualCents) note.push(`usually ${money(base, row.usualCents)}`);
      if (row.archived) note.push("category archived");
      button.append(bar, element("div", "item-note", note.join(" · ")));
      button.addEventListener("click", () => {
        state.budget = row;
        show("budget");
      });
      li.append(button);
      return li;
    }),
  );
}

async function showBudget() {
  const view = state.budgets ?? (await api("GET", "/budgets"));
  const row = state.budget;
  const f = $("budget-form");
  const base = view.baseCurrency;
  f.reset();
  formErrorIn(f, "");
  disarmBudgetDelete();
  $("budget-title").textContent = row ? row.name : "New budget";
  $("budget-delete").hidden = !row;
  $("budget-category-field").hidden = Boolean(row);

  // A new one can be for all spending (if that has none yet) or a
  // category without one.
  const choices = [...(view.options.hasTotal ? [] : [""]), ...view.options.categories];
  f.elements.category.replaceChildren(
    ...choices.map((category) => {
      const option = element("option", "", category || "All spending");
      option.value = category;
      return option;
    }),
  );

  const usual = () => (row ? row.usualCents : (view.options.usual?.[f.elements.category.value] ?? 0));
  const showUsual = () => {
    const cents = usual();
    $("budget-usual").hidden = !cents;
    $("budget-usual").textContent = `Use your usual: ${money(base, cents)} a month`;
    $("budget-usual").onclick = () => (f.elements.limit.value = (cents / 100).toFixed(2));
  };

  f.elements.category.onchange = showUsual;
  f.elements.limit.value = row ? (row.limitCents / 100).toFixed(2) : "";
  showUsual();
}

async function saveBudget(event) {
  event.preventDefault();
  const f = $("budget-form");
  formErrorIn(f, "");
  const row = state.budget;
  const input = { category: row ? row.category : f.elements.category.value, limit: f.elements.limit.value.trim().replace(",", ".") };

  try {
    if (row) await api("PUT", `/budgets/${row.id}`, input);
    else await api("POST", "/budgets", input);
    haptic("success");
    state.budget = null;
    await show("budgets");
  } catch (err) {
    haptic("error");
    formErrorIn(f, String(err.message ?? err));
  }
}

const disarmBudgetDelete = twoTaps($("budget-delete"), async () => {
  try {
    await api("DELETE", `/budgets/${state.budget.id}`);
    haptic("success");
    state.budget = null;
    await show("budgets");
  } catch (err) {
    formErrorIn($("budget-form"), String(err.message ?? err));
  }
});

// --- investments -----------------------------------------------------------------------

// gain says how an investment did against its cost: "+$ 120.00 (+12%)".
function gain(currency, gainCents, costCents) {
  const percent = costCents ? ` (${gainCents >= 0 ? "+" : ""}${Math.round((gainCents / costCents) * 100)}%)` : "";
  return `${gainCents > 0 ? "+" : ""}${money(currency, gainCents)}${percent}`;
}

async function loadInvestments() {
  state.investments = await api("GET", "/assets?kind=investment");
  return state.investments;
}

async function showInvestments() {
  const view = await loadInvestments();
  const base = view.baseCurrency;
  $("investments-total").textContent = money(base, view.totalCents);
  $("investments-gain-row").hidden = !view.withCostCents;
  $("investments-cost").textContent = money(base, view.costCents);
  $("investments-gain").textContent = gain(base, view.gainCents, view.costCents);
  $("investments-gain").className = view.gainCents < 0 ? "negative" : "positive";
  $("investments-types").replaceChildren(
    ...(view.byType ?? []).map((t) => {
      const li = element("li");
      li.append(item(`${t.type} (${t.count})`, money(base, t.baseCents), "item-note"));
      return li;
    }),
  );

  const notes = [];
  if (view.ignored) notes.push(`${view.ignored} not counted in net worth`);
  if (view.missingRates) notes.push(`${view.missingRates} without a rate to ${base}`);
  $("investments-note").textContent = notes.join("; ");
  $("investments-note").hidden = notes.length === 0;

  $("investments-empty").hidden = view.assets.length > 0;
  $("investment-list").hidden = view.assets.length === 0;
  $("investment-list").replaceChildren(
    ...view.assets.map((row) => {
      const li = element("li");
      const button = element("button", "entry");
      button.type = "button";
      button.append(item(row.name, money(row.currency, row.valueCents)));
      const note = [row.type];
      if (!row.isBase && row.hasRate) note.push(`≈ ${money(base, row.baseCents)}`);
      if (row.hasGain) note.push(gain(row.currency, row.gainCents, row.costCents));
      if (row.ignoreInNetWorth) note.push("not in net worth");
      note.push(ago(row.lastUpdatedAt));
      button.append(element("div", "item-note", note.filter(Boolean).join(" · ")));
      button.addEventListener("click", () => {
        state.investment = row;
        show("investment");
      });
      li.append(button);
      return li;
    }),
  );
}

// assetInput is the API's input for an investment: row's fields, with
// changes.
function assetInput(row, changes) {
  return {
    kind: "investment",
    currency: row.currency,
    name: row.name,
    type: row.type,
    value: (row.valueCents / 100).toFixed(2),
    cost: row.costCents || row.hasGain ? (row.costCents / 100).toFixed(2) : "",
    acquiredAt: row.acquiredAt,
    description: row.description,
    ignoreInNetWorth: row.ignoreInNetWorth,
    ...changes,
  };
}

async function showInvestment() {
  const view = await loadInvestments();
  if (state.investment) {
    state.investment = view.assets.find((a) => a.id === state.investment.id) ?? null;
    if (!state.investment) {
      show("investments");
      return;
    }
  }

  const row = state.investment;
  const base = view.baseCurrency;
  $("investment-value-card").hidden = !row;
  $("investment-new-value").hidden = Boolean(row);
  $("investment-delete").hidden = !row;
  // A new one's form is the screen: its heading is the title.
  $("investment-form-title").textContent = row ? "Details" : "New investment";
  $("investment-form-title").className = row ? "subtitle" : "title";
  disarmInvestmentDelete();
  formErrorIn($("value-form"), "");

  const f = $("investment-form");
  f.reset();
  formErrorIn(f, "");
  f.elements.name.value = row?.name ?? "";
  f.elements.type.value = row?.type ?? "";
  f.elements.cost.value = row && (row.costCents || row.hasGain) ? (row.costCents / 100).toFixed(2) : "";
  f.elements.acquiredAt.value = row?.acquiredAt ?? "";
  f.elements.description.value = row?.description ?? "";
  f.elements.counted.checked = row ? !row.ignoreInNetWorth : true;
  fillSelect(f.elements.currency, view.currencies, row?.currency ?? base);
  $("investment-types-list").replaceChildren(...(view.types ?? []).map((type) => Object.assign(element("option"), { value: type })));

  if (!row) {
    $("investment-history-card").hidden = true;
    return;
  }

  $("investment-name").textContent = row.name;
  $("investment-value").textContent = money(row.currency, row.valueCents);
  $("investment-base").textContent = !row.isBase && row.hasRate ? `≈ ${money(base, row.baseCents)}` : "";
  $("investment-gain").textContent = [row.hasGain ? `${gain(row.currency, row.gainCents, row.costCents)} on ${money(row.currency, row.costCents)}` : "", ago(row.lastUpdatedAt)].filter(Boolean).join(" · ");
  $("value-form").elements.value.value = (row.valueCents / 100).toFixed(2);

  const history = await api("GET", `/assets/${row.id}/values`);
  renderHistory($("investment-history-card"), $("investment-history"), history, row.currency);
}

async function updateValue(event) {
  event.preventDefault();
  const f = $("value-form");
  formErrorIn(f, "");
  try {
    // A new value is kept in the history too.
    await api("PUT", `/assets/${state.investment.id}`, assetInput(state.investment, { value: f.elements.value.value.trim().replace(",", ".") }));
    haptic("success");
    await showInvestment();
  } catch (err) {
    haptic("error");
    formErrorIn(f, String(err.message ?? err));
  }
}

async function saveInvestment(event) {
  event.preventDefault();
  const f = $("investment-form");
  formErrorIn(f, "");
  const e = f.elements;
  const fields = {
    currency: e.currency.value,
    name: e.name.value.trim(),
    type: e.type.value.trim(),
    cost: e.cost.value.trim().replace(",", "."),
    acquiredAt: e.acquiredAt.value,
    description: e.description.value.trim(),
    ignoreInNetWorth: !e.counted.checked,
  };

  try {
    const row = state.investment;
    if (row) await api("PUT", `/assets/${row.id}`, assetInput(row, fields));
    else await api("POST", "/assets", { kind: "investment", value: e.value.value.trim().replace(",", "."), ...fields });
    haptic("success");
    state.investment = null;
    await show("investments");
  } catch (err) {
    haptic("error");
    formErrorIn(f, String(err.message ?? err));
  }
}

const disarmInvestmentDelete = twoTaps($("investment-delete"), async () => {
  try {
    await api("DELETE", `/assets/${state.investment.id}`);
    haptic("success");
    state.investment = null;
    await show("investments");
  } catch (err) {
    formErrorIn($("investment-form"), String(err.message ?? err));
  }
});

// --- wiring ------------------------------------------------------------------------

for (const tab of document.querySelectorAll(".tabs button")) {
  tab.addEventListener("click", () => {
    // Leaving an edit drops it; Add starts empty.
    if (state.editing) {
      state.editing = null;
      resetForm();
    }
    show(tab.dataset.screen);
  });
}

form.addEventListener("submit", save);
for (const radio of form.elements.kind) radio.addEventListener("change", () => renderCategories(""));
$("month-prev").addEventListener("click", () => {
  state.month = shiftMonth(state.month, -1);
  show("entries");
});
$("month-next").addEventListener("click", () => {
  state.month = shiftMonth(state.month, 1);
  show("entries");
});

tg?.BackButton?.onClick(() => {
  if (parents[state.screen]) {
    show(parents[state.screen]);
    return;
  }

  state.editing = null;
  resetForm();
  show("entries");
});

$("balance-form").addEventListener("submit", saveBalance);
$("account-counted").addEventListener("change", (event) => toggleAccount({ ignoreInSummaries: !event.target.checked }));
$("account-archived").addEventListener("change", (event) => toggleAccount({ archived: event.target.checked }));
$("account-new").addEventListener("click", () => show("new-account"));
$("account-form").addEventListener("submit", addAccount);
for (const row of document.querySelectorAll("#more .nav-row")) {
  row.addEventListener("click", () => {
    if (row.dataset.kind) {
      state.payKind = row.dataset.kind;
      state.payMode = "active";
      show("payments");
      return;
    }

    // Budgets open on the running month.
    state.budgetMonth = "";
    show(row.dataset.screen);
  });
}

$("budgets-prev").addEventListener("click", () => {
  state.budgetMonth = shiftMonth(state.budgetMonth, -1);
  show("budgets");
});
$("budgets-next").addEventListener("click", () => {
  state.budgetMonth = shiftMonth(state.budgetMonth, 1);
  show("budgets");
});
$("budget-new").addEventListener("click", () => {
  state.budget = null;
  show("budget");
});
$("budget-form").addEventListener("submit", saveBudget);
$("investment-new").addEventListener("click", () => {
  state.investment = null;
  show("investment");
});
$("value-form").addEventListener("submit", updateValue);
$("investment-form").addEventListener("submit", saveInvestment);

for (const button of document.querySelectorAll(".filter button")) {
  button.addEventListener("click", () => {
    state.payMode = button.dataset.mode;
    showPayments();
  });
}

$("payment-new").addEventListener("click", () => {
  state.payment = null;
  show("payment");
});
$("payment-form").addEventListener("submit", savePayment);
$("paid-mark").addEventListener("click", markPaid);

$("archived-toggle").addEventListener("click", () => {
  state.showArchived = !state.showArchived;
  showAccounts();
});

function start() {
  resetForm();
  if (!tg?.initData) {
    document.querySelector(".tabs").hidden = true;
    showMessage("Open this from the bot's menu button in Telegram.");
    return;
  }

  tg.ready();
  tg.expand();
  show("home");
}

start();
