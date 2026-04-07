-- =====================================================
-- Схема БД "Учёт и планирование ремонтов"
-- Под задачи 4.1–4.4
-- =====================================================

BEGIN;

-- =====================================================
-- 1. Справочник машин и механизмов
-- =====================================================

CREATE TABLE IF NOT EXISTS machines (
    machine_id       SERIAL PRIMARY KEY,
    model            VARCHAR(100) NOT NULL,
    plate_number     VARCHAR(50) UNIQUE,
    serial_number    VARCHAR(100) UNIQUE,
    commission_year  INTEGER CHECK (commission_year >= 1900),
    notes            TEXT
);

-- =====================================================
-- 2. События по машинам
-- =====================================================

CREATE TABLE IF NOT EXISTS machine_events (
    event_id          SERIAL PRIMARY KEY,
    machine_id        INTEGER NOT NULL REFERENCES machines(machine_id) ON DELETE CASCADE,
    event_date        DATE NOT NULL,
    start_hours       NUMERIC(10,1) CHECK (start_hours >= 0),
    end_hours         NUMERIC(10,1) CHECK (end_hours >= 0),
    operation_status  VARCHAR(50),
    location          VARCHAR(200),
    technical_notes   TEXT,
    CHECK (
        start_hours IS NULL OR end_hours IS NULL OR end_hours >= start_hours
    )
);

-- =====================================================
-- 3. Акты выполненных ремонтов
-- =====================================================

CREATE TABLE IF NOT EXISTS repair_acts (
    act_id           SERIAL PRIMARY KEY,
    machine_id       INTEGER NOT NULL REFERENCES machines(machine_id) ON DELETE CASCADE,
    repair_type      VARCHAR(50) NOT NULL,
    start_date       DATE,
    end_date         DATE,
    hours_before     NUMERIC(10,1) CHECK (hours_before >= 0),
    hours_after      NUMERIC(10,1) CHECK (hours_after >= 0),
    status_after     VARCHAR(50),
    conclusion       TEXT,
    CHECK (
        start_date IS NULL OR end_date IS NULL OR end_date >= start_date
    ),
    CHECK (
        hours_before IS NULL OR hours_after IS NULL OR hours_after >= hours_before
    )
);

-- =====================================================
-- 4. Заявки на ремонт
-- =====================================================

CREATE TABLE IF NOT EXISTS repair_requests (
    request_id              SERIAL PRIMARY KEY,
    request_status          VARCHAR(50) NOT NULL DEFAULT 'новая',
    machine_id              INTEGER NOT NULL REFERENCES machines(machine_id) ON DELETE RESTRICT,
    priority_weight         INTEGER NOT NULL CHECK (priority_weight BETWEEN 1 AND 10),
    moto_hours_at_request   NUMERIC(10,1) NOT NULL CHECK (moto_hours_at_request >= 0),
    forecast_cost           NUMERIC(12,2) CHECK (forecast_cost >= 0),
    repair_type             VARCHAR(50) NOT NULL,
    critical_parts_required BOOLEAN NOT NULL DEFAULT FALSE,
    required_qualification  INTEGER CHECK (required_qualification >= 0),
    desired_month           DATE,
    notes                   TEXT,
    CHECK (
        desired_month IS NULL OR EXTRACT(DAY FROM desired_month) = 1
    ),
    CHECK (request_status IN (
        'новая',
        'готова к планированию',
        'готова к назначению',
        'назначена',
        'отклонена',
        'выполняется',
        'завершена'
    ))
);

-- =====================================================
-- 5. Технологические карты ремонтов
-- =====================================================

CREATE TABLE IF NOT EXISTS repair_tech_cards (
    tech_card_id            SERIAL PRIMARY KEY,
    repair_type             VARCHAR(50) NOT NULL,
    model                   VARCHAR(100) NOT NULL,
    labor_hours             NUMERIC(8,1) NOT NULL CHECK (labor_hours > 0),
    required_specialization  VARCHAR(100) NOT NULL,
    required_qualification  INTEGER NOT NULL CHECK (required_qualification >= 0),
    operations_description   TEXT,
    notes                   TEXT,
    UNIQUE (repair_type, model)
);

-- =====================================================
-- 6. Ресурсы по месяцам
-- Месяц хранится как первая дата месяца
-- =====================================================

CREATE TABLE IF NOT EXISTS monthly_resources (
    month                    DATE PRIMARY KEY,
    available_hours          NUMERIC(10,1) NOT NULL CHECK (available_hours >= 0),
    budget                   NUMERIC(12,2) NOT NULL CHECK (budget >= 0),
    max_units_in_repair      INTEGER NOT NULL CHECK (max_units_in_repair >= 0),
    critical_parts_available BOOLEAN NOT NULL DEFAULT FALSE,
    notes                    TEXT,
    CHECK (EXTRACT(DAY FROM month) = 1)
);

-- =====================================================
-- 7. План ремонтов
-- =====================================================

