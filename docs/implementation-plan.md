# План имплементации

Конкретные изменения по файлам для функциональности из
[functional-spec.md](./functional-spec.md).

## Принцип

DDD-разделение на ограниченные поддомены. **Корень пакета поддомена — только
модель** (value objects, сущности, инварианты). Всё остальное — адаптеры, слой
юзкейсов, порты — в подпакетах. Общего пакета `domain`/`models` нет.

Порты (интерфейсы) объявляет потребитель — слой юзкейсов `deckbuilder/usecase` (гошная
идиома «accept interfaces»); адаптеры удовлетворяют их структурно, не импортируя
ядро. Перевод между поддоменами (`lexicon.Word` → `flashcard.Note`) — явная
трансляция в `deckbuilder/usecase`. TUI вызывает сценарии ядра только внутри `tea.Cmd` и
зависит только от `deckbuilder/usecase`.

## Структура проекта

Стандартный гошный layout + DDD-поддомены; в корне поддомена — модель, остальное — в
подпакетах:

```
cmd/anki/                 # composition root: флаги, сборка зависимостей, запуск
cmd/wikt-import/          # разовый импорт: сырой дамп kaikki -> компактный JSONL
internal/
  lexicon/                # ПОДДОМЕН — корень: только МОДЕЛЬ
    word.go               #   Word, PartOfSpeech, Gender, грам. value objects
    wiktionary/           #   подпакет-адаптер: live German Wiktionary API -> lexicon.Word
    wiktextract/          #   подпакет-адаптер: офлайн-дамп kaikki (dewiktionary) -> lexicon.Word
  flashcard/              # ПОДДОМЕН — корень: только МОДЕЛЬ
    note.go               #   Note, Field, Deck, DeckName + инвариант полноты
    anki/                 #   подпакет-адаптер: хранение карточек через pkg/ankiconnect
  deckbuilder/             # ЯДРО — корень: только МОДЕЛЬ
    suggestion.go         #   Suggestion
    usecase/              #   слой юзкейсов: AddWord, ScanDeck, Apply, Decks + порты
  tui/                    # слой интерфейса: Bubble Tea (зависит только от deckbuilder/usecase)
pkg/ankiconnect/          # переиспользуемый низкоуровневый HTTP-клиент AnkiConnect
```

Импорты — по пути модуля, например `anki/internal/lexicon`,
`anki/internal/deckbuilder/usecase`. Зависимости направлены внутрь: `deckbuilder/usecase`
зависит от портов, которые сам и объявляет; адаптеры зависят от модели своего
поддомена и инфраструктуры, но не от ядра.

## 1. Поддомен `lexicon` (`internal/lexicon`)

Модель — корень пакета (`word.go`):

- enum `PartOfSpeech`: `Noun`, `Verb`, `Adjective`, `Other`.
- `Word`: общая часть (`Lemma`, `PartOfSpeech`, `Definitions []string` —
  немецкие значения, `Russian []string` — русские переводы из `{{Übersetzungen}}`)
  плюс грамматические value objects под часть речи:
  - Noun: `Gender` (`m`/`f`/`n` → der/die/das), `Plural string`
  - Verb: `PartizipII string`, `Auxiliary string` (haben/sein), `Präteritum string`,
    `Reflexive bool`
  - Adjective: `Comparative string`, `Superlative string`

Адаптер — подпакет (`wiktionary/`), возвращает `lexicon.Word`:

- GET `https://de.wiktionary.org/w/api.php?action=query&prop=revisions&rvslots=main&rvprop=content&format=json&titles=<word>`
- Извлечение wikitext из ответа MediaWiki, парсинг шаблонов:
  - `{{Wortart|…|Deutsch}}` → часть речи
  - `{{Deutsch Substantiv Übersicht …}}` → `Genus`, `Nominativ Plural`
  - шаблон спряжения глагола → Partizip II, Hilfsverb, reflexive (`sich`)
  - секция `{{Übersetzungen}}`, строка `*{{ru}}:` → `Russian` (значения из
    `{{Ü|ru|…}}`/`{{Üt|ru|…}}`, ударение U+0301 снимается, дубликаты убираются)
