#include "generic_table_page.h"

#include "net/tcpjsonclient.h"
#include "widgets/record_editor_dialog.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QHeaderView>
#include <QTableWidget>
#include <QTableWidgetItem>
#include <QPushButton>
#include <QLabel>
#include <QLineEdit>
#include <QComboBox>
#include <QMessageBox>
#include <QDate>
#include <QRegularExpression>
#include <algorithm>

static QString normalizeText(const QJsonValue& v)
{
    if (v.isNull() || v.isUndefined()) return {};
    if (v.isString()) {
        const QString s = v.toString().trimmed();
        if (s.isEmpty()) return {};
        const QDate d = QDate::fromString(s, "yyyy-MM-dd");
        if (d.isValid())
            return d.toString("dd-MM-yyyy");
        return s;
    }
    if (v.isDouble()) {
        const double d = v.toDouble();
        if (qFuzzyCompare(d + 1.0, 1.0))
            return QString::number(static_cast<qint64>(d));
        return QString::number(d, 'f', 3).remove(QRegularExpression("\\.?0+$"));
    }
    if (v.isBool())
        return v.toBool() ? "да" : "нет";
    return QString::fromUtf8(QJsonDocument(v.toObject()).toJson(QJsonDocument::Compact));
}

GenericTablePage::GenericTablePage(const TableDef& def,
                                   TcpJsonClient* client,
                                   LookupStore* lookup,
                                   QWidget* parent)
    : QWidget(parent),
    m_def(def),
    m_client(client),
    m_lookup(lookup)
{
    auto* root = new QVBoxLayout(this);
    root->setContentsMargins(10, 10, 10, 10);
    root->setSpacing(10);

    m_title = new QLabel(QString("<div style='font-size:18px;font-weight:700;color:#111;'>%1</div>").arg(def.title), this);
    root->addWidget(m_title);

    auto* filterRow = new QHBoxLayout();
    filterRow->setSpacing(8);

    m_searchEdit = new QLineEdit(this);
    m_searchEdit->setPlaceholderText("Поиск...");

    m_searchColumn = new QComboBox(this);
    m_sortColumn = new QComboBox(this);
    m_sortOrder = new QComboBox(this);

    filterRow->addWidget(m_searchEdit, 2);
    filterRow->addWidget(m_searchColumn, 1);
    filterRow->addWidget(m_sortColumn, 1);
    filterRow->addWidget(m_sortOrder, 0);

    root->addLayout(filterRow);

    m_table = new QTableWidget(this);
    m_table->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_table->setSelectionMode(QAbstractItemView::SingleSelection);
    m_table->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_table->setAlternatingRowColors(true);
    m_table->setSortingEnabled(false);
    m_table->verticalHeader()->setVisible(false);
    m_table->horizontalHeader()->setStretchLastSection(true);
    m_table->horizontalHeader()->setSectionsClickable(true);
    root->addWidget(m_table, 1);

    auto* btns = new QHBoxLayout();

    m_refreshBtn = new QPushButton("Обновить", this);
    btns->addWidget(m_refreshBtn);

    if (!m_def.readOnly) {
        m_addBtn = new QPushButton("Добавить", this);
        m_editBtn = new QPushButton("Редактировать", this);
        if (!m_def.deleteAction.isEmpty())
            m_deleteBtn = new QPushButton("Удалить", this);

        btns->addWidget(m_addBtn);
        btns->addWidget(m_editBtn);
        if (m_deleteBtn)
            btns->addWidget(m_deleteBtn);
    }

    btns->addStretch();
    root->addLayout(btns);

    connect(m_refreshBtn, &QPushButton::clicked, this, &GenericTablePage::reload);
    if (m_addBtn) connect(m_addBtn, &QPushButton::clicked, this, &GenericTablePage::onAdd);
    if (m_editBtn) connect(m_editBtn, &QPushButton::clicked, this, &GenericTablePage::onEdit);
    if (m_deleteBtn) connect(m_deleteBtn, &QPushButton::clicked, this, &GenericTablePage::onDelete);

    connect(m_searchEdit, &QLineEdit::textChanged, this, &GenericTablePage::applyFilters);
    connect(m_searchColumn, &QComboBox::currentIndexChanged, this, &GenericTablePage::applyFilters);
    connect(m_sortColumn, &QComboBox::currentIndexChanged, this, &GenericTablePage::applyFilters);
    connect(m_sortOrder, &QComboBox::currentIndexChanged, this, &GenericTablePage::applyFilters);

    rebuildFilterControls();

    setStyleSheet(R"(
        QWidget { background: #ffffff; color: #111; }
        QLabel { color: #111; }
        QLineEdit, QComboBox {
            min-height: 32px;
            border: 1px solid #cfd6df;
            border-radius: 10px;
            padding: 6px 10px;
            background: #ffffff;
        }
        QTableWidget {
            border: 1px solid #d8dde3;
            border-radius: 12px;
            gridline-color: #e8edf2;
            selection-background-color: #dbeafe;
            selection-color: #111;
            background: #ffffff;
        }
        QHeaderView::section {
            background: #f4f6f8;
            padding: 6px;
            border: 1px solid #d8dde3;
            font-weight: 600;
        }
        QPushButton {
            min-height: 34px;
            padding: 8px 16px;
            border-radius: 10px;
            border: 1px solid #cfd6df;
            background: #111827;
            color: #ffffff;
        }
        QPushButton:hover { background: #1f2937; }
    )");
}

void GenericTablePage::rebuildFilterControls()
{
    m_searchColumn->clear();
    m_sortColumn->clear();
    m_visibleFields.clear();

    m_searchColumn->addItem("Все столбцы", QString());
    for (const auto& field : m_def.fields) {
        if (!field.visibleInTable)
            continue;
        m_searchColumn->addItem(field.label, field.key);
        m_sortColumn->addItem(field.label, field.key);
        m_visibleFields << field;
    }

    if (m_sortOrder->count() == 0) {
        m_sortOrder->addItem("↑", "asc");
        m_sortOrder->addItem("↓", "desc");
    }

    if (m_sortColumn->count() > 0)
        m_sortColumn->setCurrentIndex(0);
    m_searchColumn->setCurrentIndex(0);
    m_sortOrder->setCurrentIndex(0);
}

QString GenericTablePage::searchColumnKey() const
{
    return m_searchColumn ? m_searchColumn->currentData().toString() : QString();
}

QString GenericTablePage::displayValue(const QJsonObject& row, const FieldDef& field) const
{
    const auto direct = row.value(field.key);
    QString value = normalizeText(direct);
    if (!value.isEmpty())
        return value;

    if (!m_lookup)
        return value;

    const QString machineId = normalizeText(row.value("machine_id"));
    const QString materialCode = normalizeText(row.value("material_code"));
    const QString unitId = normalizeText(row.value("unit_id"));

    if (field.key == "model") {
        if (!machineId.isEmpty()) {
            const QString x = m_lookup->machineModel(machineId);
            if (!x.isEmpty()) return x;
        }
    } else if (field.key == "material_name") {
        if (!materialCode.isEmpty()) {
            const QString x = m_lookup->materialName(materialCode);
            if (!x.isEmpty()) return x;
        }
    } else if (field.key == "unit_symbol") {
        if (!unitId.isEmpty()) {
            const QString x = m_lookup->unitSymbolById(unitId);
            if (!x.isEmpty()) return x;
        }
        if (!materialCode.isEmpty()) {
            const QString x = m_lookup->materialUnitSymbol(materialCode);
            if (!x.isEmpty()) return x;
        }
    }

    return value.isEmpty() ? "—" : value;
}

QString GenericTablePage::filterValue(const QJsonObject& row, const FieldDef& field) const
{
    return displayValue(row, field);
}

bool GenericTablePage::rowMatchesFilters(const QJsonObject& row) const
{
    const QString global = m_searchEdit ? m_searchEdit->text().trimmed().toLower() : QString();
    const QString selectedKey = searchColumnKey();

    QString joined;

    for (const auto& field : m_visibleFields) {
        const QString s = filterValue(row, field);
        joined += s + " | ";

        if (!global.isEmpty()) {
            if (!selectedKey.isEmpty()) {
                if (field.key == selectedKey) {
                    if (!s.toLower().contains(global))
                        return false;
                }
            }
        }
    }

    if (!global.isEmpty() && selectedKey.isEmpty()) {
        if (!joined.toLower().contains(global))
            return false;
    }

    return true;
}

void GenericTablePage::sortVisibleRows(QJsonArray& rows) const
{
    if (!m_sortColumn || m_sortColumn->count() == 0)
        return;

    const QString key = m_sortColumn->currentData().toString();
    const bool desc = (m_sortOrder->currentData().toString() == "desc");

    auto getFieldKind = [&](const QString& k) -> FieldKind {
        for (const auto& f : m_visibleFields) {
            if (f.key == k)
                return f.kind;
        }
        return FieldKind::Text;
    };

    const FieldKind kind = getFieldKind(key);

    QVector<QJsonObject> vec;
    vec.reserve(rows.size());
    for (const auto& v : rows)
        vec.push_back(v.toObject());

    auto sortKeyText = [&](const QJsonObject& row) -> QString {
        for (const auto& f : m_visibleFields) {
            if (f.key == key)
                return displayValue(row, f);
        }
        return {};
    };

    std::sort(vec.begin(), vec.end(), [&](const QJsonObject& a, const QJsonObject& b) {
        const QString sa = sortKeyText(a);
        const QString sb = sortKeyText(b);

        bool less = false;

        if (kind == FieldKind::Int || kind == FieldKind::Double) {
            less = sa.toDouble() < sb.toDouble();
        } else if (kind == FieldKind::Date) {
            QDate da = QDate::fromString(sa, "dd-MM-yyyy");
            QDate db = QDate::fromString(sb, "dd-MM-yyyy");
            if (!da.isValid()) da = QDate::fromString(sa, "yyyy-MM-dd");
            if (!db.isValid()) db = QDate::fromString(sb, "yyyy-MM-dd");
            if (da.isValid() && db.isValid())
                less = da < db;
            else
                less = sa.toLower() < sb.toLower();
        } else {
            less = sa.toLower() < sb.toLower();
        }

        return desc ? !less : less;
    });

    rows = QJsonArray();
    for (const auto& o : vec)
        rows.append(o);
}

void GenericTablePage::reload()
{
    if (m_def.listAction.isEmpty()) {
        QMessageBox::warning(this, "Ошибка", "Для таблицы не задан listAction.");
        return;
    }

    QString err;
    const QJsonObject resp = m_client->request(m_def.listAction, {}, &err);
    if (!err.isEmpty()) {
        QMessageBox::warning(this, "Ошибка", err);
        return;
    }

    const QJsonValue data = resp.value("data");
    if (!data.isArray()) {
        QMessageBox::warning(this, "Ошибка", "Сервер вернул не массив.");
        return;
    }

    m_rows = data.toArray();
    applyFilters();
    emit changed();
}

void GenericTablePage::setRows(const QJsonArray& rows)
{
    m_rows = rows;
    applyFilters();
}

void GenericTablePage::clearData()
{
    m_rows = {};
    m_visibleRows = {};
    rebuildTable();
}

void GenericTablePage::refreshView()
{
    applyFilters();
}

void GenericTablePage::applyFilters()
{
    QJsonArray filtered;
    for (const auto& v : m_rows) {
        const QJsonObject row = v.toObject();
        if (rowMatchesFilters(row))
            filtered.append(row);
    }

    sortVisibleRows(filtered);
    m_visibleRows = filtered;
    rebuildTable();
}

void GenericTablePage::rebuildTable()
{
    QStringList headers;
    m_visibleFields.clear();

    for (const auto& field : m_def.fields) {
        if (field.visibleInTable) {
            headers << field.label;
            m_visibleFields << field;
        }
    }

    m_table->clear();
    m_table->setColumnCount(headers.size());
    m_table->setHorizontalHeaderLabels(headers);
    m_table->setRowCount(m_visibleRows.size());

    for (int r = 0; r < m_visibleRows.size(); ++r) {
        const QJsonObject row = m_visibleRows[r].toObject();
        int c = 0;
        for (const auto& field : m_visibleFields) {
            auto* item = new QTableWidgetItem(displayValue(row, field));
            m_table->setItem(r, c, item);
            ++c;
        }
    }

    m_table->resizeColumnsToContents();
}

QJsonObject GenericTablePage::selectedRowObject() const
{
    const auto sel = m_table->selectionModel()->selectedRows();
    if (sel.isEmpty()) return {};
    const int r = sel.first().row();
    if (r < 0 || r >= m_visibleRows.size()) return {};
    return m_visibleRows.at(r).toObject();
}

void GenericTablePage::onAdd()
{
    RecordEditorDialog dlg(m_def, {}, m_lookup, this);
    if (dlg.exec() != QDialog::Accepted)
        return;

    QString err;
    const QJsonObject payload = dlg.data();
    const QJsonObject resp = m_client->request(m_def.upsertAction, payload, &err);
    Q_UNUSED(resp);

    if (!err.isEmpty()) {
        QMessageBox::warning(this, "Ошибка", err);
        return;
    }

    reload();
}

void GenericTablePage::onEdit()
{
    const QJsonObject src = selectedRowObject();
    if (src.isEmpty()) {
        QMessageBox::information(this, "Выбор строки", "Выберите строку для редактирования.");
        return;
    }

    RecordEditorDialog dlg(m_def, src, m_lookup, this);
    if (dlg.exec() != QDialog::Accepted)
        return;

    QString err;
    const QJsonObject resp = m_client->request(m_def.upsertAction, dlg.data(), &err);
    Q_UNUSED(resp);

    if (!err.isEmpty()) {
        QMessageBox::warning(this, "Ошибка", err);
        return;
    }

    reload();
}

void GenericTablePage::onDelete()
{
    if (m_def.deleteAction.isEmpty()) {
        QMessageBox::information(this, "Удаление", "Удаление для этой таблицы запрещено.");
        return;
    }

    const QJsonObject src = selectedRowObject();
    if (src.isEmpty()) {
        QMessageBox::information(this, "Выбор строки", "Выберите строку для удаления.");
        return;
    }

    QJsonObject payload;
    for (const auto& key : m_def.keyFields) {
        if (src.contains(key))
            payload[key] = src.value(key);
    }

    QString err;
    const QJsonObject resp = m_client->request(m_def.deleteAction, payload, &err);
    Q_UNUSED(resp);

    if (!err.isEmpty()) {
        QMessageBox::warning(this, "Ошибка", err);
        return;
    }

    reload();
}