CREATE TABLE IF NOT EXISTS repair_plan (
    plan_id             SERIAL PRIMARY KEY,
    request_id          INTEGER NOT NULL UNIQUE REFERENCES repair_requests(request_id) ON DELETE CASCADE,
    assigned_month      DATE NOT NULL REFERENCES monthly_resources(month),
    planned_start_date  DATE,
    planned_end_date    DATE,
    parts_status        VARCHAR(50) NOT NULL DEFAULT 'не обеспечены',
    assignment_status   VARCHAR(50) NOT NULL DEFAULT 'черновик',
    rejection_reason    TEXT,
    CHECK (EXTRACT(DAY FROM assigned_month) = 1),
    CHECK (
        planned_start_date IS NULL OR planned_end_date IS NULL OR planned_end_date >= planned_start_date
    ),
    CHECK (assignment_status IN (
        'черновик',
        'готова к назначению',
        'назначена',
        'отклонена',
        'утверждена',
        'выполняется'
    ))
);

-- =====================================================
-- 8. Нормы расхода материалов
-- =====================================================

CREATE TABLE IF NOT EXISTS material_norms (
    norm_id                  SERIAL PRIMARY KEY,
    repair_type              VARCHAR(50) NOT NULL,
    model                    VARCHAR(100) NOT NULL,
    material_name            VARCHAR(200) NOT NULL,
    material_code            VARCHAR(50) NOT NULL,
    unit                     VARCHAR(20) NOT NULL,
    consumption_per_repair   NUMERIC(10,2) NOT NULL CHECK (consumption_per_repair > 0),
    notes                    TEXT,
    UNIQUE (repair_type, model, material_code)
);

-- =====================================================
-- 9. Бригады исполнителей
-- =====================================================

CREATE TABLE IF NOT EXISTS brigades (
    brigade_number         INTEGER PRIMARY KEY,
    team_composition       TEXT,
    specialization         VARCHAR(100) NOT NULL,
    qualification          INTEGER NOT NULL CHECK (qualification >= 0),
    available_start        DATE NOT NULL,
    available_end          DATE NOT NULL,
    available_hours        NUMERIC(10,1) NOT NULL CHECK (available_hours >= 0),
    current_assigned_hours NUMERIC(10,1) NOT NULL DEFAULT 0 CHECK (current_assigned_hours >= 0),
    contact                VARCHAR(200),
    notes                  TEXT,
    CHECK (available_end >= available_start)
);

-- =====================================================
-- 10. Назначение бригад на ремонты
-- =====================================================

CREATE TABLE IF NOT EXISTS repair_assignments (
    assignment_id      SERIAL PRIMARY KEY,
    request_id         INTEGER NOT NULL UNIQUE REFERENCES repair_plan(request_id) ON DELETE CASCADE,
    brigade_number     INTEGER NOT NULL REFERENCES brigades(brigade_number),
    start_date         DATE,
    end_date           DATE,
    planned_hours      NUMERIC(8,1) NOT NULL CHECK (planned_hours > 0),
    responsible_person VARCHAR(200),
    assignment_status  VARCHAR(50) NOT NULL DEFAULT 'назначена',
    notes              TEXT,
    CHECK (start_date IS NULL OR end_date IS NULL OR end_date >= start_date),
    CHECK (assignment_status IN ('назначена', 'не назначена'))
);

-- =====================================================
-- 11. Индексы
-- =====================================================

CREATE INDEX IF NOT EXISTS idx_machines_model ON machines(model);
CREATE INDEX IF NOT EXISTS idx_machine_events_machine_date ON machine_events(machine_id, event_date);
CREATE INDEX IF NOT EXISTS idx_repair_acts_machine_date ON repair_acts(machine_id, end_date);
CREATE INDEX IF NOT EXISTS idx_repair_requests_machine_status ON repair_requests(machine_id, request_status);
CREATE INDEX IF NOT EXISTS idx_repair_requests_priority ON repair_requests(priority_weight);
CREATE INDEX IF NOT EXISTS idx_repair_plan_assigned_month ON repair_plan(assigned_month);
CREATE INDEX IF NOT EXISTS idx_repair_plan_request ON repair_plan(request_id);
CREATE INDEX IF NOT EXISTS idx_material_norms_model ON material_norms(model);
CREATE INDEX IF NOT EXISTS idx_material_norms_repair_type ON material_norms(repair_type);
CREATE INDEX IF NOT EXISTS idx_brigades_specialization ON brigades(specialization);
CREATE INDEX IF NOT EXISTS idx_repair_assignments_brigade ON repair_assignments(brigade_number);
CREATE INDEX IF NOT EXISTS idx_repair_assignments_request ON repair_assignments(request_id);

-- =====================================================
-- 12. Представление: актуальный реестр состояния машин
-- Задача 4.1
-- =====================================================

