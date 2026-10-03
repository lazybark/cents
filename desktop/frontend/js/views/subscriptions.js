// Subscriptions: minor regular payments, like streaming, apps and software.
import { regularView } from "./regular.js";

const view = regularView({
  kind: "subscription",
  view: "subscriptions",
  addButton: "add-subscription",
  title: "Subscriptions",
  plural: "subscription(s)",
  addTitle: "Add subscription",
  namePlaceholder: "Netflix, Spotify, GitHub…",
  empty: "No subscriptions found.",
});

export const title = view.config.title;
export const init = view.init;
export const show = view.show;
