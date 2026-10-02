# cents

## Plans

- Export to CSV (specific account, expense, history, etc.)
- Import is the other way around: import via join, replacing existing data where necessary
- Automatic currency exchange update, with writing down the latest time in config, noting it in the interface and refreshing in case it was outdated

invoice could be linked to an income or expense, but it needs some smart filtering, because there will be a lot of invoices for some users. And we need to sort them in this dropdown reversed by date. So that user can easily create invoice and then proceed to adding some related transactions.



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

Page with historical account data, where all accounts are aggregated historically