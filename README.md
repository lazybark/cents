# cents

## Plans

- [ ] Add a "goals" manager (to track financial goals and progress towards them) (name a goal, set target amount overall or target amount on specific account, track progress on progress bar)
- [ ] Invoice tracker (sent) (to track invoices with recepient, date, number, also optional link to the invoice file, and payment status)
- [ ] Taxes (track tax payments and deductions)
- [ ] Flow to rename currency and not lose the data for accounts & other entities related to the currency (like transactions, budgets, etc.)
- [ ] Same flow for payment methods.
- [ ] Store DB schema by versions once we have 0.0.1 release, to be able to track changes and update the DB schema on app updates without losing data.
- [ ] Backup and restore DB.
- [ ] When user opens app, it tries to fetch DB. If no DB or it's empty (lacks settings table) - suggest user to create new DB or open existing. Then this setting is saved in some config file.
- [ ] CSV exports for everything (accounts, transactions, budgets, goals, debts, invoices, etc.)