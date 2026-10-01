// Property: cars, apartments, land and the like.
import { assetsView } from "./assets.js";

const view = assetsView({
  kind: "property",
  view: "property",
  addButton: "add-property",
  title: "Property",
  countNoun: "item(s) of property",
  addTitle: "Add property",
  namePlaceholder: "Apartment in Lisbon",
  costLabel: "Purchase price",
  costWord: "a purchase price",
  costHeading: "Paid",
  acquiredLabel: "Bought on",
  acquiredHeading: "Bought",
  empty: "No property yet.",
});

export const title = view.config.title;
export const init = view.init;
export const show = view.show;
