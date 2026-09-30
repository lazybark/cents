// Package currency is the list of currencies the app knows: ISO 4217
// currencies plus precious metals and a few cryptocurrencies, with names
// and symbols. It's built in, so picking currencies works offline; rates
// for them come from the rates package.
package currency

import "strings"

type Currency struct {
	Code   string
	Name   string
	Symbol string
	// Crypto marks cryptocurrencies and metals, which only the main rate
	// source has.
	Crypto bool
}

var all = []Currency{
	{Code: "AED", Name: "UAE Dirham", Symbol: "د.إ"},
	{Code: "AFN", Name: "Afghan Afghani", Symbol: "؋"},
	{Code: "ALL", Name: "Albanian Lek", Symbol: "L"},
	{Code: "AMD", Name: "Armenian Dram", Symbol: "֏"},
	{Code: "ANG", Name: "Netherlands Antillean Guilder", Symbol: "ƒ"},
	{Code: "AOA", Name: "Angolan Kwanza", Symbol: "Kz"},
	{Code: "ARS", Name: "Argentine Peso", Symbol: "$"},
	{Code: "AUD", Name: "Australian Dollar", Symbol: "A$"},
	{Code: "AWG", Name: "Aruban Florin", Symbol: "ƒ"},
	{Code: "AZN", Name: "Azerbaijani Manat", Symbol: "₼"},
	{Code: "BAM", Name: "Bosnia-Herzegovina Convertible Mark", Symbol: "KM"},
	{Code: "BBD", Name: "Barbadian Dollar", Symbol: "$"},
	{Code: "BDT", Name: "Bangladeshi Taka", Symbol: "৳"},
	{Code: "BGN", Name: "Bulgarian Lev", Symbol: "лв"},
	{Code: "BHD", Name: "Bahraini Dinar", Symbol: ".د.ب"},
	{Code: "BIF", Name: "Burundian Franc", Symbol: "FBu"},
	{Code: "BMD", Name: "Bermudian Dollar", Symbol: "$"},
	{Code: "BND", Name: "Brunei Dollar", Symbol: "$"},
	{Code: "BOB", Name: "Bolivian Boliviano", Symbol: "Bs"},
	{Code: "BRL", Name: "Brazilian Real", Symbol: "R$"},
	{Code: "BSD", Name: "Bahamian Dollar", Symbol: "$"},
	{Code: "BTN", Name: "Bhutanese Ngultrum", Symbol: "Nu"},
	{Code: "BWP", Name: "Botswana Pula", Symbol: "P"},
	{Code: "BYN", Name: "Belarusian Ruble", Symbol: "Br"},
	{Code: "BZD", Name: "Belize Dollar", Symbol: "$"},
	{Code: "CAD", Name: "Canadian Dollar", Symbol: "C$"},
	{Code: "CDF", Name: "Congolese Franc", Symbol: "FC"},
	{Code: "CHF", Name: "Swiss Franc", Symbol: "Fr"},
	{Code: "CLP", Name: "Chilean Peso", Symbol: "$"},
	{Code: "CNY", Name: "Chinese Yuan", Symbol: "¥"},
	{Code: "COP", Name: "Colombian Peso", Symbol: "$"},
	{Code: "CRC", Name: "Costa Rican Colón", Symbol: "₡"},
	{Code: "CUP", Name: "Cuban Peso", Symbol: "$"},
	{Code: "CVE", Name: "Cape Verdean Escudo", Symbol: "$"},
	{Code: "CZK", Name: "Czech Koruna", Symbol: "Kč"},
	{Code: "DJF", Name: "Djiboutian Franc", Symbol: "Fdj"},
	{Code: "DKK", Name: "Danish Krone", Symbol: "kr"},
	{Code: "DOP", Name: "Dominican Peso", Symbol: "$"},
	{Code: "DZD", Name: "Algerian Dinar", Symbol: "دج"},
	{Code: "EGP", Name: "Egyptian Pound", Symbol: "E£"},
	{Code: "ERN", Name: "Eritrean Nakfa", Symbol: "Nfk"},
	{Code: "ETB", Name: "Ethiopian Birr", Symbol: "Br"},
	{Code: "EUR", Name: "Euro", Symbol: "€"},
	{Code: "FJD", Name: "Fijian Dollar", Symbol: "$"},
	{Code: "FKP", Name: "Falkland Islands Pound", Symbol: "£"},
	{Code: "GBP", Name: "British Pound", Symbol: "£"},
	{Code: "GEL", Name: "Georgian Lari", Symbol: "₾"},
	{Code: "GHS", Name: "Ghanaian Cedi", Symbol: "₵"},
	{Code: "GIP", Name: "Gibraltar Pound", Symbol: "£"},
	{Code: "GMD", Name: "Gambian Dalasi", Symbol: "D"},
	{Code: "GNF", Name: "Guinean Franc", Symbol: "FG"},
	{Code: "GTQ", Name: "Guatemalan Quetzal", Symbol: "Q"},
	{Code: "GYD", Name: "Guyanese Dollar", Symbol: "$"},
	{Code: "HKD", Name: "Hong Kong Dollar", Symbol: "HK$"},
	{Code: "HNL", Name: "Honduran Lempira", Symbol: "L"},
	{Code: "HTG", Name: "Haitian Gourde", Symbol: "G"},
	{Code: "HUF", Name: "Hungarian Forint", Symbol: "Ft"},
	{Code: "IDR", Name: "Indonesian Rupiah", Symbol: "Rp"},
	{Code: "ILS", Name: "Israeli New Shekel", Symbol: "₪"},
	{Code: "INR", Name: "Indian Rupee", Symbol: "₹"},
	{Code: "IQD", Name: "Iraqi Dinar", Symbol: "ع.د"},
	{Code: "IRR", Name: "Iranian Rial", Symbol: "﷼"},
	{Code: "ISK", Name: "Icelandic Króna", Symbol: "kr"},
	{Code: "JMD", Name: "Jamaican Dollar", Symbol: "$"},
	{Code: "JOD", Name: "Jordanian Dinar", Symbol: "د.ا"},
	{Code: "JPY", Name: "Japanese Yen", Symbol: "¥"},
	{Code: "KES", Name: "Kenyan Shilling", Symbol: "KSh"},
	{Code: "KGS", Name: "Kyrgyzstani Som", Symbol: "с"},
	{Code: "KHR", Name: "Cambodian Riel", Symbol: "៛"},
	{Code: "KMF", Name: "Comorian Franc", Symbol: "CF"},
	{Code: "KPW", Name: "North Korean Won", Symbol: "₩"},
	{Code: "KRW", Name: "South Korean Won", Symbol: "₩"},
	{Code: "KWD", Name: "Kuwaiti Dinar", Symbol: "د.ك"},
	{Code: "KYD", Name: "Cayman Islands Dollar", Symbol: "$"},
	{Code: "KZT", Name: "Kazakhstani Tenge", Symbol: "₸"},
	{Code: "LAK", Name: "Lao Kip", Symbol: "₭"},
	{Code: "LBP", Name: "Lebanese Pound", Symbol: "ل.ل"},
	{Code: "LKR", Name: "Sri Lankan Rupee", Symbol: "Rs"},
	{Code: "LRD", Name: "Liberian Dollar", Symbol: "$"},
	{Code: "LSL", Name: "Lesotho Loti", Symbol: "L"},
	{Code: "LYD", Name: "Libyan Dinar", Symbol: "ل.د"},
	{Code: "MAD", Name: "Moroccan Dirham", Symbol: "د.م."},
	{Code: "MDL", Name: "Moldovan Leu", Symbol: "L"},
	{Code: "MGA", Name: "Malagasy Ariary", Symbol: "Ar"},
	{Code: "MKD", Name: "Macedonian Denar", Symbol: "ден"},
	{Code: "MMK", Name: "Myanmar Kyat", Symbol: "K"},
	{Code: "MNT", Name: "Mongolian Tögrög", Symbol: "₮"},
	{Code: "MOP", Name: "Macanese Pataca", Symbol: "MOP$"},
	{Code: "MRU", Name: "Mauritanian Ouguiya", Symbol: "UM"},
	{Code: "MUR", Name: "Mauritian Rupee", Symbol: "Rs"},
	{Code: "MVR", Name: "Maldivian Rufiyaa", Symbol: "Rf"},
	{Code: "MWK", Name: "Malawian Kwacha", Symbol: "MK"},
	{Code: "MXN", Name: "Mexican Peso", Symbol: "$"},
	{Code: "MYR", Name: "Malaysian Ringgit", Symbol: "RM"},
	{Code: "MZN", Name: "Mozambican Metical", Symbol: "MT"},
	{Code: "NAD", Name: "Namibian Dollar", Symbol: "$"},
	{Code: "NGN", Name: "Nigerian Naira", Symbol: "₦"},
	{Code: "NIO", Name: "Nicaraguan Córdoba", Symbol: "C$"},
	{Code: "NOK", Name: "Norwegian Krone", Symbol: "kr"},
	{Code: "NPR", Name: "Nepalese Rupee", Symbol: "Rs"},
	{Code: "NZD", Name: "New Zealand Dollar", Symbol: "NZ$"},
	{Code: "OMR", Name: "Omani Rial", Symbol: "ر.ع."},
	{Code: "PAB", Name: "Panamanian Balboa", Symbol: "B/."},
	{Code: "PEN", Name: "Peruvian Sol", Symbol: "S/"},
	{Code: "PGK", Name: "Papua New Guinean Kina", Symbol: "K"},
	{Code: "PHP", Name: "Philippine Peso", Symbol: "₱"},
	{Code: "PKR", Name: "Pakistani Rupee", Symbol: "Rs"},
	{Code: "PLN", Name: "Polish Złoty", Symbol: "zł"},
	{Code: "PYG", Name: "Paraguayan Guaraní", Symbol: "₲"},
	{Code: "QAR", Name: "Qatari Riyal", Symbol: "ر.ق"},
	{Code: "RON", Name: "Romanian Leu", Symbol: "lei"},
	{Code: "RSD", Name: "Serbian Dinar", Symbol: "дин"},
	{Code: "RUB", Name: "Russian Ruble", Symbol: "₽"},
	{Code: "RWF", Name: "Rwandan Franc", Symbol: "FRw"},
	{Code: "SAR", Name: "Saudi Riyal", Symbol: "﷼"},
	{Code: "SBD", Name: "Solomon Islands Dollar", Symbol: "$"},
	{Code: "SCR", Name: "Seychellois Rupee", Symbol: "Rs"},
	{Code: "SDG", Name: "Sudanese Pound", Symbol: "£"},
	{Code: "SEK", Name: "Swedish Krona", Symbol: "kr"},
	{Code: "SGD", Name: "Singapore Dollar", Symbol: "S$"},
	{Code: "SHP", Name: "Saint Helena Pound", Symbol: "£"},
	{Code: "SLE", Name: "Sierra Leonean Leone", Symbol: "Le"},
	{Code: "SOS", Name: "Somali Shilling", Symbol: "Sh"},
	{Code: "SRD", Name: "Surinamese Dollar", Symbol: "$"},
	{Code: "SSP", Name: "South Sudanese Pound", Symbol: "£"},
	{Code: "STN", Name: "São Tomé and Príncipe Dobra", Symbol: "Db"},
	{Code: "SYP", Name: "Syrian Pound", Symbol: "£"},
	{Code: "SZL", Name: "Swazi Lilangeni", Symbol: "L"},
	{Code: "THB", Name: "Thai Baht", Symbol: "฿"},
	{Code: "TJS", Name: "Tajikistani Somoni", Symbol: "SM"},
	{Code: "TMT", Name: "Turkmenistani Manat", Symbol: "m"},
	{Code: "TND", Name: "Tunisian Dinar", Symbol: "د.ت"},
	{Code: "TOP", Name: "Tongan Paʻanga", Symbol: "T$"},
	{Code: "TRY", Name: "Turkish Lira", Symbol: "₺"},
	{Code: "TTD", Name: "Trinidad and Tobago Dollar", Symbol: "TT$"},
	{Code: "TWD", Name: "New Taiwan Dollar", Symbol: "NT$"},
	{Code: "TZS", Name: "Tanzanian Shilling", Symbol: "TSh"},
	{Code: "UAH", Name: "Ukrainian Hryvnia", Symbol: "₴"},
	{Code: "UGX", Name: "Ugandan Shilling", Symbol: "USh"},
	{Code: "USD", Name: "US Dollar", Symbol: "$"},
	{Code: "UYU", Name: "Uruguayan Peso", Symbol: "$U"},
	{Code: "UZS", Name: "Uzbekistani Som", Symbol: "soʻm"},
	{Code: "VES", Name: "Venezuelan Bolívar", Symbol: "Bs.S"},
	{Code: "VND", Name: "Vietnamese Đồng", Symbol: "₫"},
	{Code: "VUV", Name: "Vanuatu Vatu", Symbol: "VT"},
	{Code: "WST", Name: "Samoan Tālā", Symbol: "T"},
	{Code: "XAF", Name: "Central African CFA Franc", Symbol: "FCFA"},
	{Code: "XCD", Name: "East Caribbean Dollar", Symbol: "EC$"},
	{Code: "XCG", Name: "Caribbean Guilder", Symbol: "Cg"},
	{Code: "XOF", Name: "West African CFA Franc", Symbol: "CFA"},
	{Code: "XPF", Name: "CFP Franc", Symbol: "₣"},
	{Code: "YER", Name: "Yemeni Rial", Symbol: "﷼"},
	{Code: "ZAR", Name: "South African Rand", Symbol: "R"},
	{Code: "ZMW", Name: "Zambian Kwacha", Symbol: "ZK"},
	{Code: "ZWG", Name: "Zimbabwe Gold", Symbol: "ZiG"},
	{Code: "XAU", Name: "Gold (troy ounce)", Symbol: "XAU", Crypto: true},
	{Code: "XAG", Name: "Silver (troy ounce)", Symbol: "XAG", Crypto: true},
	{Code: "BTC", Name: "Bitcoin", Symbol: "₿", Crypto: true},
	{Code: "ETH", Name: "Ethereum", Symbol: "Ξ", Crypto: true},
	{Code: "USDT", Name: "Tether", Symbol: "USDT", Crypto: true},
	{Code: "USDC", Name: "USD Coin", Symbol: "USDC", Crypto: true},
	{Code: "SOL", Name: "Solana", Symbol: "SOL", Crypto: true},
}

