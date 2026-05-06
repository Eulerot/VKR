#pragma once

#include <QString>
#include <QStringList>
#include <QList>

enum class FieldKind {
    Text,
    Int,
    Double,
    Date,
    Enum,
    Multiline
};

struct FieldDef {
    QString key;
    QString label;
    FieldKind kind = FieldKind::Text;
    bool required = false;

    QStringList options;
    QStringList optionValues;

    QString placeholder;
    bool visibleInTable = true;
    bool visibleInEditor = true;
    bool editable = true;
};

struct TableDef {
    QString title;
    QString listAction;
    QString upsertAction;
    QString deleteAction;
    QStringList keyFields;
    QList<FieldDef> fields;
    bool readOnly = false;
};

enum class ControlMode {
    None,
    Year,
    Month
};

struct WorkspaceConfig {
    QString taskCode;
    QString title;
    QString solveAction;
    QString listAction;
    ControlMode controlMode = ControlMode::None;
    TableDef outputTable;
    QList<TableDef> inputTables;
    QList<TableDef> referenceTables;
};

inline FieldDef f(const QString& key,
                  const QString& label,
                  FieldKind kind = FieldKind::Text,
                  bool required = false,
                  const QStringList& options = {},
                  const QString& placeholder = {},
                  bool visibleInTable = true,
                  bool visibleInEditor = true,
                  bool editable = true,
                  const QStringList& optionValues = {})
{
    FieldDef x;
    x.key = key;
    x.label = label;
    x.kind = kind;
    x.required = required;
    x.options = options;
    x.optionValues = optionValues;
    x.placeholder = placeholder;
    x.visibleInTable = visibleInTable;
    x.visibleInEditor = visibleInEditor;
    x.editable = editable;
    return x;
}

inline TableDef tableDef(const QString& title,
                         const QString& listAction,
                         const QString& upsertAction,
                         const QString& deleteAction,
                         const QStringList& keyFields,
                         const QList<FieldDef>& fields,
                         bool readOnly = false)
{
    TableDef t;
    t.title = title;
    t.listAction = listAction;
    t.upsertAction = upsertAction;
    t.deleteAction = deleteAction;
    t.keyFields = keyFields;
    t.fields = fields;
    t.readOnly = readOnly;
    return t;
}

inline TableDef machineRefTable()
{
    return tableDef(
        "Справочник наличия машин и механизмов",
        "machines.list",
        "machines.upsert",
        "",
        {"machine_id"},
        {
            f("machine_id", "Код классификатора машины", FieldKind::Text, true),
            f("model", "Марка и модель машины", FieldKind::Text, true),
            f("plate_number", "Гос. номер", FieldKind::Text, false),
            f("serial_number", "Заводской номер", FieldKind::Text, false),
            f("commission_year", "Год ввода в эксплуатацию", FieldKind::Int, false),
            f("notes", "Примечания", FieldKind::Multiline, false)
        }
        );
}

