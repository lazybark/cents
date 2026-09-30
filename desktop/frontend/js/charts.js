// Small SVG charts for month-by-month figures: bars and lines over a row of
// months, with a value axis and a tooltip per month. No library, so the app
// works offline. Months with a note (see notes.js) get a marker and show it,
// and clicking a month adds or edits its note.
import { editNote, noteFor } from "./notes.js";

const SVG = "http://www.w3.org/2000/svg";
const WIDTH = 720;
const HEIGHT = 260;
const MARGIN = { top: 14, right: 14, bottom: 30, left: 70 };

function svg(name, attrs = {}, text) {
  const node = document.createElementNS(SVG, name);
  for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, String(value));
  if (text !== undefined) node.textContent = text;
  return node;
}

// monthRange lists every "YYYY-MM" from first to last, inclusive.
export function monthRange(first, last) {
  const months = [];
  let [year, month] = first.split("-").map(Number);
  const [lastYear, lastMonth] = last.split("-").map(Number);

  while (year < lastYear || (year === lastYear && month <= lastMonth)) {
    months.push(`${year}-${String(month).padStart(2, "0")}`);
    month += 1;
    if (month > 12) {
      month = 1;
      year += 1;
    }
  }

  return months;
}

// monthLabel reads like "Jan 26"; long like "January 2026".
export function monthLabel(month, long = false) {
  const [year, index] = month.split("-").map(Number);
  const date = new Date(year, index - 1, 1);

  return long
    ? date.toLocaleString(undefined, { month: "long", year: "numeric" })
    : `${date.toLocaleString(undefined, { month: "short" })} ${String(year).slice(2)}`;
}

// compact shortens cents for an axis: "12.5k", "-3M", "250".
export function compact(cents) {
  const value = cents / 100;
  const abs = Math.abs(value);
  const short = (n, suffix) => `${Number(n.toFixed(1))}${suffix}`;

  if (abs >= 1e6) return short(value / 1e6, "M");
  if (abs >= 1e3) return short(value / 1e3, "k");
  // Small values keep their cents, so close ticks don't read the same.
  if (abs > 0 && abs < 10) return String(Number(value.toFixed(2)));
  return String(Math.round(value));
}

// niceTicks spreads about count round values over min..max.
function niceTicks(min, max, count = 5) {
  if (min === max) {
    const pad = Math.abs(min) * 0.1 || 100;
    min -= pad;
    max += pad;
  }

  const raw = (max - min) / count;
  const magnitude = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= raw);
  const start = Math.floor(min / step) * step;
  const ticks = [];

  for (let value = start; value <= max + step * 0.5; value += step) ticks.push(Math.round(value));

  return ticks;
}

