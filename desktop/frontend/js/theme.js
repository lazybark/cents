// Themes: sets of the colors in style.css, picked in Settings and kept in
// this browser's storage (a preference of the machine, not the database).
// "system" follows the computer's light or dark mode. index.html applies
// the saved theme before anything draws; this module changes it.

export const STORAGE_KEY = "cents.theme";

// Swatches show each theme's background, panel and accent colors.
export const THEMES = [
  { id: "system", name: "System", note: "Paper in light mode, Amber in dark", swatch: ["#F6F1E7", "#1F1A17", "#E7C96D"] },
  { id: "amber", name: "Amber", note: "Warm brown and gold, like the TUI", swatch: ["#1F1A17", "#26201C", "#E7C96D"] },
  { id: "ocean", name: "Ocean", note: "Navy and sky blue", swatch: ["#121A24", "#172231", "#6CB6FF"] },
  { id: "forest", name: "Forest", note: "Deep green and sand", swatch: ["#141B16", "#19221B", "#E8B86B"] },
  { id: "plum", name: "Plum", note: "Purple and lavender", swatch: ["#1C1621", "#231B29", "#C99BF0"] },
  { id: "graphite", name: "Graphite", note: "Neutral grey and teal", swatch: ["#18191B", "#1F2023", "#4FD1C5"] },
  { id: "paper", name: "Paper", note: "Light", swatch: ["#F6F1E7", "#FFFCF6", "#8C5E0D"] },
];

const DEFAULT = "amber";
const dark = window.matchMedia?.("(prefers-color-scheme: dark)");

export function savedTheme() {
  try {
    const id = localStorage.getItem(STORAGE_KEY);
    return THEMES.some((t) => t.id === id) ? id : DEFAULT;
  } catch {
    return DEFAULT;
  }
}

// applyTheme shows theme (an id from THEMES) now.
export function applyTheme(id) {
  const shown = id === "system" ? (dark?.matches === false ? "paper" : "amber") : id;
  if (shown === DEFAULT) document.documentElement.removeAttribute("data-theme");
  else document.documentElement.dataset.theme = shown;
}

// setTheme shows theme and remembers it.
export function setTheme(id) {
  applyTheme(id);
  try {
    localStorage.setItem(STORAGE_KEY, id);
  } catch {
    // Not remembered; it still shows until the app closes.
  }
}

// With "system", follow the computer when it switches.
dark?.addEventListener?.("change", () => {
  if (savedTheme() === "system") applyTheme("system");
});
