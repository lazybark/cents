// Obligations: serious regular payments, like rent, insurance and bills.
import { regularView } from "./regular.js";

const view = regularView({
  kind: "obligation",
  view: "obligations",
  addButton: "add-obligation",
  title: "Obligations",
  plural: "obligation(s)",
  addTitle: "Add obligation",
  namePlaceholder: "Rent, car insurance, electricity…",
  empty: "No obligations found.",
});

export const title = view.config.title;
export const init = view.init;
export const show = view.show;
