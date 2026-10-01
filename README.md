# cents

## Plans

- [ ] Flow to rename currency and not lose the data for accounts & other entities related to the currency (like transactions, budgets, etc.)
- [ ] Same flow for payment methods.
- [ ] Store DB schema by versions once we have 0.0.1 release, to be able to track changes and update the DB schema on app updates without losing data.
Backup and restore DB.

CSV exports for everything (accounts, transactions, budgets, goals, debts, invoices, etc.)

Analytics features: montly stats, yearly stats, category stats, etc.

Account history log: maybe in case account amount change we can log this and show history of changes for the account, with possibility to filter by date, etc.

Edit account: ability to enter name of payment method or set an ID of existing.

Data importer (CSV, JSON)

Credits: loaner (bank, other org), from date, due date, amount, currency, logs

How to make historical data NOT updated with currency changes? Need to store the resulting value at the moment of closing the debt

Page with historical account data, where all accounts are aggregated historically

Fix: account log update enter does not work on checkbox, only when user is back to Amount input

Make Update log on save by default as true

Make account history entries deletable

Fix: account list has a ? placeholders on unicode symbols in account names, though the actual name is OK