// chart draws months along the bottom and series over them into container.
//   months:  ["YYYY-MM", …]
//   series:  [{ type: "bar" | "line", values: [cents | null], className }]
//            Bars get "positive"/"negative" classes by sign unless they have
//            a className.
//   zero:    whether the value axis always includes 0 (true for bars).
//   stacked: bars stack on each other in one column instead of sitting
//            side by side (for parts of a whole, all positive).
//   axis:    (cents) => label on the value axis.
//   tooltip: (index) => lines of text for that month.
//   label:   what the chart shows, for screen readers.
//   notes:   whether months show and take notes (true by default).
export function chart(container, { months, series, zero = true, stacked = false, axis = compact, tooltip, label, notes = true }) {
  container.replaceChildren();
  container.classList.add("chart");

  const present = (v) => v !== null && v !== undefined;
  const values = series.flatMap((s) => s.values).filter(present);
  if (stacked) {
    // The scale has to fit each column's total.
    const bars = series.filter((s) => s.type === "bar");
    months.forEach((_, i) => values.push(bars.reduce((sum, s) => sum + (s.values[i] ?? 0), 0)));
  }
  if (months.length === 0 || values.length === 0) return;

  let min = Math.min(...values);
  let max = Math.max(...values);
  if (zero) {
    min = Math.min(0, min);
    max = Math.max(0, max);
  } else {
    const pad = (max - min) * 0.08;
    min -= pad;
    max += pad;
  }

  const ticks = niceTicks(min, max);
  const low = Math.min(ticks[0], min);
  const high = Math.max(ticks[ticks.length - 1], max);
  const plotWidth = WIDTH - MARGIN.left - MARGIN.right;
  const plotHeight = HEIGHT - MARGIN.top - MARGIN.bottom;
  const band = plotWidth / months.length;
  const y = (value) => MARGIN.top + plotHeight - ((value - low) / (high - low || 1)) * plotHeight;
  const x = (index) => MARGIN.left + band * index + band / 2;

  const root = svg("svg", { viewBox: `0 0 ${WIDTH} ${HEIGHT}`, role: "img", "aria-label": label ?? "chart" });

  // Value axis and grid.
  const grid = svg("g", { class: "chart-grid" });
  for (const tick of ticks) {
    if (tick < low || tick > high) continue;
    grid.append(
      svg("line", { x1: MARGIN.left, x2: WIDTH - MARGIN.right, y1: y(tick), y2: y(tick), class: tick === 0 ? "chart-zero" : "" }),
      svg("text", { x: MARGIN.left - 8, y: y(tick) + 4, "text-anchor": "end" }, axis(tick)),
    );
  }
  root.append(grid);

  // Month labels: spread out so they don't collide, always the last one.
  const every = Math.ceil(months.length / 9);
  const labels = svg("g", { class: "chart-axis" });
  const last = months.length - 1;
  months.forEach((month, i) => {
    const regular = i % every === 0;
    // The last month gets a label too, unless it would crowd the one before.
    const lastFits = i === last && i % every >= Math.max(2, every * 0.75);
    if (!regular && !lastFits) return;
    labels.append(svg("text", { x: x(i), y: HEIGHT - 10, "text-anchor": "middle" }, monthLabel(month)));
  });
  root.append(labels);

  // Bars first, lines on top of them.
  const bars = series.filter((s) => s.type === "bar");
  const columns = stacked ? 1 : Math.max(bars.length, 1);
  const barWidth = Math.min(28, (band * 0.7) / columns);
  const stackedTo = months.map(() => 0);
  bars.forEach((s, n) => {
    const group = svg("g");
    s.values.forEach((value, i) => {
      if (!present(value)) return;
      const base = stacked ? stackedTo[i] : 0;
      const top = y(Math.max(base + value, base));
      const height = Math.max(Math.abs(y(base + value) - y(base)), value === 0 ? 0 : 1);
      const offset = stacked ? 0 : (n - (bars.length - 1) / 2) * barWidth;
      if (stacked) stackedTo[i] += value;
      group.append(svg("rect", { x: x(i) - barWidth / 2 + offset, y: top, width: barWidth, height, class: s.className ?? (value < 0 ? "bar-negative" : "bar-positive") }));
    });
    root.append(group);
  });

  for (const s of series.filter((s) => s.type === "line")) {
    const points = s.values.map((v, i) => [i, v]).filter(([, v]) => v !== null && v !== undefined);
    if (points.length > 1) {
      root.append(svg("polyline", { points: points.map(([i, v]) => `${x(i)},${y(v)}`).join(" "), class: `chart-line ${s.className ?? ""}` }));
    }
    const dots = svg("g", { class: `chart-dots ${s.className ?? ""}` });
    for (const [i, v] of points) dots.append(svg("circle", { cx: x(i), cy: y(v), r: months.length > 40 ? 2 : 3 }));
    root.append(dots);
  }

  // A marker over each month with a note.
  if (notes) {
    const markers = svg("g");
    months.forEach((month, i) => {
      if (noteFor(month)) markers.append(svg("path", { d: `M ${x(i) - 4} ${MARGIN.top - 10} h 8 l -4 6 z`, class: "chart-note" }));
    });
    root.append(markers);
  }

  // One hover column per month, showing its tooltip.
  const tip = document.createElement("div");
  tip.className = "chart-tip";
  tip.hidden = true;

  const hover = svg("g", { class: notes ? "chart-hover clickable" : "chart-hover" });
  months.forEach((month, i) => {
    const column = svg("rect", { x: MARGIN.left + band * i, y: MARGIN.top, width: band, height: plotHeight });
    if (notes) column.addEventListener("click", () => editNote(month));
    column.addEventListener("mouseenter", () => {
      column.classList.add("active");
      const lines = (tooltip?.(i) ?? [monthLabel(month, true)]).map((line, n) => {
        const div = document.createElement("div");
        if (n === 0) div.className = "chart-tip-title";
        div.textContent = line;
        return div;
      });

      if (notes) {
        const note = noteFor(month);
        if (note) {
          const div = document.createElement("div");
          div.className = "chart-tip-note";
          div.textContent = note;
          lines.push(div);
        }

        const hint = document.createElement("div");
        hint.className = "chart-tip-hint";
        hint.textContent = note ? "Click to edit the note" : "Click to add a note";
        lines.push(hint);
      }

      tip.replaceChildren(...lines);
      tip.hidden = false;
      // Keep the tip inside the chart: left of the column on the right half.
      const share = (MARGIN.left + band * (i + 0.5)) / WIDTH;
      tip.style.left = share < 0.6 ? `calc(${share * 100}% + 12px)` : "";
      tip.style.right = share < 0.6 ? "" : `calc(${(1 - share) * 100}% + 12px)`;
    });
    column.addEventListener("mouseleave", () => {
      column.classList.remove("active");
      tip.hidden = true;
    });
    hover.append(column);
  });
  root.append(hover);

  container.append(root, tip);
}

// legend shows what each series' color means.
export function legend(container, items) {
  container.replaceChildren(
    ...items.map(([className, text]) => {
      const item = document.createElement("span");
      item.className = "chart-legend-item";
      const swatch = document.createElement("span");
      swatch.className = `chart-swatch ${className}`;
      item.append(swatch, text);
      return item;
    }),
  );
}
