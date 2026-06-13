# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
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

## Локальные TLS-сертификаты

Инструкция по генерации локального сертификата сервера и приватного ключа находится в [certs/tls-certificates.md](./certs/tls-certificates.md).

## Профилирование памяти (pprof)

В файле [profiles/profiling.md](./profiles/profiling.md) можно найти подробное описание процесса профилирования памяти.

### Текстовый анализ результатов оптимизации

В процессе оптимизации был выявлен основной источник аллокаций в ответах сервера - gzip-сжатие.
В базовом heap-профиле по `alloc_space` подавляющая часть аллоцированной памяти приходилась на `compress/flate.NewWriter`
(и связанные с ним `compress/flate.(*compressor).initDeflate`), что указывало на создание нового `gzip.Writer` на каждый запрос.

Что было сделано:

- Добавлено переиспользование `gzip.Writer` через `sync.Pool` (writer сбрасывается через `Reset(w)` перед использованием и `Reset(nil)` перед возвратом в пул).

Итог:

- По `alloc_space` суммарные аллокации на той же нагрузке снизились примерно с `~156 GB` до `~4.45 GB`.
- В diff (`-diff_base`) основной вклад в снижение дают `compress/flate.NewWriter` и `compress/flate.(*compressor).initDeflate`.

### Результат оптимизации (diff)

```
go tool pprof -top -sample_index=alloc_space -diff_base=profiles/base.pprof profiles/result.pprof
```

```
File: server
Build ID: 462461e32bbaff4f3602fa7dea2c94e4c01237f8
Type: alloc_space
Time: 2026-03-25 13:23:06 +07
Showing nodes accounting for -149.40GB, 97.97% of 152.49GB total
Dropped 142 nodes (cum <= 0.76GB)
      flat  flat%   sum%        cum   cum%
 -123.02GB 80.67% 80.67%  -149.62GB 98.11%  compress/flate.NewWriter (inline)
  -25.86GB 16.96% 97.63%   -25.86GB 16.96%  compress/flate.(*compressor).initDeflate (inline)
   -0.76GB   0.5% 98.13%    -0.76GB   0.5%  io.init.func1
    0.20GB  0.13% 98.00%     0.83GB  0.54%  net/http.(*conn).readRequest
    0.01GB 0.009% 97.99%     1.40GB  0.92%  zerogravity-82/metrics/internal/httpserver/handler.MetricRouter.updatesHandler.func8
    0.01GB 0.009% 97.98%  -148.09GB 97.11%  net/http.(*conn).serve
    0.01GB 0.0077% 97.97%  -148.28GB 97.23%  zerogravity-82/metrics/internal/httpserver/handler.MetricRouter.withLogging.func1.1
         0     0% 97.97%    -0.83GB  0.54%  bufio.(*Writer).Flush
         0     0% 97.97%   -26.59GB 17.44%  compress/flate.(*compressor).init
         0     0% 97.97%  -149.62GB 98.11%  compress/gzip.(*Writer).Close
         0     0% 97.97%  -149.62GB 98.11%  compress/gzip.(*Writer).Write
         0     0% 97.97%     1.40GB  0.92%  github.com/go-chi/chi/v5.(*ChainHandler).ServeHTTP
         0     0% 97.97%  -148.08GB 97.11%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 97.97%     1.40GB  0.92%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 97.97%     1.40GB  0.92%  github.com/go-chi/chi/v5/middleware.AllowContentType.func1.1
         0     0% 97.97%  -148.28GB 97.23%  github.com/go-chi/chi/v5/middleware.RealIP.func1
         0     0% 97.97%  -148.28GB 97.23%  github.com/go-chi/chi/v5/middleware.StripSlashes.func1
         0     0% 97.97%    -0.80GB  0.52%  io.Copy (inline)
         0     0% 97.97%    -0.78GB  0.51%  io.CopyN
         0     0% 97.97%    -0.80GB  0.52%  io.copyBuffer
         0     0% 97.97%    -0.80GB  0.52%  io.discard.ReadFrom
         0     0% 97.97%    -0.83GB  0.54%  net/http.(*chunkWriter).Write
         0     0% 97.97%    -0.83GB  0.54%  net/http.(*chunkWriter).writeHeader
         0     0% 97.97%    -0.85GB  0.56%  net/http.(*response).finishRequest
         0     0% 97.97%  -148.28GB 97.23%  net/http.HandlerFunc.ServeHTTP
         0     0% 97.97%  -148.08GB 97.11%  net/http.serverHandler.ServeHTTP
         0     0% 97.97%       -1GB  0.66%  sync.(*Pool).Get
         0     0% 97.97%  -149.62GB 98.11%  zerogravity-82/metrics/internal/httpserver/handler.(*compressResponseWriter).Close
         0     0% 97.97%  -148.22GB 97.20%  zerogravity-82/metrics/internal/httpserver/handler.MetricRouter.withGzip.func2.1
```
