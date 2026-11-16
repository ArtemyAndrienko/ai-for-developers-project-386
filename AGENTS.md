# AGENTS.md

Инструкции ИИ-агентам для работы в этом репозитории.

## Команды

| Команда | Что делает |
| --- | --- |
| `mise install` | ставит закреплённые инструменты (go 1.27.4, node 24.22.2 из `mise.toml`) |
| `mise run setup` | npm-зависимости `frontend/` и `typespec/` |
| `mise run dev-backend` | API на http://localhost:8080 (при старте создаётся демо-встреча) |
| `mise run dev-frontend` | UI на http://localhost:5173 (vite проксирует `/api` на `:8080`) |
| `mise run lint` | gofmt+go vet, ESLint, проверка формата `main.tsp` |
| `mise run test` | `check-contract` + `go test` + `vitest` |
| `mise run build` | `backend/bin/calendar-api` + `frontend/dist` |
| `mise run check` | **все проверки разом** (то же выполняет CI на каждый push) |
| `mise run codegen` | TypeSpec → `openapi.json` → `frontend/src/api/schema.d.ts` |

## Структура

- `backend/` — Go (только stdlib `net/http`): `main.go` — роуты/JSON,
  `store.go` — хранилище и бизнес-правила, `slots.go` — генерация интервалов дня
- `frontend/` — Vite + vanilla TypeScript: `src/views/{book,create,mine}.ts`,
  `src/api.ts` — обёртка над сгенерированным SDK
- `typespec/` — контракт API `main.tsp` и результат `tsp-output/…/openapi.json`
- `scripts/check-contract.sh` — сверка маршрутов OpenAPI ↔ Go (входит в `test`)

## Правила (неочевидные из кода)

- **Коммиты — Conventional Commits**: `feat|fix|docs|refactor|test|chore|ci(scope):imperative
  subject`, например `feat(backend): per-owner workday hours`.
- Контракт API правится **только** в `typespec/main.tsp`, затем `mise run codegen`.
  Файлы `typespec/tsp-output/**` и `frontend/src/api/schema.d.ts` — артефакты
  генерации, руками не редактировать; `frontend/src/types.ts` — только ре-экспорт
  схем из `schema.d.ts`.
- Хранилище in-memory в Go-структурах: данные живут до рестарта процесса;
  замена на embedded-БД планируется, публичные методы `Store` менять не стоит.
- Занятость интервалов сквозная: брони одного создателя (по email) блокируют
  пересекающиеся слоты во всех его встречах на ту же дату; границы рабочего дня
  хранятся парой (email, дата), дефолт 09:00–18:00.
- Не удалять и не переименовывать `.github/workflows/hexlet-check.yml` и репозиторий.
