## Target Clean Architecture (Option B)

### Layered Topology

- `domain/`: чистый слой бизнес-правил. Здесь лежат:
  - `domain/policy`: сущности и правила доступа (`Principal`, `Action`, owner policies).
  - `domain/authz`: абстракции контроля доступа (owner-control, ACL-интерфейсы).
  - `domain/service`: бизнес-интерфейсы (`Service`, `Factory`, DTO).
- `application/`: сценарии использования, собирающие доменные порты и оркестрацию.
  - `application/crud`: конструкторы CRUD-сценариев, связывающие сервисы и политики.
  - `application/endpoints`: описание публичных use-case в терминах команд/запросов.
- `adapter/`: инфраструктурные реализации портов.
  - `adapter/http/gin`: HTTP-реализация endpoint-порта (Gin), трансформации ошибок.
  - `adapter/db/bun`: реализации репозиториев на Bun.
  - `adapter/log`: интеграция с zerolog/sentry.

### Порты и зависимости

- Доменные интерфейсы (порты) объявляются в `domain/*/port`.
- `application` зависит только от доменных портов и DTO.
- `adapter` реализует порты и зависит от конкретных внешних библиотек.
- Внешний слой (`cmd`, интеграции) связывает `application` с конкретными адаптерами.

### Миграция из текущей структуры

1. **Разделить публичные пакеты**:
   - переместить доменные типы из `pkg/policy`, `pkg/auth`, `pkg/service` в `domain/...`.
   - оставить в `adapter/...` всё, что зависит от `gin`, `bun`, `zerolog`, контекста HTTP.
2. **Определить порты**:
   - выделить интерфейсы репозиториев (`Repository`, `Getter`, `Lister`, ...) в `domain/service/port`.
   - определить порты для HTTP-эндпоинтов (команды/квери) в `domain/endpoint/port`.
3. **Обновить адаптеры**:
   - `pkg/endpoint` → `adapter/http/gin`.
   - `pkg/repository` → `adapter/db/bun`.
   - `internal/l` → `adapter/log`.
4. **Application orchestrators**:
   - собрать конструкторы CRUD и middleware в `application/crud`/`application/middleware`.
   - Упорядочить middleware как use-case декораторы (напр., ACL, audit).
5. **Документация и CI**:
   - обновить `docs/package-structure.md` ссылкой на чистую архитектуру.
   - проверить, что `go test ./...` покрывает новый layout.

### Дополнительные шаги

- Добавить `cmd/example` с проводкой use-case через адаптеры.
- Настроить линтеры (`golangci-lint`) для проверки зависимости слоёв.
- Рассмотреть генерацию wire-контейнеров для сборки адаптеров/application.
