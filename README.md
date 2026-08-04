# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки

Запуск бенчмарков ключевых компонентов:

```bash
go test -bench=. -benchmem ./...
```

Покрыты: сокращение/чтение URL (`handler`), подпись куки (`auth`), gzip middleware, file/memory repository, audit.

## Профилирование памяти

Профили сохранены в `profiles/`:
- `profiles/base.pprof` — до оптимизаций
- `profiles/result.pprof` — после оптимизаций

Снимались через `/debug/pprof/allocs` под нагрузкой (~650 запросов shorten/follow с gzip и file storage + audit).

### Что оптимизировали

1. **gzip middleware** — `sync.Pool` для `gzip.Writer` / `gzip.Reader` вместо создания на каждый запрос.
2. **file repository** — убран `SetIndent` при сериализации JSON (убирает `appendIndent` / лишние аллокации).
3. **audit FileObserver** — файл держится открытым, буфер переиспользуется.

### Diff профилей

```bash
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

```
File: shortener-prof
Type: alloc_space
Time: 2026-08-04 21:57:12 MSK
Showing nodes accounting for -400.62MB, 89.67% of 446.78MB total
Dropped 75 nodes (cum <= 2.23MB)
      flat  flat%   sum%        cum   cum%
 -178.93MB 40.05% 40.05%  -221.60MB 49.60%  compress/flate.NewWriter (inline)
 -126.12MB 28.23% 68.28%  -145.77MB 32.63%  encoding/json.appendIndent
  -41.16MB  9.21% 77.49%   -41.16MB  9.21%  compress/flate.(*compressor).initDeflate (inline)
  -27.61MB  6.18% 83.67%   -27.61MB  6.18%  bytes.growSlice
  -19.66MB  4.40% 88.07%   -19.66MB  4.40%  encoding/json.appendNewline (inline)
   -5.01MB  1.12% 89.19%    -5.01MB  1.12%  sync.(*Pool).pinSlow
   -2.13MB  0.48% 89.67%  -175.52MB 39.29%  github.com/tkalexx/shorturl.git/internal/repository.(*FileRepository).save
    0.50MB  0.11% 89.56%  -402.12MB 90.00%  github.com/tkalexx/shorturl.git/internal/logger.LoggingMiddleware.func1
   -0.50MB  0.11% 89.67%    -2.50MB  0.56%  net/http.readRequest
   -0.50MB  0.11% 89.78%    -3.50MB  0.78%  net/http.(*conn).readRequest
    0.50MB  0.11% 89.67%  -175.02MB 39.17%  github.com/tkalexx/shorturl.git/internal/repository.(*FileRepository).Save
         0     0% 89.67%   -21.21MB  4.75%  bytes.(*Buffer).Write
         0     0% 89.67%    -6.40MB  1.43%  bytes.(*Buffer).WriteString
         0     0% 89.67%   -27.61MB  6.18%  bytes.(*Buffer).grow
         0     0% 89.67%   -42.67MB  9.55%  compress/flate.(*compressor).init
         0     0% 89.67%  -221.60MB 49.60%  compress/gzip.(*Writer).Write
         0     0% 89.67%  -395.98MB 88.63%  encoding/json.(*Encoder).Encode
         0     0% 89.67%   -28.11MB  6.29%  encoding/json.(*encodeState).marshal
         0     0% 89.67%   -28.11MB  6.29%  encoding/json.(*encodeState).reflectValue
         0     0% 89.67%   -28.11MB  6.29%  encoding/json.arrayEncoder.encode
         0     0% 89.67%   -28.11MB  6.29%  encoding/json.sliceEncoder.encode
         0     0% 89.67%   -21.71MB  4.86%  encoding/json.stringEncoder
         0     0% 89.67%   -28.11MB  6.29%  encoding/json.structEncoder.encode
         0     0% 89.67%  -399.12MB 89.33%  github.com/go-chi/chi/v5.(*ChainHandler).ServeHTTP
         0     0% 89.67%  -403.12MB 90.23%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 89.67%  -399.12MB 89.33%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 89.67%  -399.12MB 89.33%  github.com/tkalexx/shorturl.git/internal/auth.(*Manager).Middleware-fm.(*Manager).Middleware.func1
         0     0% 89.67%  -221.60MB 49.60%  github.com/tkalexx/shorturl.git/internal/gzip.(*compressWriter).Write
         0     0% 89.67%  -400.12MB 89.56%  github.com/tkalexx/shorturl.git/internal/gzip.Middleware.func1
         0     0% 89.67%  -301.75MB 67.54%  github.com/tkalexx/shorturl.git/internal/handler.(*Handler).shortenJSON
         0     0% 89.67%   -96.86MB 21.68%  github.com/tkalexx/shorturl.git/internal/handler.(*Handler).shortener
         0     0% 89.67%  -175.02MB 39.17%  github.com/tkalexx/shorturl.git/internal/handler.(*Service).Shorten
```

Отрицательные значения показывают снижение аллокаций (~400 MB на том же сценарии нагрузки).
