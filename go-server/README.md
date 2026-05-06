# Repair Planner TCP Server (Go)

Простой TCP-сервер на Go для дипломного проекта.

## Что умеет
- принимает JSON-запросы по TCP;
- работает с PostgreSQL;
- добавляет/обновляет записи справочников и документов;
- формирует реестр состояния машин;
- рассчитывает:
  - **6.2 годовой план ремонтов**;
  - **6.4 распределение бригад на ремонты**;
- рассчитывает ведомость материалов по месяцу.

## Протокол
Каждый запрос — одна JSON-строка:

```json
{"action":"registry.get","payload":{}}
```

Ответ:

```json
{"ok":true,"data":...}
```

## Переменные окружения
- `DB_HOST`
- `DB_PORT`
- `DB_USER`
- `DB_PASSWORD`
- `DB_NAME`
- `TCP_PORT`

## Важно
В `docker-compose.yml` у вас PostgreSQL, а схема БД в исходном виде написана в стиле MySQL/InnoDB.
Для запуска на PostgreSQL схему нужно привести к PostgreSQL-совместимому виду.
В коде сервера используются имена таблиц в нижнем регистре, как это обычно работает в PostgreSQL при создании некавыченых идентификаторов.

## Примеры действий
- `ping`
- `machines.upsert`
- `events.add`
- `acts.add`
- `registry.get`
- `repair_requests.upsert`
- `repair_tech_cards.upsert`
- `monthly_resources.upsert`
- `annual_plan.solve`
- `annual_plan.list`
- `materials.upsert`
- `material_norms.upsert`
- `materials.solve`
- `brigades.upsert`
- `brigade_availability.upsert`
- `monthly_repair_plan.upsert`
- `brigade_assignments.solve`
- `brigade_assignments.list`

## Запуск
1. Соберите контейнер:
   `docker compose build go-server`
2. Поднимите:
   `docker compose up -d`

## Замечание по LP
Для задач назначения в дипломе здесь используется простая точная схема ветвления и границ для 0-1 модели.
Это компактно, прозрачно и хорошо объясняется в пояснительной записке.