// symbols links the symbols people use as currency names to a currency,
// where the symbol means one currency only. "$", "¥" and the like are
// shared, so they aren't here.
var symbols = map[string]string{
	"€": "EUR", "£": "GBP", "₾": "GEL", "₸": "KZT", "₽": "RUB", "₴": "UAH", "₹": "INR", "₺": "TRY",
	"₩": "KRW", "zł": "PLN", "₪": "ILS", "₫": "VND", "₱": "PHP", "฿": "THB", "₼": "AZN", "֏": "AMD",
	"₮": "MNT", "₦": "NGN", "₲": "PYG", "₡": "CRC", "₭": "LAK", "₵": "GHS", "Kč": "CZK", "Ft": "HUF",
	"lei": "RON", "R$": "BRL", "US$": "USD", "A$": "AUD", "C$": "CAD", "NZ$": "NZD", "HK$": "HKD",
	"S$": "SGD", "₿": "BTC", "Ξ": "ETH",
}

// All lists every known currency, by code.
func All() []Currency {
	return append([]Currency(nil), all...)
}

// Find looks a currency up by its code, ignoring case.
func Find(code string) (Currency, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, c := range all {
		if c.Code == code {
			return c, true
		}
	}

	return Currency{}, false
}

// CodeFor guesses which currency a name stands for: the name is a code
// itself ("eur"), or a symbol that means one currency only ("€"). It
// returns "" when it can't tell, like for "$".
func CodeFor(name string) string {
	name = strings.TrimSpace(name)
	if c, ok := Find(name); ok {
		return c.Code
	}

	return symbols[name]
}
