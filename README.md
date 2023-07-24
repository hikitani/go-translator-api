# HTTP API для перевода текстов

Позволяет получать переведенные тексты с английского на русский. Под капотом используется Яндекс переводчик.

## Инсталяция

1. Качаем [последнюю](https://github.com/RUScape/go-translator-api/releases) версию.
1. Распаковываем zip архив
1. Запускаем exe файл в командной строке (также можем указать свой port прослушивания через аргумент `-port`, по умолчанию используется `8080`)

## Пример получения перевода

```bash
$ curl -X POST -d '{"text": "Hello, World!"}' http://localhost:8081/translate
{"translated":"Привет, Мир!"}
```
