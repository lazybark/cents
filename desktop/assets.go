package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

var errAssetNotFound = errors.New("asset not found")

// AssetRow is a property or investment. BaseCents is its value in the base
// currency at today's rate (HasRate is false when there is none); Gain is
// value minus cost, when a cost was given.
type AssetRow struct {
	ID               uint      `json:"id"`
	Kind             string    `json:"kind"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	Currency         string    `json:"currency"`
	IsBase           bool      `json:"isBase"`
	HasRate          bool      `json:"hasRate"`
	ValueCents       int64     `json:"valueCents"`
	BaseCents        int64     `json:"baseCents"`
	CostCents        int64     `json:"costCents"`
	HasGain          bool      `json:"hasGain"`
	GainCents        int64     `json:"gainCents"`
	AcquiredAt       string    `json:"acquiredAt"`
	Description      string    `json:"description"`
	IgnoreInNetWorth bool      `json:"ignoreInNetWorth"`
	LastUpdatedAt    time.Time `json:"lastUpdatedAt"`
}

type AssetTypeTotal struct {
	Type      string `json:"type"`
	Count     int    `json:"count"`
	BaseCents int64  `json:"baseCents"`
}

// AssetsView lists one kind of asset. Totals are in the base currency over
// the assets counted in net worth: GainCents compares the value of those
// with a cost (WithCostCents) to what they cost.
type AssetsView struct {
	BaseCurrency  string           `json:"baseCurrency"`
	Kind          string           `json:"kind"`
	Assets        []AssetRow       `json:"assets"`
	TotalCents    int64            `json:"totalCents"`
	CostCents     int64            `json:"costCents"`
	WithCostCents int64            `json:"withCostCents"`
	GainCents     int64            `json:"gainCents"`
	ByType        []AssetTypeTotal `json:"byType"`
	MissingRates  int              `json:"missingRates"`
	Ignored       int              `json:"ignored"`
	Currencies    []string         `json:"currencies"`
	Types         []string         `json:"types"`
}

// AssetInput is an asset as typed into the form; ID is zero for a new one.
// Kind and currency are only used when creating it.
type AssetInput struct {
	ID               uint   `json:"id"`
	Kind             string `json:"kind"`
	Currency         string `json:"currency"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	Value            string `json:"value"`
	Cost             string `json:"cost"`
	AcquiredAt       string `json:"acquiredAt"`
	Description      string `json:"description"`
	IgnoreInNetWorth bool   `json:"ignoreInNetWorth"`
}

type AssetValueLogInput struct {
	AssetID uint   `json:"assetId"`
	Date    string `json:"date"`
	Value   string `json:"value"`
}

// Assets lists the property ("property") or investments ("investment").
func (a *API) Assets(kind string) (AssetsView, error) {
	parsed, err := asset.ParseKind(kind)
	if err != nil {
		return AssetsView{}, err
	}

	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return AssetsView{}, err
	}

	items, err := storage.LoadAssets()
	if err != nil {
		return AssetsView{}, err
	}

	listed := asset.Filter(items, parsed)
	totals := asset.Total(listed, stts)

	view := AssetsView{
		BaseCurrency:  stts.BaseCurrencyLabel(),
		Kind:          string(parsed),
		Assets:        make([]AssetRow, 0, len(listed)),
		TotalCents:    totals.ValueCents,
		CostCents:     totals.CostCents,
		WithCostCents: totals.WithCostCents,
		GainCents:     totals.WithCostCents - totals.CostCents,
		ByType:        make([]AssetTypeTotal, 0, len(totals.ByType)),
		MissingRates:  totals.MissingRates,
		Ignored:       totals.Ignored,
		Currencies:    stts.CurrencyOptions(),
		Types:         asset.TypeOptions(parsed),
	}

	for _, t := range totals.ByType {
		view.ByType = append(view.ByType, AssetTypeTotal{Type: t.Type, Count: t.Count, BaseCents: t.BaseCents})
	}

	for _, item := range listed {
		view.Assets = append(view.Assets, assetRow(item, stts))
	}

	return view, nil
}

// CreateAsset saves a new asset and starts its value history with today's
// value.
func (a *API) CreateAsset(input AssetInput) (AssetRow, error) {
	kind, err := asset.ParseKind(input.Kind)
	if err != nil {
		return AssetRow{}, err
	}

	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return AssetRow{}, err
	}

	now := time.Now()
	entry, err := asset.New(kind, input.Currency, assetFields(input), dates.ISO, now)
	if err != nil {
		return AssetRow{}, err
	}

	currency, ok := matchOption(stts.CurrencyOptions(), strings.TrimSpace(input.Currency))
	if !ok {
		return AssetRow{}, fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
	}

	entry.Currency = currency

	if err := storage.CreateAsset(&entry); err != nil {
		return AssetRow{}, fmt.Errorf("save failed: %w", err)
	}

	if err := storage.UpsertAssetValueLog(entry.ID, now, entry.ValueCents); err != nil {
		return AssetRow{}, fmt.Errorf("value history save failed: %w", err)
	}

	return assetRow(entry, stts), nil
}

