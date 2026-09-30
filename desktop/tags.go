package desktop

import (
	"fmt"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/tag"
)

// TagInput adds a tag (ID 0), or renames or archives one.
type TagInput struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

func (a *API) SaveTag(input TagInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	entry := tag.Tag{ID: input.ID, Name: input.Name, Archived: input.Archived}
	if err := storage.SaveTag(&entry); err != nil {
		return fmt.Errorf("tag save failed: %w", err)
	}

	return nil
}

// TagSummary is what a tag's entries add up to, in the base currency.
type TagSummary struct {
	ID           uint          `json:"id"`
	Name         string        `json:"name"`
	Archived     bool          `json:"archived"`
	Count        int           `json:"count"`
	IncomeCents  int64         `json:"incomeCents"`
	ExpenseCents int64         `json:"expenseCents"`
	NetCents     int64         `json:"netCents"`
	MissingRates int           `json:"missingRates"`
	First        string        `json:"first"`
	Last         string        `json:"last"`
	Categories   []TagCategory `json:"categories"`
}

type TagCategory struct {
	Category string `json:"category"`
	IsIncome bool   `json:"isIncome"`
	Cents    int64  `json:"cents"`
}

// TagsView is Incomes & expenses → Tags: every tag in use, most recently
// used first, and how many tags there are without entries.
type TagsView struct {
	BaseCurrency string       `json:"baseCurrency"`
	Tags         []TagSummary `json:"tags"`
	Unused       int          `json:"unused"`
}

func (a *API) CashflowTags() (TagsView, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return TagsView{}, err
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return TagsView{}, fmt.Errorf("failed to load cashflows: %w", err)
	}

	tags, err := storage.LoadTags()
	if err != nil {
		return TagsView{}, err
	}

	byName := map[string]tag.Tag{}
	for _, t := range tags {
		byName[usageKey(t.Name)] = t
	}

	view := TagsView{BaseCurrency: stts.BaseCurrencyLabel(), Tags: make([]TagSummary, 0)}
	for _, t := range cashflow.ByTag(entries) {
		known := byName[usageKey(t.Tag)]
		summary := TagSummary{
			ID: known.ID, Name: t.Tag, Archived: known.Archived, Count: t.Count,
			IncomeCents: t.IncomeCents, ExpenseCents: t.ExpenseCents, NetCents: t.IncomeCents - t.ExpenseCents, MissingRates: t.MissingRates,
			First: formatDay(t.First), Last: formatDay(t.Last), Categories: make([]TagCategory, 0, len(t.Categories)),
		}

		for _, c := range t.Categories {
			summary.Categories = append(summary.Categories, TagCategory(c))
		}

		view.Tags = append(view.Tags, summary)
	}

	view.Unused = len(tags) - len(view.Tags)
	return view, nil
}
