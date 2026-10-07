# Contributing to AMLens

Frontend and backend share one repository so API and interface changes can be reviewed together. `main` is the stable branch. Use short-lived branches and pull requests for changes.

## Commits

Use Conventional Commits for every language:

- `feat(backend): add analysis endpoint`
- `fix(frontend): preserve selected graph node`
- `test(backend): cover invalid imports`
- `build(docker): update runtime image`
- `ci: verify deployment workflow`
- `docs: explain input schema`

Keep each commit focused on one logical change and include its tests. Do not add real customer data, hackathon archives, databases, analysis results or keys. Use synthetic examples.

## Localization

The frontend catalog is `frontend/src/i18n/messages.json`. Each entry has `en`, `ru` and `kk`; existing Russian source phrases serve as stable message keys. Preserve every `{0}`, `{1}` interpolation placeholder. Use `t()` in rendered text and `useLocale()` in components; never evaluate translated labels once at module startup. Use the active locale for display formatting and keep identifiers as strings.

Backend presentation translations live in `backend/internal/i18n/messages.json`, with source phrases matching persisted analytical explanations. Preserve printf placeholders and their order. Localization happens when serializing a response, without rewriting stored snapshots. Machine-readable field names, role codes, identifiers and numeric values remain unchanged.

English is the initial language. Do not infer the initial preference from browser language; respect the saved choice. Documentation is maintained in English. Update all three translations when adding interface text and run localization tests with the normal test suites.

## Before a pull request

Run the [README checks](README.md#checks-and-limitations). For container changes, also run `node scripts/smoke-compose.mjs`. Update documentation when changing HTTP contracts, input schemas or deployment settings.