- Запросы шлют заголовок `User-Agent` (Wikimedia отвечает 403 без него).
- Парсер `key=value` поверх содержимого шаблона.
- Обработка: страница отсутствует → ошибка «не найдено»; часть речи не распознана
  → `PartOfSpeech = Other` с определениями.

**Офлайн-адаптер — подпакет (`wiktextract/`)**, тоже реализует `Dictionary`:

- Источник — дамп kaikki.org `dewiktionary` (редакция Deutsch), одна JSON-запись
  на строку. `ParseEntry(line) (*lexicon.Word, ok)` маппит запись:
  - `lang_code != "de"` или `tags` содержит `"form-of"` → запись пропускается
    (это словоформы вроде «Genitiv Singular …», а не леммы).
  - род — из `tags` верхнего уровня (`masculine`/`feminine`/`neuter`);
  - формы из `forms[]` по тегам: Plural = `nominative`+`plural`; Partizip II =
    `participle`+`perfect`; Hilfsverb = `auxiliary`; Präteritum = теги ровно
    `["past"]`; Komparativ/Superlativ = `["comparative"]`/`["superlative"]`
    (берём первую одно-словную форму, для суперлатива допускаем префикс `am `);
  - определения — глоссы `senses[].glosses` (без form-of-значений), русский —
    `translations[]` с `lang_code:"ru"`. И то и другое ограничено (3 значения,
    6 переводов), чтобы карточка не разрасталась.
- `Open(path)` грузит компактный дамп в память (`map[lemma]Word`), `Lookup`
  делает точное совпадение, затем фолбэк по lower-case (леммы на карточках бывают
  не в том регистре). Возвращает `lexicon.ErrNotFound`, когда слова нет.
- `cmd/wikt-import -in raw.jsonl -out compact.jsonl` — разово стримит сырой дамп
  (поддержка `.gz`/`.bz2`), прогоняет `ParseEntry`, пишет компактный JSONL
  (json от `lexicon.Word`, по одной лемме). Это убирает 3 ГБ форм/примеров и
  делает старт быстрым.

Выбор источника — флаг `cmd/anki -dict <compact.jsonl>`; по умолчанию указывает
на `data/de-compact.jsonl`, при отсутствии файла — фолбэк на live API.

(Порт `Dictionary`, который реализует адаптер, объявлен в `deckbuilder/usecase` — см. §4.)

## 2. Поддомен `flashcard` (`internal/flashcard`)

Модель — корень пакета (`note.go`):

- `DeckName string`, `Deck`, `Field`.
- `Note`: `Front`, `Back`, `Tags []string`, привязка к deck/model.
- Инвариант полноты: `IsComplete(note) bool` (пустой Back или нет ожидаемых
  грам. полей → неполная). Живёт в корне, т. к. это инвариант модели карточки.

Адаптер — подпакет (`anki/`), реализует порт `Cards` (объявлен в `deckbuilder/usecase`,
§4) через `pkg/ankiconnect`, транслируя модель в DTO:

- `Add` → `addNote`; `Decks` → `deckNames`
- `FindIncomplete` → `findNotes` (`deck:"<name>"`) + `notesInfo`, фильтр по
  `IsComplete`
- `Update` → `updateNoteFields` (только переданные поля)

## 3. Низкоуровневый клиент (`pkg/ankiconnect`)

Без доменных типов приложения — только конверт и действия:

- **Починить `getRequest`** (сейчас тело ответа игнорируется → блокирует build):
  AnkiConnect всегда отвечает HTTP 200 с `{"result": …, "error": …}`. Декодировать в
  обёртку, вернуть ошибку при `error != null`, размаршалить `result` в переданную
  цель.
- Методы-действия: `AddNote`, `DeckNames`, `FindNotes`, `NotesInfo`,
  `UpdateNoteFields` (опц. `CreateDeck`). Каждый объявляет свой params-тип и
  делегирует общему `getRequest`.