CREATE OR REPLACE VIEW current_machine_registry AS
WITH last_event AS (
    SELECT DISTINCT ON (machine_id)
        machine_id,
        event_date,
        end_hours,
        operation_status,
        location,
        technical_notes
    FROM machine_events
    ORDER BY machine_id, event_date DESC, event_id DESC
),
last_repair AS (
    SELECT DISTINCT ON (machine_id)
        machine_id,
        end_date,
        hours_after,
        status_after,
        conclusion
    FROM repair_acts
    ORDER BY machine_id, end_date DESC NULLS LAST, act_id DESC
),
last_hours AS (
    SELECT DISTINCT ON (machine_id)
        machine_id,
        hours
    FROM (
        SELECT
            machine_id,
            end_hours AS hours,
            event_date AS source_date,
            1 AS source_priority
        FROM machine_events
        WHERE end_hours IS NOT NULL

        UNION ALL

        SELECT
            machine_id,
            hours_after AS hours,
            end_date AS source_date,
            2 AS source_priority
        FROM repair_acts
        WHERE hours_after IS NOT NULL
    ) s
    ORDER BY machine_id, source_date DESC NULLS LAST, source_priority DESC
)
SELECT
    row_number() OVER (ORDER BY m.machine_id) AS row_no,
    m.machine_id,
    COALESCE(m.plate_number, '') AS plate_number,
    COALESCE(m.serial_number, '') AS serial_number,
    COALESCE(m.model, '') AS model,
    COALESCE(lr.status_after, '') AS technical_state,
    COALESCE(le.operation_status, '') AS operation_status,
    COALESCE(lh.hours, 0)::float8 AS current_hours,
    COALESCE(le.location, '') AS location,
    COALESCE(le.technical_notes, '') AS technical_notes,
    COALESCE(m.notes, '') AS notes,
    COALESCE(
        concat_ws(' / ',
            NULLIF(m.notes, ''),
            NULLIF(lr.conclusion, '')
        ),
        ''
    ) AS remarks
FROM machines m
LEFT JOIN last_event le ON le.machine_id = m.machine_id
LEFT JOIN last_repair lr ON lr.machine_id = m.machine_id
LEFT JOIN last_hours lh ON lh.machine_id = m.machine_id
ORDER BY m.machine_id;

-- =====================================================
-- 13. Представление: месячный план ремонтов
-- Задача 4.4
-- =====================================================

CREATE OR REPLACE VIEW monthly_repair_plan AS
SELECT
    rp.plan_id,
    rp.request_id,
    rr.machine_id,
    m.model,
    rr.repair_type,
    tc.required_specialization,
    COALESCE(rr.required_qualification, tc.required_qualification) AS required_qualification,
    rp.assigned_month,
    rp.planned_start_date,
    rp.planned_end_date,
    COALESCE(tc.labor_hours, 0)::float8 AS labor_hours,
    rr.priority_weight,
    rp.parts_status,
    rp.assignment_status,
    rp.rejection_reason,
    rr.notes AS request_notes
FROM repair_plan rp
JOIN repair_requests rr ON rr.request_id = rp.request_id
JOIN machines m ON m.machine_id = rr.machine_id
LEFT JOIN repair_tech_cards tc
    ON tc.repair_type = rr.repair_type
   AND tc.model = m.model
ORDER BY rp.assigned_month, rr.priority_weight DESC, rp.request_id;

-- =====================================================
-- 14. Представление: потребность материалов по месяцам
-- Для удобства сервера
-- =====================================================

CREATE OR REPLACE VIEW material_demand_by_month AS
SELECT
    rp.assigned_month,
    mn.material_code,
    mn.material_name,
    mn.unit,
    SUM(mn.consumption_per_repair)::float8 AS demand_quantity,
    'Сформировано по плану ремонтов' AS notes
FROM material_norms mn
JOIN repair_requests rr
    ON rr.repair_type = mn.repair_type
JOIN machines m
    ON m.machine_id = rr.machine_id
   AND m.model = mn.model
JOIN repair_plan rp
    ON rp.request_id = rr.request_id
WHERE rp.assignment_status = 'назначена'
GROUP BY
    rp.assigned_month,
    mn.material_code,
    mn.material_name,
    mn.unit
ORDER BY rp.assigned_month, mn.material_code;

-- =====================================================
-- 15. Функция: потребность материалов для выбранного месяца
-- Задача 4.3
-- =====================================================

CREATE OR REPLACE FUNCTION get_material_demand(p_month DATE)
RETURNS TABLE (
    material_code VARCHAR,
    material_name VARCHAR,
    unit VARCHAR,
    demand_quantity NUMERIC,
    notes TEXT
)
LANGUAGE sql
STABLE
AS $$
    SELECT
        mn.material_code,
        mn.material_name,
        mn.unit,
        SUM(mn.consumption_per_repair) AS demand_quantity,
        'Плановый месяц: ' || to_char(p_month, 'MM.YYYY') AS notes
    FROM material_norms mn
    JOIN repair_requests rr
        ON rr.repair_type = mn.repair_type
    JOIN machines m
        ON m.machine_id = rr.machine_id
       AND m.model = mn.model
    JOIN repair_plan rp
        ON rp.request_id = rr.request_id
    WHERE rp.assigned_month = p_month
      AND rp.assignment_status = 'назначена'
    GROUP BY mn.material_code, mn.material_name, mn.unit
    ORDER BY mn.material_code;
$$;

COMMIT;