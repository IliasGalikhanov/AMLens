# Вклад в AMLens

Фронтенд и бэкенд находятся в одном репозитории: изменения API и интерфейса можно проверять вместе. Стабильная ветка — main; для изменений создавайте короткоживущие ветки и pull request.

## Коммиты

Используем Conventional Commits для всех языков. Язык кода не меняет формат коммита:

- feat(backend): add analysis endpoint
- fix(frontend): preserve selected graph node
- test(backend): cover invalid imports
- build(docker): update runtime image
- ci: verify deployment workflow
- docs: explain input schema

Один коммит должен решать одну логическую задачу; тесты изменения включайте в тот же коммит. Не добавляйте реальные данные клиентов, архивы хакатона, базы, результаты анализа и ключи. Используйте синтетические примеры.

## Перед pull request

Запустите проверки из [README](README.md#проверки-и-ограничения). Изменения контейнеров дополнительно проверяйте командой node scripts/smoke-compose.mjs. Обновляйте документацию при изменении HTTP-контракта, схем входных данных и настроек запуска.