inline WorkspaceConfig task61Config()
{
    WorkspaceConfig c;
    c.taskCode = "6.1";
    c.title = "6.1 — Актуальный реестр состояния машин и механизмов";
    c.solveAction = "registry.get";
    c.listAction = "registry.get";
    c.controlMode = ControlMode::None;

    c.outputTable = tableDef(
        "АКТУАЛЬНЫЙ РЕЕСТР МАШИН И МЕХАНИЗМОВ",
        "registry.get",
        "",
        "",
        {},
        {
            f("machine_id", "Код классификатора машины"),
            f("plate_number", "Гос. номер"),
            f("serial_number", "Заводской номер"),
            f("model", "Марка и модель машины"),
            f("technical_state", "Техническое состояние"),
            f("operation_status", "Статус эксплуатации"),
            f("hours", "Наработка, моточасы"),
            f("location", "Местонахождение (объект)")
        },
        true
        );

    c.inputTables = {
        tableDef(
            "Путевые листы",
            "machine_events.list",
            "machine_events.upsert",
            "machine_events.delete",
            {"event_id"},
            {
                f("event_id", "№ п/п", FieldKind::Int, false, {}, "", false, false, false),
                f("event_date", "Дата", FieldKind::Date, true),
                f("machine_id", "Код классификатора машины", FieldKind::Text, true),
                f("driver_name", "Водитель", FieldKind::Text, false),
                f("work_object", "Объект работ", FieldKind::Text, false),
                f("start_hours", "Начальное количество моточасов", FieldKind::Int, false),
                f("end_hours", "Конечное количество моточасов", FieldKind::Int, false),
                f("operation_status", "Статус эксплуатации", FieldKind::Text, false),
                f("location", "Местоположение", FieldKind::Text, false),
                f("technical_notes", "Техническое состояние", FieldKind::Enum, true, {"исправна", "неисправна"})
            }
            ),
        tableDef(
            "Акты ремонта",
            "repair_acts.list",
            "repair_acts.upsert",
            "repair_acts.delete",
            {"repair_act_id"},
            {
                f("repair_act_id", "№ п/п", FieldKind::Int, false, {}, "", false, false, false),
                f("machine_id", "Код классификатора машины", FieldKind::Text, true),
                f("repair_type", "Вид ремонта", FieldKind::Text, true),
                f("start_date", "Дата начала ремонта", FieldKind::Date, false),
                f("end_date", "Дата окончания ремонта", FieldKind::Date, true),
                f("hours_before", "Наработка до ремонта", FieldKind::Int, false),
                f("hours_after", "Наработка после ремонта", FieldKind::Int, false),
                f("conclusion", "Заключение о техническом состоянии", FieldKind::Enum, true, {"исправна", "неисправна"})
            }
            )
    };

    c.referenceTables = {
        machineRefTable()
};

return c;
}

inline WorkspaceConfig task62Config()
{
    WorkspaceConfig c;
    c.taskCode = "6.2";
    c.title = "6.2 — Календарное планирование ремонтов";
    c.solveAction = "annual_plan.solve";
    c.listAction = "annual_plan.list";
    c.controlMode = ControlMode::None;

    c.outputTable = tableDef(
        "ПЛАН РЕМОНТОВ НА ГОД",
        "annual_plan.list",
        "",
        "",
        {},
        {
            f("request_id", "Номер заявки"),
            f("machine_id", "Код классификатора машины"),
            f("model", "Модель машины"),
            f("repair_type", "Тип ремонта"),
            f("required_qualification", "Требуемая квалификация", FieldKind::Int),
            f("labor_hours", "Трудоемкость (ч)", FieldKind::Int),
            f("priority_weight", "Приоритет (вес)", FieldKind::Int),
            f("forecast_cost", "Прогнозная стоимость", FieldKind::Double),
            f("assigned_month", "Назначенный месяц", FieldKind::Int)
        },
        true
        );

    c.inputTables = {
        tableDef(
            "Реестр заявок на ремонт",
            "repair_requests.list",
            "repair_requests.upsert",
            "repair_requests.delete",
            {"request_id"},
            {
                f("request_id", "Номер заявки", FieldKind::Text, true),
                f("request_status", "Статус заявки", FieldKind::Enum, true, {"новая", "в работе", "согласована", "отклонена", "выполнена"}),
                f("machine_id", "Код классификатора машины", FieldKind::Text, true),
                f("model", "Модель машины", FieldKind::Text, false, {}, "", true, true, false),
                f("priority_weight", "Числовой вес заявки", FieldKind::Int, true),
                f("motohours_at_request", "Наработка на момент заявки, моточасы", FieldKind::Int, false),
                f("forecast_cost", "Прогнозная стоимость (руб.)", FieldKind::Double, false),
                f("repair_type", "Тип ремонта", FieldKind::Text, true),
                f("critical_parts_required", "Требование критических запчастей (да/нет)", FieldKind::Enum, true, {"да", "нет"}),
                f("required_qualification", "Требуемая квалификация", FieldKind::Int, true),
                f("desired_month", "Желаемый месяц", FieldKind::Int, false),
                f("notes", "Примечания", FieldKind::Multiline, false)
            }
            ),
        tableDef(
            "Технологическая карта осуществления ремонтных операций",
            "repair_tech_cards.list",
            "repair_tech_cards.upsert",
            "repair_tech_cards.delete",
            {"repair_type", "machine_id"},
            {
                f("techcard_id", "№ п/п", FieldKind::Int, false, {}, "", false, false, false),
                f("repair_type", "Тип ремонта", FieldKind::Text, true),
                f("machine_id", "Код классификатора машины", FieldKind::Text, true),
                f("model", "Модель машины", FieldKind::Text, false, {}, "", true, true, false),
                f("labor_hours", "Трудоемкость (ч)", FieldKind::Int, true),
                f("required_qualification", "Требуемая квалификация", FieldKind::Int, true),
                f("operations_description", "Описание операций", FieldKind::Multiline, false),
                f("notes", "Примечания", FieldKind::Multiline, false)
            }
            )
    };

    c.referenceTables = {
        machineRefTable()
};

return c;
}