- Заголовки запроса: `Content-Type: application/json` и
  `Origin: http://localhost` — без `Origin` из списка `webCorsOriginList`
  AnkiConnect отвечает HTTP 403. Проверять `resp.StatusCode` и отдавать понятную
  ошибку со статусом и телом.

## 4. Ядро `deckbuilder` (`internal/deckbuilder`)

Модель — корень пакета (`suggestion.go`):

- `Suggestion{ NoteID uint64, Lemma string, Fields map[string]string, Skipped bool, Reason string }`.

Слой юзкейсов — подпакет (`deckbuilder/usecase`):

Outbound-порты (объявлены здесь, у потребителя; реализуются адаптерами структурно):

- `Dictionary { Lookup(ctx, lemma string) (*lexicon.Word, error) }`
- `Cards` интерфейс:
  - `Add(ctx, deck flashcard.DeckName, note flashcard.Note) (id uint64, err error)`
  - `Decks(ctx) ([]flashcard.DeckName, error)`
  - `FindIncomplete(ctx, deck flashcard.DeckName) ([]flashcard.Note, error)`
  - `Update(ctx, id uint64, fields map[string]string) error`

Inbound-порт (от него зависит TUI, чтобы не тянуть конкретный `service`):

- `Service` интерфейс; реализует приватный `service` (`var _ Service = (*service)(nil)`):
  - `PreviewWord(ctx, lemma) (flashcard.Note, error)` — lookup + трансляция, **без
    записи в Anki**: TUI показывает карточку и просит подтверждение.
  - `AddNote(ctx, deck, note) (uint64, error)` — запись уже подтверждённой карточки.
  - `AddWord(ctx, deck, lemma) (uint64, error)` — `PreviewWord` + `AddNote` одним
    шагом (для неинтерактивных вызовов).
  - `ScanDeck`, `Apply`, `Decks`.

Трансляция `lexicon.Word → flashcard.Note` (anti-corruption между поддоменами):

- Back строится единообразно: **русский перевод** (`strings.Join(Russian, ", ")`)
  → формы слова → немецкое значение под разделителем `──` (если определения есть
  и выше уже что-то есть). Разделы с пустыми данными пропускаются.
- Noun → Front `<артикль> <Lemma>`; формы: `Plural: die <Plural>` (плейсхолдер
  `—`/`-` отбрасывается через `normalizePlural`).
- Verb → Front `<Lemma>` (+ `sich`); формы: `Partizip II: <…> · Hilfsverb: <…>`,
  `Präteritum: <…>`.
- Adjective → формы: степени сравнения.
- Other → только перевод + определения.
- Теги `["german", "auto"]`.

Сценарии (`service.go`):

- `PreviewWord(ctx, lemma)` → извлечь lemma (убрать артикль/`sich`, как в
  ScanDeck, т.к. страницы Wiktionary озаглавлены по голой лемме) →
  `Dictionary.Lookup` → трансляция (артикль восстанавливается по роду) →
  вернуть `flashcard.Note` **без записи**.
- `AddNote(ctx, deck, note)` → `Cards.Add`.
- `AddWord(ctx, deck, lemma)` → `PreviewWord` + `AddNote` одним шагом.
- `ScanDeck(ctx, deck) ([]Suggestion, error)`:
  1. `Cards.FindIncomplete(deck)`.
  2. Из Front извлечь lemma (убрать артикль/`sich`), `Dictionary.Lookup`.
  3. Собрать недостающие поля через ту же трансляцию →
     `Suggestion{ NoteID, Lemma, Fields }`.
  4. Не найдено / неоднозначно → `Suggestion{ Skipped: true, Reason }`.
- `Apply(ctx, sel []Suggestion) (updated, skipped int, err error)` →
  `Cards.Update` по выбранным.
- `Decks(ctx)` → проброс `Cards.Decks` (чтобы TUI не зависел от порта напрямую).

