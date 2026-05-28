# cents

## Plans

- [ ] Flow to rename currency and not lose the data for accounts & other entities related to the currency (like transactions, budgets, etc.)
- [ ] Same flow for payment methods.
- [ ] Store DB schema by versions once we have 0.0.1 release, to be able to track changes and update the DB schema on app updates without losing data.
Backup and restore DB.

When user opens app, it tries to fetch DB. If no DB or it's empty (lacks settings table) - suggest user to create new DB or open existing. Then this setting is saved in some config file. So basically we have to be able to fetch any DB file.

CSV exports for everything (accounts, transactions, budgets, goals, debts, invoices, etc.)

Analytics features: montly stats, yearly stats, category stats, etc.

Account history log: maybe in case account amount change we can log this and show history of changes for the account, with possibility to filter by date, etc.

Edit account: ability to enter name of payment method or set an ID of existing.

Make sure app can init DB on first launch. And also can restore all tables in case one of them is missing.

Store settings in JSON file? Then we can easily setup app. And we do not depend on any DB to actually store settings. We can keep there path to DB.

Data importer (CSV, JSON)

Credits: loaner (bank, other org), from date, due date, amount, currency, logs

General data on main screen. Like monthly subscr cost, total money on all accounts, total debts, unpaid taxes, unpaid invoices, total goals progress.