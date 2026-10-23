# Importing records from a cents database

In the **desktop app**, open **Backup & export → Import from another database →
Choose database…**. Pick an unencrypted cents `.db` file or backup made with the
current app. The source file must be a complete SQLite snapshot, not a copy of
a database that is being written; the existing Backup action makes one safely.

The preview shows the selected file, the current destination (a local database
or server), and counts by record type. It has not added any records. Click
**Import records** to merge, or **Cancel** to leave the destination unchanged.
The app refreshes all views after a successful import.

- The source is read from your computer. In server mode, the desktop uploads it
  to the connected server using your current bearer token. A read-only token
  cannot import. The server and desktop both need the import-capable build.
- Accounts, entries, tags, categories, currencies, budgets, invoices,
  subscriptions, obligations, assets, debts, credits, goals, taxes, histories
  and month notes are imported together. Database-local IDs and references are
  remapped. Source-only custom SQL, triggers and views are not imported.
- Existing records win conflicts; this is an add-missing-records operation,
  rather than synchronization of edits. Current balances, settings and backup
  paths stay as they are. Importing the same snapshot repeatedly skips rows
  already present. Different transactions with equal amounts are retained.
- Both budgets must use the same base currency because stored base amounts
  are not converted. A new, empty budget with no currency setup can adopt the
  source's base currency. Named categories/tags and matching currencies are
  reused; accounts with different identities remain separate accounts.
- Both source and destination must be unencrypted for this first version.
  The source file is capped at 32 MB and 100,000 rows. Incompatible older
  schemas must be opened and backed up with the current desktop app first.
- Confirmation uses the exact snapshot previewed. Changing the selected
  destination requires a new preview. Each merge commits atomically: an error
  rolls back all records and the duplicate-detection metadata.

This flow is only in the desktop app. The Telegram Mini App has no import
control or import endpoint. The guarded server endpoint is
`POST /api/v1/import?preview=true` (preview is the default) or `preview=false`
to commit, with `Content-Type: application/vnd.sqlite3` and a binary body.
Ordinary JSON endpoints keep their existing 1 MB request limit.

After updating the code, rebuild the desktop with `make build`. For a server
running in Docker, run `make build-docker` to rebuild `cents-cloud:local`, then
recreate the container using that image with its existing data volume and configs.
Restarting an existing container alone does not pick up a rebuilt image.
No additional environment variables are required.