Тест-даблы — подпакет (`deckbuilder/usecase/usecasetest`): программируемые фейки
`Dictionary`, `Cards` и `Service`, переиспользуемые тестами ядра (D) и TUI (E),
чтобы моки не расходились. Трансляция, извлечение lemma и сценарии — под
table-тестами на этих фейках.

## 5. TUI на Bubble Tea (`internal/tui`)

Зависимости в `go.mod` (сейчас их нет; Go 1.25 совместим):

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles` (`textinput`, `list`, `spinner`)
- `github.com/charmbracelet/lipgloss`

Файлы пакета `internal/tui`:

- `model.go` — `tea.Model`: enum `state`, компоненты, `Init/Update/View`.
- `commands.go` — `tea.Cmd`-обёртки над сценариями `deckbuilder/usecase`: `AddWord`,
  `ScanDeck`, `Apply`, `Decks`. TUI знает только про `deckbuilder/usecase`, не про порты
  и адаптеры.
- `styles.go` — цветные стили lipgloss, собранные в одном месте (`View` берёт стили
  отсюда, цвета не хардкодятся по месту):
  - семантические стили статусов: `ok` (зелёный), `err` (красный),
    `warn`/`skipped` (жёлтый);
  - цвет рода для существительных: der/die/das — три разных
    `lipgloss.AdaptiveColor`;
  - акценты: заголовок, выбранный пункт `list`, активный `textinput`, подсказки
    горячих клавиш.
  - Цвета — `lipgloss.AdaptiveColor` (тёмная/светлая тема). lipgloss сам уважает
    `NO_COLOR` и не-TTY → монохромная деградация без доп. кода.

Состояния и переходы (`state`):

1. `stateDeckPick` — список из `DeckNames`; выбор → `stateModePick`.
   Пропускается, если deck задан флагом.
2. `stateModePick` — выбор режима: добавление слова → `stateAddInput`;
   сканирование → `stateScanLoading`.

Режим добавления слова (предпросмотр обязателен — карточка не пишется в Anki, пока
пользователь не подтвердил):

3. `stateAddInput` — `textinput`; Enter → preview-cmd (`PreviewWord`) →
   `stateAddLoading`.
4. `stateAddLoading` — `spinner` («Looking up»); по `previewDoneMsg{Note}` →
   `stateAddPreview`, по `lookupErrMsg` → `stateAddResult` с ошибкой.
5. `stateAddPreview` — рендер будущей карточки (Front/Back/tags); `Enter`/`y` →
   подтвердить → add-cmd (`AddNote`) → `stateAddSaving`; `Esc`/`n` → отмена →
   `stateAddInput`.
6. `stateAddSaving` — `spinner` («Adding»); по `addDoneMsg`/`addErrMsg` →
   `stateAddResult`.
7. `stateAddResult` — итог (id заметки или ошибка); любая клавиша → `stateAddInput`.

Режим сканирования:

8. `stateScanLoading` — `spinner`; на входе запускает scan-cmd
   (`deckbuilder.ScanDeck`); по `scanDoneMsg` → `stateScanList`.
9. `stateScanList` — `list` с чекбоксами (предложенные дополнения);
   `space` — отметить/снять, `a` — применить выбранные (apply-cmd) →
   `stateApplyLoading`, `Esc` → `stateModePick`.
10. `stateApplyLoading` — `spinner`; по `applyDoneMsg` → `stateScanSummary`.
11. `stateScanSummary` — итог (обновлено/пропущено) → `stateModePick`.

Сообщения: `decksMsg`, `previewDoneMsg{flashcard.Note}`, `lookupErrMsg{error}`,
`addDoneMsg{uint64}`, `addErrMsg{error}`, `scanDoneMsg{[]Suggestion}`,
`scanErrMsg{error}`, `applyDoneMsg{updated, skipped int}`, `applyErrMsg{error}`.

**Правило:** все HTTP-вызовы — только внутри `tea.Cmd`, возвращающих `tea.Msg`.
В `Update` никаких синхронных сетевых вызовов, иначе UI зависает.

## 6. Точка входа (`cmd/anki/main.go`)

Composition root — единственное место, где сходятся адаптеры:

- Флаги (с дефолтами под сетап пользователя, чтобы TUI запускался без флагов):
  `-anki` (`http://localhost:8765`), `-deck` (`DEUTSCH`), `-model` (`Простая`),
  `-dict` (`data/de-compact.jsonl`). Если файла дампа нет — мягкий фолбэк на live
  Wiktionary API (не ошибка), с сообщением в stderr.
