// Connecting to a cents server, from the setup screen or Settings. A server
// whose certificate this computer doesn't trust shows its fingerprint first,
// to compare with the server's and trust.
import { api } from "./api.js";
import { busy, formError } from "./ui.js";

const $ = (id) => document.getElementById(id);

const el = {
  dialog: $("server-dialog"),
  form: $("server-form"),
  plain: $("server-plain"),
  untrusted: $("server-untrusted"),
  fingerprint: $("server-fingerprint"),
  submit: $("server-submit"),
};

const state = {
  // certificate is the fingerprint shown, trusted on the next submit.
  certificate: "",
  connected: null,
};

el.form.addEventListener("submit", connect);
el.form.elements.url.addEventListener("input", () => {
  untrust();
  showPlain();
});
el.form.elements.token.addEventListener("input", () => formError(el.form, ""));

// openServerDialog asks for a server and connects to it; connected(url) runs
// once the app uses it.
export function openServerDialog(connected) {
  state.connected = connected;
  el.form.reset();
  formError(el.form, "");
  untrust();
  showPlain();
  el.dialog.showModal();
  el.form.elements.url.focus();
}

async function connect(event) {
  event.preventDefault();
  formError(el.form, "");
  const f = el.form.elements;

  try {
    const result = await busy(el.form, () => api.ConnectServer({ url: f.url.value, token: f.token.value, certificate: state.certificate }));
    if (result.untrusted) {
      state.certificate = result.certificate;
      el.fingerprint.textContent = result.certificate;
      el.untrusted.hidden = false;
      el.submit.textContent = "Trust it and connect";
      return;
    }

    el.dialog.close();
    state.connected?.(f.url.value.trim());
  } catch (err) {
    formError(el.form, String(err));
  }
}

// untrust forgets a fingerprint shown for another address.
function untrust() {
  state.certificate = "";
  el.untrusted.hidden = true;
  el.submit.textContent = "Connect";
}

// showPlain warns about plain HTTP to another machine.
function showPlain() {
  let link = null;
  try {
    link = new URL(el.form.elements.url.value.trim());
  } catch {
    // Not an address yet.
  }

  const local = link && ["localhost", "127.0.0.1", "[::1]"].includes(link.hostname);
  el.plain.hidden = !(link?.protocol === "http:" && !local);
}
