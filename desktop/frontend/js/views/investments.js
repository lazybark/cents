// Investments: stocks, bonds, funds and the like.
import { assetsView } from "./assets.js";

const view = assetsView({
  kind: "investment",
  view: "investments",
  addButton: "add-investment",
  title: "Investments",
  countNoun: "investment(s)",
  addTitle: "Add investment",
  namePlaceholder: "S&P 500 ETF",
  costLabel: "Amount invested",
  costWord: "an invested amount",
  costHeading: "Invested",
  acquiredLabel: "Invested on",
  acquiredHeading: "Since",
  empty: "No investments yet.",
});

export const title = view.config.title;
export const init = view.init;
export const show = view.show;