// UpdateAsset changes an asset; its kind and currency stay. A new value is
// also recorded as today's value in its history.
func (a *API) UpdateAsset(input AssetInput) (AssetRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return AssetRow{}, err
	}

	current, err := findAsset(storage, input.ID)
	if err != nil {
		return AssetRow{}, err
	}

	now := time.Now()
	updated, err := current.Edit(assetFields(input), dates.ISO, now)
	if err != nil {
		return AssetRow{}, err
	}

	if err := storage.SaveAsset(&updated); err != nil {
		return AssetRow{}, fmt.Errorf("save failed: %w", err)
	}

	if updated.ValueCents != current.ValueCents {
		if err := storage.UpsertAssetValueLog(updated.ID, now, updated.ValueCents); err != nil {
			return AssetRow{}, fmt.Errorf("value history save failed: %w", err)
		}
	}

	return assetRow(updated, stts), nil
}

func (a *API) DeleteAsset(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findAsset(storage, id); err != nil {
		return err
	}

	return storage.DeleteAsset(id)
}

// AssetValueLogs returns an asset's value history, newest first.
func (a *API) AssetValueLogs(assetID uint) ([]ValueLog, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	logs, err := storage.LoadAssetValueLogs(assetID)
	if err != nil {
		return nil, err
	}

	result := make([]ValueLog, 0, len(logs))
	for _, entry := range logs {
		result = append(result, ValueLog{ID: entry.ID, Date: entry.LogDate.Local().Format(logDateLayout), ValueCents: entry.ValueCents})
	}

	return result, nil
}

// SaveAssetValueLog records the asset's value on a past day (or replaces
// that day's). The current value doesn't change: history entries are only
// snapshots, as for accounts.
func (a *API) SaveAssetValueLog(input AssetValueLogInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	rawDay := strings.TrimSpace(input.Date)
	if rawDay == "" {
		return errors.New("log date is required")
	}

	day, err := time.ParseInLocation(logDateLayout, rawDay, time.Local)
	if err != nil {
		return errors.New("log date must use YYYY-MM-DD format")
	}

	value, err := money.ParseAmountCents(input.Value)
	if err != nil {
		return fmt.Errorf("log value error: %w", err)
	}

	if _, err := findAsset(storage, input.AssetID); err != nil {
		return err
	}

	return storage.UpsertAssetValueLog(input.AssetID, day, value)
}

func (a *API) DeleteAssetValueLog(assetID uint, logID uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findAsset(storage, assetID); err != nil {
		return err
	}

	return storage.DeleteAssetValueLog(assetID, logID)
}

func assetFields(input AssetInput) asset.Fields {
	return asset.Fields{
		Name:             input.Name,
		Type:             input.Type,
		Value:            input.Value,
		Cost:             input.Cost,
		AcquiredAt:       input.AcquiredAt,
		Description:      input.Description,
		IgnoreInNetWorth: input.IgnoreInNetWorth,
	}
}

func findAsset(storage StorageWorker, id uint) (asset.Asset, error) {
	items, err := storage.LoadAssets()
	if err != nil {
		return asset.Asset{}, err
	}

	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return asset.Asset{}, errAssetNotFound
}

func assetRow(item asset.Asset, stts settings.AppSettings) AssetRow {
	baseCents, hasRate := stts.ConvertToBaseCents(item.Currency, item.ValueCents)
	gain, hasGain := item.GainCents()

	return AssetRow{
		ID:               item.ID,
		Kind:             item.Kind,
		Name:             item.Name,
		Type:             item.Type,
		Currency:         item.Currency,
		IsBase:           stts.IsBase(item.Currency),
		HasRate:          hasRate,
		ValueCents:       item.ValueCents,
		BaseCents:        baseCents,
		CostCents:        item.CostCents,
		HasGain:          hasGain,
		GainCents:        gain,
		AcquiredAt:       formatOptionalDay(item.AcquiredAt),
		Description:      item.Description,
		IgnoreInNetWorth: item.IgnoreInNetWorth,
		LastUpdatedAt:    item.LastUpdatedAt,
	}
}