- Создать `slog.Logger`, `ankiconnect` клиент → `flashcard/anki` адаптер (`Cards`),
  `lexicon/wiktionary` адаптер (`Dictionary`), затем `deckbuilder/usecase` сервис с этими
  портами.
- Собрать `tui` model с инъекцией сервиса `deckbuilder/usecase`.
- `tea.NewProgram(model).Run()`.

## 7. Тесты

Unit-тесты лежат рядом с кодом в каждом пакете (`testdata/` для фикстур):

- `internal/lexicon/wiktextract` — `ParseEntry` на реальных JSON-фикстурах kaikki
  (существительное с/без Plural, глагол, прилагательное), `Client.Open`+`Lookup`
  на компактном дампе.
- `internal/lexicon/wiktionary` — table-тесты парсера на фикстурах wikitext
  (существительное / глагол / прилагательное / не найдено).
- `internal/flashcard` — инвариант `IsComplete` по частям речи.
- `internal/flashcard/anki` — `httptest.Server` вместо AnkiConnect: проверка
  трансляции и формы конверта.
- `pkg/ankiconnect` — `httptest.Server`: конверт и проброс ошибок из тела.
- `internal/deckbuilder/usecase` — трансляция `Word → Note`, `ScanDeck`/`Apply`,
  извлечение lemma (на фейках `usecasetest`).
- `internal/tui` — `Update(msg) -> model` чистый: подаём сообщения, проверяем
  переходы (на фейке `Service`); терминал не нужен.

Contract-тесты — compile-time-проверки структурного соответствия адаптеров портам:
`var _ usecase.Dictionary = (*wiktionary.Client)(nil)`,
`var _ usecase.Cards = (*anki.Adapter)(nil)`, `var _ usecase.Service = (*service)(nil)`.

Интеграционные тесты (тег `//go:build integration`) — реальные пакеты, внешний HTTP
на `httptest`: `usecase` + `flashcard/anki` (AnkiConnect-двойник) и `usecase` +
`lexicon/wiktionary` (ответ MediaWiki) сквозь трансляцию, без моков портов.

Прогон: `go test ./... -race`, `go test -tags integration ./...`, `go vet ./...`.
Подробное разбиение тестов по агентам — в [subagent-plan.md](./subagent-plan.md).

## Порядок реализации

1. Создать каркас layout (`cmd` / `internal` / `pkg`); перенести существующий код в
   `pkg/ankiconnect`.
2. Починить `getRequest` (разблокирует `go build`).
3. Поддомен `lexicon`: модель (корень) + адаптер `wiktionary` с тестами.
4. Дополнить `pkg/ankiconnect` действиями (`AddNote`, `DeckNames`, `FindNotes`,
   `NotesInfo`, `UpdateNoteFields`).
5. Поддомен `flashcard`: модель (корень) + адаптер `anki` с тестами.
6. Ядро `deckbuilder`: модель `Suggestion` (корень) + `usecase` (порты, трансляция,
   сценарии `AddWord`/`ScanDeck`/`Apply`/`Decks`) + тесты.
7. TUI (Bubble Tea) — оба режима (`internal/tui`).
8. Связка в `cmd/anki/main.go`.

## Открытые вопросы

- Источник данных: Wiktionary (без ключа, парсинг wikitext) или платный API (ключ).
- Выбор deck: экран-список через `DeckNames` или только флаг `-deck`.
