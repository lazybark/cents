import { THEMES, savedTheme, applyTheme, setTheme } from "./js/theme.js";

const $ = (id) => document.getElementById(id);
const api = window.go?.adminapp?.API;
let connected = false, busy = false, connecting = false, generation = 0;
let entries = [], nextBefore = 0, activeQuery = {}, pendingConnection;

function message(id, value = "") {
  $(id).textContent = String(value);
  $(id).hidden = !value;
}
function errorText(error) { return error?.message || String(error); }
function controls() {
  $("refresh").disabled = !connected || busy || connecting;
  $("older").disabled = busy || connecting;
  $("auto").disabled = !connected || connecting;
  $("connect").disabled = $("retry").disabled = $("disconnect").disabled = connecting;
  document.querySelector(".log-panel").setAttribute("aria-busy", String(busy));
}
function connectionPanel(show) {
  $("connect-panel").hidden = !show;
  $("cancel-connect").hidden = !connected;
  if (show) $("address").focus();
}
async function showConnection() {
  const status = await api.Status();
  connected = status.connected;
  $("address").value = status.url || "";
  $("retry").hidden = !status.remembered || connected;
  $("disconnect").hidden = !status.url;
  $("log-view").hidden = !connected;
  $("connection").textContent = connected ? "Connection…" : "Connect to server…";
  $("server").textContent = connected ? status.url : "Connect to your cents server to inspect its logs.";
  connectionPanel(!connected);
  controls();
}
async function runConnection(input, retry = false) {
  if (connecting) return;
  connecting = true;
  // Ignore results from an earlier connection or an outstanding refresh.
  generation++;
  busy = false;
  controls();
  message("connect-error");
  try {
    const result = retry ? (await api.Reconnect(), { connected: true }) : await api.Connect(input);
    if (result.untrusted) {
      pendingConnection = { ...input, certificate: result.certificate };
      $("fingerprint").textContent = result.certificate;
      $("certificate").showModal();
      return;
    }
    $("token").value = "";
    entries = [];
    activeQuery = {};
    $("filters").reset();
    await showConnection();
    await load();
  } catch (error) {
    message("connect-error", errorText(error));
  } finally {
    connecting = false;
    controls();
  }
}

function queryFromForm() {
  const fields = new FormData($("filters"));
  const query = { level: fields.get("level"), component: fields.get("component"), client: fields.get("client"), status: fields.get("status").trim().toLowerCase(), search: fields.get("search").trim() };
  if (query.status && !/^[1-5](?:[0-9]{2}|xx)$/.test(query.status)) throw new Error("HTTP status must be a code from 100 to 599 or a group such as 4xx.");
  for (const key of ["from", "to"]) {
    const raw = fields.get(key);
    query[key] = raw ? new Date(raw).toISOString() : "";
  }
  if (query.from && query.to && query.to < query.from) throw new Error("The end time must not precede the start time.");
  return query;
}