inline WorkspaceConfig task63Config()
{
    WorkspaceConfig c;
    c.taskCode = "6.3";
    c.title = "6.3 — Ведомость потребности материалов";
    c.solveAction = "materials.solve";
    c.listAction = "material_demand.list";
    c.controlMode = ControlMode::Month;

    c.outputTable = tableDef(
        "ВЕДОМОСТЬ ПОТРЕБНОСТИ МАТЕРИАЛОВ",
        "material_demand.list",
        "",
        "",
        {},
        {
            f("material_code", "Код материала"),
            f("material_name", "Наименование материала"),
            f("unit", "Единица измерения"),
            f("demand_quantity", "Потребность в месяце (шт./кг/м)", FieldKind::Double),
            f("notes", "Примечания", FieldKind::Multiline)
        },
        true
        );

    c.inputTables = {
        tableDef(
            "Нормативная карта ремонта",
            "material_norms.list",
            "material_norms.upsert",
            "material_norms.delete",
            {"repair_type", "model", "material_code"},
            {
                f("norm_id", "№ п/п", FieldKind::Int, false, {}, "", false, false, false),
                f("repair_type", "Тип ремонта", FieldKind::Text, true),
                f("model", "Модель машины", FieldKind::Text, true, {}, "", true, true, false),
                f("material_code", "Код материала", FieldKind::Text, true),
                f("material_name", "Наименование материала", FieldKind::Text, false, {}, "", true, true, false),
                f("unit_symbol", "Единица измерения", FieldKind::Text, false, {}, "", true, true, false),
                f("consumption_per_repair", "Норма расходования на один ремонт", FieldKind::Double, true)
            }
            ),
        tableDef(
            "Годовой план ремонтов",
            "repair_plan.list",
            "",
            "",
            {"request_id"},
            {
                f("plan_id", "№ п/п", FieldKind::Int, false, {}, "", false, false, false),
                f("request_id", "Номер заявки", FieldKind::Text, true),
                f("machine_id", "Код классификатора машины", FieldKind::Text, true),
                f("model", "Модель машины", FieldKind::Text, true, {}, "", true, true, false),
                f("repair_type", "Тип ремонта", FieldKind::Text, true),
                f("required_qualification", "Требуемая квалификация", FieldKind::Int, true),
                f("labor_hours", "Трудоемкость (ч)", FieldKind::Int, true),
                f("priority_weight", "Приоритет (вес)", FieldKind::Int, true),
                f("forecast_cost", "Прогнозная стоимость", FieldKind::Double, false),
                f("assigned_month", "Назначенный месяц", FieldKind::Int, false)
            },
            true
            )
    };

    c.referenceTables = {
        tableDef(
            "Справочник материалов и запасных частей",
            "materials.list",
            "materials.upsert",
            "materials.delete",
            {"material_code"},
            {
                f("material_code", "Код материала", FieldKind::Text, true),
                f("material_name", "Наименование материала", FieldKind::Text, true),
                f("unit_symbol", "Единица измерения", FieldKind::Text, true),
                f("unit_id", "Unit ID", FieldKind::Int, false, {}, "", false, false, false)
            }
            ),
        tableDef(
            "Справочник единиц измерения",
            "units.list",
            "units.upsert",
            "",
            {"unit_id"},
            {
                f("unit_id", "Unit ID", FieldKind::Int, true),
                f("unit_symbol", "Символ", FieldKind::Text, true),
                f("unit_name", "Наименование", FieldKind::Text, true)
            }
            ),
        machineRefTable()
    };

    return c;
}

