# Roadmap

Пункти впорядковані за пріоритетом усередині кожного розділу. Усе, що
нижче, — поза кореневим функціоналом (клієнт + роутер + FSM) і може жити в
окремих підпакетах, щоб `gogram` лишався без зайвих залежностей.

## Зроблено (жовтень 2026)

- Один прохід JSON на виклик, пул буферів, плоскі `With*`-опції без алокацій.
- `Caller`/`Transformer` pipeline: `Throttle` (per-chat flood control),
  `Logging` (slog), `WithCaller` для тестів/альтернативних транспортів.
- Typed callback data `Callback[T]`, `InlineKeyboardBuilder`.
- `New<Method>Params` builders, `Detached()`, typed context keys `Key[T]`.
- FSM на `goshard` з мемоізацією стану на апдейт; `sqlstore` підпакет.
- `tgtest`: `Recorder` + конструктори апдейтів + assert-хелпери.
- `client.Me()` (кешований `GetMe`).
- Толерантність до невідомих підтипів у `getUpdates`.

## Наступне

1. **Pagination + Calendar** — вирішити розташування (`pkg/`-підпакети
   `paginator` і `calendar` поверх `Callback[T]` + `InlineKeyboardBuilder`;
   без залежностей, крім `gogram`).
2. **Композиція фільтрів** методами (`f.And(g)`, `f.Or(g)`, `f.Not()`) і
   відсутні фільтри: `FilterPrivate/Group/Channel`, `FilterMessagePhoto/Media`,
   `FilterUsers(ids...)`.
3. **`WithStorer`** на рівні клієнта → `ctx.FSM()` і `FilterState("name")`
   без аргументу storer; Conversation DSL.
4. Deep links (`BotLink`) поверх `client.Me()`.
5. `sqlstore`: TTL/очищення, тест на SQLite в CI.
6. `WithLocalAddrs` (multi-IP egress), low priority.

## Pagination

Задача: універсальний пагінатор списку елементів у inline-клавіатурі
з навігацією `‹ 1/7 ›`, без стану на сервері (сторінка кодується у callback data).

- `Paginate(items []PaginationItem, page, perPage int, navPrefix string)` —
  детермінована побудова клавіатури; `navPrefix page` як callback data,
  тобто обробник реєструється одним `HandleInlineKeyboardButton`.
- Опції: кількість колонок, формат лічильника, кнопки «на початок/в кінець»,
  додатковий нижній ряд (наприклад, «Назад»).
- Джерело даних — зріз (`[]PaginationItem`) **і** `iter.Seq` з лінивим
  завантаженням через `func(page int) ([]T, total int)`, щоб не тягнути
  весь список із БД заради однієї сторінки.
- Контракт на довжину `callback_data` (≤ 64 байти): валідація на етапі
  побудови, а не під час відправки.

## Calendar

Inline-календар для вибору дати (поширений запит у ботах бронювання/нагадувань):

- Сітка місяця з навігацією за місяцями/роками, локалізація назв днів та
  місяців, перший день тижня (пн/нд), мінімальна/максимальна дата,
  заборонені дні (вихідні, зайняті слоти) через `func(time.Time) bool`.
- Без стану: вибраний місяць кодується у callback data (`cal 2026-10`),
  результат — `cal 2026-10-12`. Обробник отримує `time.Time`.
- Опційно: вибір часу (слоти по N хвилин) як другий крок у тому ж обробнику.
- Пакет `calendar` поверх keyboard builder; залежність лише від `gogram`.

## Deep links (колишній `link.go`)

`client.Me()` уже є; лишилося `client.Link(ctx, gogram.Start("ref123"))` /
`StartGroup(...)` з валідацією параметра (1–64 символи `[A-Za-z0-9_-]`,
константа `StartParamMaxLen`).

## Multi-IP egress (`WithLocalAddrs`)

