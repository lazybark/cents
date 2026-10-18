package bot

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lazybark/cents/money"
)

// entry is an income or expense as typed in a message:
//
//	[+]amount [currency] [category] [comment] [#tag] [@account] [date]
//
// "+" makes it an income. The currency (a name or code, also stuck to the
// amount: "12€", "$12") defaults to the base one. The category is the
// first words naming one, or the start of a name ("eat" for "Eating
// out"); without one, the bot asks. The date is today unless "yesterday",
// a YYYY-MM-DD or a DD.MM is given.
type entry struct {
	IsIncome bool
	Amount   string
	Currency string
	Category string
	Account  string
	Date     string
	Comment  string
	Tags     []string
}

// choices are what an entry can name, from the backend.
type choices struct {
	// Base is the base currency, and Currencies every currency by every way
	// to write it (name and code, lower case).
	Base       string
	Currencies map[string]string
	Income     []string
	Expense    []string
	Accounts   []string
}

func (c choices) categories(income bool) []string {
	if income {
		return c.Income
	}

	return c.Expense
}

var (
	amountPattern = regexp.MustCompile(`^([+-]?)([^\d+-]*)(\d+(?:[.,]\d{1,2})?)(\D*)$`)
	isoDate       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	dayMonth      = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.?$`)
)

// errNotEntry is a message that doesn't start with an amount.
var errNotEntry = errors.New("not an entry")

// parseEntry reads text as an entry; today is the day it's made on.
func parseEntry(text string, c choices, today time.Time) (entry, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return entry{}, errNotEntry
	}

	m := amountPattern.FindStringSubmatch(words[0])
	if m == nil {
		return entry{}, errNotEntry
	}

	e := entry{IsIncome: m[1] == "+", Amount: strings.Replace(m[3], ",", ".", 1)}
	if cents, err := money.ParseAmountCents(e.Amount); err != nil || cents == 0 {
		return entry{}, errors.New("the amount must be a number above zero")
	}

	// A currency stuck to the amount: "$12", "12€", "12usd".
	for _, stuck := range []string{m[2], m[4]} {
		if stuck == "" {
			continue
		}

		name, ok := c.Currencies[strings.ToLower(stuck)]
		if !ok || e.Currency != "" {
			return entry{}, fmt.Errorf("unknown currency %q", stuck)
		}

		e.Currency = name
	}

	var rest []string
	for i, word := range words[1:] {
		lower := strings.ToLower(word)
		switch {
		case i == 0 && e.Currency == "" && c.Currencies[lower] != "":
			e.Currency = c.Currencies[lower]
		case strings.HasPrefix(word, "#") && len(word) > 1:
			e.Tags = append(e.Tags, strings.TrimPrefix(word, "#"))
		case strings.HasPrefix(word, "@") && len(word) > 1:
			account, err := pick(c.Accounts, strings.TrimPrefix(word, "@"), "account")
			if err != nil {
				return entry{}, err
			}

			e.Account = account
		case e.Date == "" && dateWord(word, today) != "":
			e.Date = dateWord(word, today)
		default:
			rest = append(rest, word)
		}
	}

	if e.Date == "" {
		e.Date = today.Format("2006-01-02")
	}

	if e.Currency == "" {
		e.Currency = c.Base
	}

	e.Category, rest = category(c.categories(e.IsIncome), rest)
	e.Comment = strings.Join(rest, " ")

	return e, nil
}

// dateWord is the day word names, as YYYY-MM-DD, or "" for no date.
func dateWord(word string, today time.Time) string {
	switch {
	case strings.EqualFold(word, "today"):
		return today.Format("2006-01-02")
	case strings.EqualFold(word, "yesterday"):
		return today.AddDate(0, 0, -1).Format("2006-01-02")
	case isoDate.MatchString(word):
		if _, err := time.Parse("2006-01-02", word); err == nil {
			return word
		}
	}

	// DD.MM: this year, or last year's if that's still to come.
	if m := dayMonth.FindStringSubmatch(word); m != nil {
		day, err := time.Parse("2.1.2006", fmt.Sprintf("%s.%s.%d", m[1], m[2], today.Year()))
		if err != nil {
			return ""
		}

		if day.After(today) {
			day = day.AddDate(-1, 0, 0)
		}

		return day.Format("2006-01-02")
	}

	return ""
}

// category finds the category the words start with: the most words naming
// one exactly, or else the first word starting just one name. It returns
// it and the words after it; "" and all the words when none matches.
func category(names []string, words []string) (string, []string) {
	for n := len(words); n > 0; n-- {
		typed := strings.Join(words[:n], " ")
		for _, name := range names {
			if strings.EqualFold(name, typed) {
				return name, words[n:]
			}
		}
	}

	if len(words) > 0 && len([]rune(words[0])) >= 2 {
		if name, err := pick(names, words[0], ""); err == nil {
			return name, words[1:]
		}
	}

	return "", words
}

// pick is the one of names that typed names or starts (ignoring case).
func pick(names []string, typed, what string) (string, error) {
	var found []string
	for _, name := range names {
		if strings.EqualFold(name, typed) {
			return name, nil
		}

		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(typed)) {
			found = append(found, name)
		}
	}

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("no %s starts with %q", what, typed)
	default:
		return "", fmt.Errorf("%q could be the %s %s", typed, what, strings.Join(found, " or "))
	}
}