inline WorkspaceConfig task64Config()
{
    WorkspaceConfig c;
    c.taskCode = "6.4";
    c.title = "6.4 — Назначение исполнителей бригад на ремонтные работы";
    c.solveAction = "brigade_assignments.solve";
    c.listAction = "repair_assignments.list";
    c.controlMode = ControlMode::Month;

    c.outputTable = tableDef(
        "РАСПРЕДЕЛЕНИЕ БРИГАД НА РЕМОНТЫ",
        "repair_assignments.list",
        "",
        "",
        {},
        {
            f("request_id", "Номер заявки"),
            f("machine_id", "Код классификатора машины"),
            f("model", "Модель машины"),
            f("repair_type", "Тип ремонта"),
            f("start_date", "Назначенная дата начала", FieldKind::Date),
            f("end_date", "Назначенная дата окончания", FieldKind::Date),
            f("brigade_number", "Номер бригады"),
            f("specialization", "Специализация бригады"),
            f("planned_hours", "Плановые часы", FieldKind::Int),
            f("responsible_person", "Ответственный (Ф.И.О.)"),
            f("assignment_status", "Статус"),
            f("notes", "Примечания", FieldKind::Multiline)
        },
        true
        );

    c.inputTables = {
        tableDef(
            "Месячный план ремонтов",
            "monthly_repair_plan.list",
            "monthly_repair_plan.upsert",
            "monthly_repair_plan.delete",
            {"request_id"},
            {
                f("monthly_plan_id", "№ п/п", FieldKind::Int, false, {}, "", false, false, false),
                f("request_id", "Номер заявки", FieldKind::Text, true),
                f("machine_id", "Код классификатора машины", FieldKind::Text, true),
                f("model", "Модель машины", FieldKind::Text, true, {}, "", true, true, false),
                f("repair_type", "Тип ремонта", FieldKind::Text, true),
                f("required_specialization", "Требуемая специализация", FieldKind::Enum, true, {"слесарь", "электрик", "сварщик", "универсальная"}),
                f("required_qualification", "Требуемая квалификация", FieldKind::Int, true),
                f("planned_start_date", "Плановая дата начала", FieldKind::Date, false),
                f("planned_end_date", "Плановая дата окончания", FieldKind::Date, false),
                f("labor_hours", "Трудоемкость", FieldKind::Int, true),
                f("priority_weight", "Приоритет (вес)", FieldKind::Int, true),
                f("readiness_status", "Статус готовности к назначению", FieldKind::Enum, true, {"готова", "не готова"}),
                f("notes", "Примечание", FieldKind::Multiline, false)
            }
            ),
        tableDef(
            "График занятости бригад",
            "brigade_availability.list",
            "brigade_availability.upsert",
            "brigade_availability.delete",
            {"availability_id"},
            {
                f("availability_id", "№ п/п", FieldKind::Int, false, {}, "", false, false, false),
                f("brigade_number", "Номер бригады", FieldKind::Text, true),
                f("available_start", "Дата начала свободного времени", FieldKind::Date, true),
                f("available_end", "Дата окончания свободного времени", FieldKind::Date, true),
                f("available_hours", "Доступные часы в периоде", FieldKind::Int, false),
                f("current_assigned_hours", "Текущие назначения (часы)", FieldKind::Int, false),
                f("contact", "Контакт", FieldKind::Text, false),
                f("notes", "Примечания", FieldKind::Multiline, false)
            }
            )
    };

    c.referenceTables = {
        tableDef(
            "Справочник бригад",
            "brigades.list",
            "brigades.upsert",
            "brigades.delete",
            {"brigade_number"},
            {
                f("brigade_number", "Номер бригады", FieldKind::Text, true),
                f("team_composition", "Состав бригады (ФИО)", FieldKind::Multiline, true),
                f("specialization", "Специализация (слесарь/электрик/сварщик и др.)", FieldKind::Enum, true, {"слесарь", "электрик", "сварщик", "универсальная"}),
                f("qualification", "Квалификация", FieldKind::Int, true),
                f("contact", "Контакт", FieldKind::Text, false),
                f("notes", "Примечания", FieldKind::Multiline, false)
            }
            ),
        machineRefTable()
    };

    return c;
}

inline WorkspaceConfig workspaceForTask(const QString& taskCode)
{
    if (taskCode == "6.1") return task61Config();
    if (taskCode == "6.2") return task62Config();
    if (taskCode == "6.3") return task63Config();
    if (taskCode == "6.4") return task64Config();
    return task61Config();
}