Опт-ін без автодетекту: `WithLocalAddrs(ips ...netip.Addr)` будує по
`http.Transport` на адресу (`Dialer{LocalAddr}`), round-robin + failover на
dial-помилку; TLS/проксі беруться з клонованого базового транспорту.
Ліміти Telegram — на токен, тож це лише відмовостійкість аплінків. Low priority.

## Решта

- Webhook: варіант з обробкою апдейтів через той самий worker-pool, що й
  polling, щоб довгі хендлери не тригерили ретраї Telegram.
- Бенчмарки: multipart-завантаження.
- Хелпери тексту: `EscapeMarkdownV2/EscapeHTML`, `Message.EntityText(e)`
  з UTF-16 offset-ами.

## Конкуренти (стан на 2026-10-12)

Джерело — GitHub (зірки, релізи, issues). Reddit/HN не верифіковано.

| Бібліотека | ★ | Bot API | Кодоген | Роутер/MW | FSM | UI (paginator/calendar) | Rate limit / 429 | `context.Context` | Статус |
|---|---|---|---|---|---|---|---|---|---|
| [go-telegram/bot](https://github.com/go-telegram/bot) | 1.9k | 10.3 | ні (ручні моделі, `reflect` у multipart) | так / так | окремий `fsm` (заморожено) | окремий `ui` (заморожено 2025) | ні ([#124](https://github.com/go-telegram/bot/issues/124)) / тільки getUpdates | так | активна |
| [mymmrac/telego](https://github.com/mymmrac/telego) | 1.1k | 10.3 | так | так / так | ні | builders, без пагінації | ні / `RetryCaller` (стрімінг і retry взаємовиключні) | так | активна |
| [tucnak/telebot v4](https://github.com/tucnak/telebot) | 4.6k | ~10.1 | ні | так / так | ні ([#288](https://github.com/tucnak/telebot/issues/288), 6 років) | `layout` (YAML) | ні / `FloodError` | **ні** ([#602](https://github.com/tucnak/telebot/issues/602)) | beta ≥2 роки |
| [gotgbot v2](https://github.com/PaulSonOfLars/gotgbot) | 0.7k | 10.3 | так (spec щодня) | dispatcher / ні | **вбудований Conversation** | ні | ні / ні | частково | rc.36 роками |
| [tgbotapi v5](https://github.com/go-telegram-bot-api/telegram-bot-api) | 6.4k | 6.0 | ні | ні | ні | ні | ні | ні | **мертва** (2022) |
| [echotron](https://github.com/NicoNex/echotron) | 0.4k | ? | ні | ні | `stateFn` | ні | **вбудований** 30/s + 20/min per-chat | ні | активна, LGPL |

**Що є у всіх:** polling + webhook, 1:1 типи, `*Params`, типізована помилка з `retry_after`, upload через `io.Reader`.

**Де gogram уже закриває прогалини:**
- кодогенерація з офіційних доків + `context.Context` всюди + одна залежність (`x/time`, `goshard`) — комбінація, якої немає ні в кого;
- rate limiter + retry на 429 із `Retry-After` з коробки (найчастіша скарга: go-telegram #124, telebot #782, telego #262/#331);
- стрімінговий multipart через `io.Pipe` без буферизації (go-telegram відмовився у v1.27, telego вимагає `BufferRequestData` → OOM);
- FSM з pluggable storage (in-memory sharded + SQL) і мемоізацією стану на апдейт;
- O(1) fast path для команд/callback і `/cmd@botname` (біль go-telegram #174/#240);
- `ctx.User()/Chat()/Message()` для будь-якого типу апдейту (go-telegram #154).

**Що просять, а ніхто не дає добре → план для gogram:**
1. **Per-chat flood control** (1 msg/s у приватному чаті, 20/min у групі) поверх глобального 30 rps — лише echotron, без роутера. Реалізація: `goshard.Map[chatID, *rate.Limiter]` з TTL-очищенням, опція `WithChatRPS`.
2. **FSM timeouts** — у gotgbot TODO; у нас є `WithTTL` для in-memory, потрібен аналог для SQL (`updated_at` уже є).
3. **UI: paginator + calendar** — лише go-telegram/ui (заморожено) і telebot `layout`. Див. розділи вище.
4. **Observability**: `slog`-логер і OTel-хуки (span на запит/хендлер) — немає ні в кого; telebot без ctx це неможливо в принципі.
5. **Толерантність до невідомих discriminator-ів** у `getUpdates` (telego #369 deadlock, go-telegram виправив у v1.26) — перевірити наші union `UnmarshalJSON` на невідомий `type` і додати тест.
6. **Тест-хелпери**: фейковий Bot API server (`gogram/tgtest`) з канонічними відповідями — у конкурентів лише інтерфейси для моків.
7. **Стабільний semver** — скарги на breaking у minor (telebot #740), rc роками (gotgbot), bump `go` directive (telego #274). Правило: `go` directive = N-1, breaking лише в major, CHANGELOG.
8. **UTF-16 offsets у entities** (telego #94) — хелпер для розрахунку/нарізки entity по UTF-16.

## Конкуренти поза Go (стан на 2026-10-13)

| Фреймворк | Версія / ★ / Bot API | Що люблять | Головні скарги |
|---|---|---|---|
| python-telegram-bot | 22.8 / 29.5k / 10.0 | `effective_user/chat/message`, `ConversationHandler`, arbitrary `callback_data` (UUID-кеш), `JobQueue` | відставання від API ([#5261](https://github.com/python-telegram-bot/python-telegram-bot/issues/5261)); `ConversationHandler` заплутаний ([#2770](https://github.com/python-telegram-bot/python-telegram-bot/issues/2770)); `AIORateLimiter` робить глобальну паузу; немає тестового інструментарію |
| aiogram 3 | 3.31 / 5.9k / 10.3 | `Router` + magic-filters `F.text == "x"`, 60+ аліасів на типах (`message.answer`), `CallbackData` factory, `InlineKeyboardBuilder.adjust()`, FSM зі стратегіями ключа, DI через middleware `data` | throttling зник у 3.x ([#1413](https://github.com/aiogram/aiogram/issues/1413)); Scenes experimental; пам'ять pydantic ([#1513](https://github.com/aiogram/aiogram/issues/1513)); OTel немає ([#1785](https://github.com/aiogram/aiogram/issues/1785)) |
| pyTelegramBotAPI | 4.37 / 8.8k / 10.3 | нульовий поріг входу | `register_next_step_handler` в пам'яті процесу; глобальні перемикачі `apihelper.ENABLE_MIDDLEWARE` |
| grammY (TS) | 1.46 / 3.8k / 10.3 | filter-queries зі звуженням типу, `ctx.msg`, **API transformers** (auto-retry/throttle/hydrate/mock/tracing на одному хуку), stateless menu з fingerprint, conversations v2, runner із `sequentialize`, `fmt` → entities | два види flavors плутають ([#709](https://github.com/grammyjs/grammY/issues/709)); replay-conversations і side effects; немає тестів ([#903](https://github.com/grammyjs/grammY/issues/903)); v2 без ETA |
| Telegraf (TS) | 4.16 (2024) / 9.2k / 7.1 | Scenes/Wizard, `Markup`, база прикладів | стагнація ([#2060](https://github.com/telegraf/telegraf/issues/2060)); типи сцен ([#894](https://github.com/telegraf/telegraf/issues/894)) |
| node-telegram-bot-api v2 | 2.x (2026-08) / 9.2k / 10.3 | koa-middleware, один params-об'єкт, retry + ratelimit з коробки, 409 як recoverable, типи з документації | повна несумісність з v1 |
| teloxide (Rust) | 0.17 (2025-07) / 4.2k / 9.1 | `#[derive(BotCommands)]`, `Dialogue` з enum-станами, **адаптери** `Throttle` (per-chat/s, per-chat/min, per-channel/min, overall/s, зберігає порядок) / `CacheMe` / `Trace` | runtime-DI dptree з паніками ([#871](https://github.com/teloxide/teloxide/issues/871)); відставання API ([#1346](https://github.com/teloxide/teloxide/issues/1346)); повільний shutdown ([#711](https://github.com/teloxide/teloxide/issues/711)) |
| Telegram.Bot (C#) | 22.10 / 3.6k / 10.3 | тонкий актуальний клієнт, implicit conversions, авто-retry 429 (`RetryThreshold=60`, `RetryCount=3`) | breaking renames у кожному minor; роутера/FSM немає |
| TelegramBots (Java) | 10.3 / 5.5k / 10.3 | 1:1 модель, Spring starters, AbilityBot | `onRegister()` вручну ([#1354](https://github.com/rubenlagus/TelegramBots/issues/1354)); деталі помилок маскуються ([#1452](https://github.com/rubenlagus/TelegramBots/issues/1452)) |
| Nutgram (PHP) | 4.50 / 755 / 10.3 | `onCommand('start {name}')` з параметрами, `$bot->sendMessage()` без chat_id, `InlineMenu`, Conversation-класи, **еталонний fake/assert DSL для тестів** | multi-bot ([#487](https://github.com/nutgram/nutgram/issues/487)) |

**Що запозичити (понад Go-прогалини вище):**
1. **Transformer-pipeline на рівні клієнта** (grammY `api.config.use`, teloxide adaptors): `client.Use(func(next Caller) Caller)` з `(ctx, method, body) → result`. На ньому будуються auto-retry, throttle, logging, OTel і **mock для тестів** — один механізм замість п'яти опцій.
2. **Throttle з правильними лімітами** (teloxide `Limits`): per-chat/s, per-chat/min, per-channel-or-supergroup/min, overall/s; зберігати порядок у чаті; не глобальна пауза як у PTB.
3. **Hydrate-результати**: `ctx.SendMessage` → `*SentMessage` з `.EditText/.Delete/.Pin` і збереженим клієнтом/chat_id.
4. **Typed callback data** (aiogram `CallbackData`, PTB arbitrary data): generics-factory + `InvalidCallbackData` для застарілих кнопок; **stateless menu** з fingerprint (grammY menu) — основа для пагінації/календаря.
5. **Декларативні команди**: реєстр з описом/scope/локалізацією → `SetMyCommands` автоматично (grammY commands, Nutgram, teloxide `BotCommands`).
6. **Outer/inner middleware + handler flags** (aiogram): middleware до фільтрів бачить усе; `WithFlag("chat_action", "typing")` як метадані хендлера.
7. **Fake-клієнт + assert DSL** (Nutgram): `tgtest.Fake(router).HearText("/start").AssertReply("sendMessage", …)` через transformer №1 — працює з реальними хендлерами без мережі.
8. **OTel-first**: span на апдейт у middleware, child-span на API-виклик у transformer, `getUpdates` виключено за замовчуванням.
9. **Conversations**: лінійний `wait()`-стиль (grammY v2/televerse) через goroutine+channel, **не** replay — уникнути «golden rule» пастки.
10. **Runner**: `Sequentialize` per-chat (вже є через `chatID % numWorkers`), backpressure, короткий дефолтний graceful-shutdown таймаут, 409 Conflict як recoverable з лімітом.

**Анти-патерни (не повторювати):** runtime-DI за типом з паніками (teloxide); глобальні mutable-перемикачі модуля (telebot); «магічний» life-cycle метод, який треба викликати вручну (Java `onRegister`); перестановка позиційних параметрів між minor (Telegram.Bot, PTB); два способи типізації контексту (grammY flavors); replay-conversations без захисту від side effects; відставання від Bot API — найреакційніші issues у PTB/teloxide/Telegraf.