function requestMetadata(entry) {
  // Fall back to stored attributes when connected to an older server.
  let attributes = {};
  try { attributes = JSON.parse(entry.attributes) || {}; } catch {}
  const status = entry.status ?? attributes.status;
  return {
    status: Number.isInteger(status) && status >= 100 && status <= 599 ? status : null,
    client: entry.client || (typeof attributes.client === "string" ? attributes.client : "") || (typeof attributes.route === "string" ? attributes.route : ""),
  };
}
function renderEntries() {
  const fragment = document.createDocumentFragment();
  for (const entry of entries) {
    const row = document.createElement("tr");
    const time = new Date(entry.timestamp);
    const request = requestMetadata(entry);
    for (const value of [time.toLocaleString(), entry.level, entry.component, request.client || "—", request.status ?? "—"]) {
      const cell = document.createElement("td");
      cell.textContent = value;
      cell.title = value;
      row.append(cell);
    }
    row.cells[0].title = entry.timestamp;
    row.cells[1].className = `level-${entry.level.toLowerCase()}`;
    row.cells[4].className = request.status >= 500 ? "status-error" : request.status >= 400 ? "status-warning" : request.status >= 200 && request.status < 300 ? "status-success" : "";
    const cell = document.createElement("td");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "event-button";
    button.textContent = entry.message || "(empty message)";
    button.title = "Inspect event: " + entry.message;
    button.addEventListener("click", () => details(entry));
    cell.append(button);
    row.append(cell);
    fragment.append(row);
  }
  $("entries").replaceChildren(fragment);
}
function details(entry) {
  const fragment = document.createDocumentFragment();
  const request = requestMetadata(entry);
  for (const [label, value] of [["Event ID", entry.id], ["Time (UTC)", entry.timestamp], ["Level", entry.level], ["Component", entry.component], ["Client", request.client || "—"], ["HTTP status", request.status ?? "—"]]) {
    const dt = document.createElement("dt"), dd = document.createElement("dd");
    dt.textContent = label;
    dd.textContent = value;
    fragment.append(dt, dd);
  }
  $("event-meta").replaceChildren(fragment);
  $("event-message").textContent = entry.message;
  try { $("event-attributes").textContent = JSON.stringify(JSON.parse(entry.attributes), null, 2); }
  catch { $("event-attributes").textContent = entry.attributes; }
  $("details").showModal();
}
async function load(older = false) {
  if (!connected || (busy && older)) return;
  const current = ++generation;
  busy = true;
  controls();
  message("log-error");
  $("log-status").textContent = older ? "Loading older events…" : "Loading logs…";
  try {
    const page = await api.ReadLogs({ ...activeQuery, limit: 100, beforeId: older ? nextBefore : 0 });
    if (current !== generation) return;
    entries = older ? entries.concat(page.entries) : page.entries;
    nextBefore = page.nextBefore;
    $("older").hidden = !page.hasMore;
    const selected = $("component").value;
    const components = [...new Set([...page.components, ...(selected ? [selected] : [])])].sort();
    $("component").replaceChildren(new Option("All components", ""), ...components.map((c) => new Option(c, c)));
    $("component").value = selected;
    const selectedClient = $("client").value;
    const clients = [...new Set([...(page.clients || []), ...entries.map((e) => requestMetadata(e).client).filter(Boolean), ...(selectedClient ? [selectedClient] : [])])].sort();
    $("client").replaceChildren(new Option("All clients", ""), ...clients.map((c) => new Option(c, c)));
    $("client").value = selectedClient;
    renderEntries();
    $("log-status").textContent = entries.length ? `${entries.length} events · newest first · updated ${new Date().toLocaleTimeString()}` : "No logs match these filters.";
  } catch (error) {
    if (current !== generation) return;
    message("log-error", errorText(error));
    $("log-status").textContent = entries.length ? `Showing ${entries.length} previously loaded events.` : "Logs could not be loaded.";
  } finally {
    if (current === generation) { busy = false; controls(); }
  }
}

$("theme").replaceChildren(...THEMES.map((t) => new Option(t.name, t.id)));
$("theme").value = savedTheme();
applyTheme(savedTheme());
$("theme").addEventListener("change", () => setTheme($("theme").value));
$("connection").addEventListener("click", () => connectionPanel(true));
$("cancel-connect").addEventListener("click", () => connectionPanel(false));
$("connect-form").addEventListener("submit", (event) => {
  event.preventDefault();
  runConnection({ url: $("address").value, token: $("token").value, remember: $("remember").checked, certificate: "" });
});
$("retry").addEventListener("click", () => runConnection(null, true));
$("disconnect").addEventListener("click", async () => {
  if (connecting) return;
  generation++;
  try {
    await api.Disconnect();
    busy = false;
    $("auto").checked = false;
    entries = [];
    renderEntries();
    $("token").value = "";
    $("details").close();
    await showConnection();
  } catch (error) { message("connect-error", errorText(error)); busy = false; controls(); }
});
$("reject-certificate").addEventListener("click", () => { pendingConnection = null; $("certificate").close(); });
$("certificate").addEventListener("cancel", () => { pendingConnection = null; });
$("trust-certificate").addEventListener("click", () => {
  const input = pendingConnection;
  pendingConnection = null;
  $("certificate").close();
  if (input) runConnection(input);
});
$("filters").addEventListener("submit", (event) => {
  event.preventDefault();
  try { activeQuery = queryFromForm(); load(); }
  catch (error) { message("log-error", errorText(error)); }
});
$("clear").addEventListener("click", () => { $("filters").reset(); activeQuery = {}; load(); });
$("refresh").addEventListener("click", () => load());
$("older").addEventListener("click", () => { $("auto").checked = false; load(true); });
setInterval(() => { if ($("auto").checked && !busy && !connecting && $("connect-panel").hidden && !document.hidden && !$("details").open) load(); }, 5000);

try {
  if (!api) throw new Error("Open cents-admin as a desktop application to connect to a server.");
  await showConnection();
  const status = await api.Status();
  if (status.remembered) await runConnection(null, true);
} catch (error) { message("connect-error", errorText(error)); }
