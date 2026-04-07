#include "mainwindow.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QFormLayout>
#include <QGroupBox>
#include <QHeaderView>
#include <QMessageBox>
#include <QDate>
#include <QJsonArray>
#include <QJsonObject>
#include <QJsonDocument>
#include <qlineedit.h>
#include <qspinbox.h>
#include <qtablewidget.h>

static QTableWidget *createTable(QWidget *parent = nullptr)
{
    auto *table = new QTableWidget(parent);
    table->setSelectionBehavior(QAbstractItemView::SelectRows);
    table->setSelectionMode(QAbstractItemView::SingleSelection);
    table->setEditTriggers(QAbstractItemView::NoEditTriggers);
    table->setAlternatingRowColors(true);
    table->horizontalHeader()->setStretchLastSection(true);
    table->verticalHeader()->setVisible(false);
    return table;
}

MainWindow::MainWindow(QWidget *parent)
    : QMainWindow(parent)
{
    buildUi();
    applyConnection();
}

void MainWindow::buildUi()
{
    auto *central = new QWidget(this);
    auto *rootLayout = new QVBoxLayout(central);

    auto *connGroup = new QGroupBox("Подключение к TCP-серверу", central);
    auto *connLayout = new QHBoxLayout(connGroup);

    m_hostEdit = new QLineEdit("127.0.0.1", connGroup);
    m_portSpin = new QSpinBox(connGroup);
    m_portSpin->setRange(1, 65535);
    m_portSpin->setValue(8080);

    auto *pingBtn = new QPushButton("Проверить связь", connGroup);
    connect(pingBtn, &QPushButton::clicked, this, &MainWindow::onPing);

    connLayout->addWidget(new QLabel("Host:", connGroup));
    connLayout->addWidget(m_hostEdit);
    connLayout->addWidget(new QLabel("Port:", connGroup));
    connLayout->addWidget(m_portSpin);
    connLayout->addWidget(pingBtn);

    rootLayout->addWidget(connGroup);

    auto *tabs = new QTabWidget(central);

    // ---------------- Tab 1: registry ----------------
    {
        auto *tab = new QWidget(tabs);
        auto *layout = new QVBoxLayout(tab);

        auto *top = new QHBoxLayout();
        auto *btn = new QPushButton("Загрузить актуальный реестр", tab);
        connect(btn, &QPushButton::clicked, this, &MainWindow::onLoadRegistry);
        top->addWidget(btn);
        top->addStretch();
        layout->addLayout(top);

        m_registryTable = createTable(tab);
        layout->addWidget(m_registryTable);

        tabs->addTab(tab, "Реестр техники");
    }

    // ---------------- Tab 2: year plan ----------------
    {
        auto *tab = new QWidget(tabs);
        auto *layout = new QVBoxLayout(tab);

        auto *formBox = new QGroupBox("Параметры расчёта", tab);
        auto *form = new QFormLayout(formBox);

        m_yearSpin = new QSpinBox(formBox);
        m_yearSpin->setRange(2000, 2100);
        m_yearSpin->setValue(QDate::currentDate().year());

        auto *genBtn = new QPushButton("Сформировать годовой план", formBox);
        connect(genBtn, &QPushButton::clicked, this, &MainWindow::onGenerateYearPlan);

        form->addRow("Год:", m_yearSpin);
        form->addRow("", genBtn);

        layout->addWidget(formBox);

        m_yearPlanTable = createTable(tab);
        layout->addWidget(m_yearPlanTable);

        tabs->addTab(tab, "Годовой план ремонтов");
    }

    // ---------------- Tab 3: material demand ----------------
    {
        auto *tab = new QWidget(tabs);
        auto *layout = new QVBoxLayout(tab);

        auto *formBox = new QGroupBox("Параметры расчёта", tab);
        auto *form = new QFormLayout(formBox);

        m_monthMaterialsEdit = new QDateEdit(QDate::currentDate(), formBox);
        m_monthMaterialsEdit->setCalendarPopup(true);
        m_monthMaterialsEdit->setDisplayFormat("MM.yyyy");
        m_monthMaterialsEdit->setDate(QDate::currentDate().addDays(1 - QDate::currentDate().day()));

        auto *calcBtn = new QPushButton("Рассчитать ведомость материалов", formBox);
        connect(calcBtn, &QPushButton::clicked, this, &MainWindow::onCalculateMaterialDemand);

        form->addRow("Месяц:", m_monthMaterialsEdit);
        form->addRow("", calcBtn);

        layout->addWidget(formBox);

        m_materialsTable = createTable(tab);
        layout->addWidget(m_materialsTable);

        tabs->addTab(tab, "Ведомость материалов");
    }

    // ---------------- Tab 4: brigade assignment ----------------
    {
        auto *tab = new QWidget(tabs);
        auto *layout = new QVBoxLayout(tab);

        auto *formBox = new QGroupBox("Параметры расчёта", tab);
        auto *form = new QFormLayout(formBox);

        m_monthBrigadesEdit = new QDateEdit(QDate::currentDate(), formBox);
        m_monthBrigadesEdit->setCalendarPopup(true);
        m_monthBrigadesEdit->setDisplayFormat("MM.yyyy");
        m_monthBrigadesEdit->setDate(QDate::currentDate().addDays(1 - QDate::currentDate().day()));

        auto *assignBtn = new QPushButton("Назначить бригады", formBox);
        connect(assignBtn, &QPushButton::clicked, this, &MainWindow::onAssignBrigades);

        form->addRow("Месяц:", m_monthBrigadesEdit);
        form->addRow("", assignBtn);

        layout->addWidget(formBox);

        m_brigadesTable = createTable(tab);
        layout->addWidget(m_brigadesTable);

        tabs->addTab(tab, "Назначение бригад");
    }

    // ---------------- Tab 5: snapshot ----------------
    {
        auto *tab = new QWidget(tabs);
        auto *layout = new QVBoxLayout(tab);

        auto *top = new QHBoxLayout();
        auto *snapBtn = new QPushButton("Загрузить snapshot БД", tab);
        connect(snapBtn, &QPushButton::clicked, this, &MainWindow::onLoadSnapshot);
        top->addWidget(snapBtn);
        top->addStretch();
        layout->addLayout(top);

        m_snapshotEdit = new QPlainTextEdit(tab);
        m_snapshotEdit->setReadOnly(true);
        layout->addWidget(m_snapshotEdit);

        tabs->addTab(tab, "Snapshot");
    }

    rootLayout->addWidget(tabs);

    m_statusLabel = new QLabel("Готово", central);
    rootLayout->addWidget(m_statusLabel);

    setCentralWidget(central);
    resize(1200, 800);
    setWindowTitle("Система учёта и планирования ремонтов");
}

void MainWindow::applyConnection()
{
    m_client.setHost(m_hostEdit->text().trimmed());
    m_client.setPort(static_cast<quint16>(m_portSpin->value()));
}

void MainWindow::setStatus(const QString &text, bool ok)
{
    m_statusLabel->setText(text);
    m_statusLabel->setStyleSheet(ok ? "color: #1b7f3a;" : "color: #b00020;");
}

QString MainWindow::jsonValueToString(const QJsonValue &v) const
{
    if (v.isNull() || v.isUndefined())
        return QString();

    if (v.isBool())
        return v.toBool() ? "да" : "нет";

    if (v.isDouble()) {
        double d = v.toDouble();
        if (qFloor(d) == d)
            return QString::number(static_cast<qint64>(d));
        return QString::number(d, 'f', 1);
    }

    if (v.isString())
        return v.toString();

    if (v.isArray() || v.isObject())
        return QString::fromUtf8(QJsonDocument(v.toArray()).toJson(QJsonDocument::Compact));

    return v.toVariant().toString();
}

void MainWindow::fillTable(QTableWidget *table,
                           const QStringList &headers,
                           const QJsonArray &rows)
{
    table->clear();
    table->setColumnCount(headers.size());
    table->setHorizontalHeaderLabels(headers);
    table->setRowCount(rows.size());

    for (int r = 0; r < rows.size(); ++r) {
        const QJsonObject obj = rows[r].toObject();
        for (int c = 0; c < headers.size(); ++c) {
            const QString key = headers[c];
            QString value = jsonValueToString(obj.value(key));
            auto *item = new QTableWidgetItem(value);
            table->setItem(r, c, item);
        }
    }

    table->resizeColumnsToContents();
}

void MainWindow::fillTextWithJson(QPlainTextEdit *edit, const QJsonValue &value)
{
    if (value.isObject() || value.isArray()) {
        edit->setPlainText(QString::fromUtf8(QJsonDocument(value.toObject()).toJson(QJsonDocument::Indented)));
    } else {
        edit->setPlainText(jsonValueToString(value));
    }
}

void MainWindow::onPing()
{
    applyConnection();

    QString error;
    if (m_client.ping(&error)) {
        setStatus("Связь с сервером есть", true);
    } else {
        setStatus(error, false);
        QMessageBox::warning(this, "Ошибка связи", error);
    }
}

void MainWindow::onLoadRegistry()
{
    applyConnection();

    QJsonObject resp;
    QString error;
    if (!m_client.request("reports.current_registry", QJsonObject{}, &resp, &error)) {
        setStatus(error, false);
        QMessageBox::warning(this, "Ошибка", error);
        return;
    }

    const QJsonArray rows = resp.value("data").toArray();

    fillTable(m_registryTable,
              {
                  "row_no",
                  "machine_id",
                  "plate_number",
                  "serial_number",
                  "model",
                  "technical_state",
                  "operation_status",
                  "current_hours",
                  "location",
                  "technical_notes",
                  "notes",
                  "remarks"
              },
              rows);

    setStatus("Актуальный реестр загружен", true);
}

void MainWindow::onGenerateYearPlan()
{
    applyConnection();

    QJsonObject data;
    data["year"] = m_yearSpin->value();

    QJsonObject resp;
    QString error;
    if (!m_client.request("algorithms.year_plan", data, &resp, &error)) {
        setStatus(error, false);
        QMessageBox::warning(this, "Ошибка", error);
        return;
    }

    const QJsonArray rows = resp.value("data").toArray();

    fillTable(m_yearPlanTable,
              {
                  "plan_id",
                  "request_id",
                  "assigned_month",
                  "planned_start_date",
                  "planned_end_date",
                  "parts_status",
                  "assignment_status",
                  "rejection_reason"
              },
              rows);

    setStatus("Годовой план сформирован", true);
}

void MainWindow::onCalculateMaterialDemand()
{
    applyConnection();

    QJsonObject data;
    data["month"] = m_monthMaterialsEdit->date().toString("yyyy-MM-01");

    QJsonObject resp;
    QString error;
    if (!m_client.request("reports.material_demand", data, &resp, &error)) {
        setStatus(error, false);
        QMessageBox::warning(this, "Ошибка", error);
        return;
    }

    const QJsonArray rows = resp.value("data").toArray();

    fillTable(m_materialsTable,
              {
                  "material_code",
                  "material_name",
                  "unit",
                  "demand_quantity",
                  "notes"
              },
              rows);

    setStatus("Ведомость материалов рассчитана", true);
}

void MainWindow::onAssignBrigades()
{
    applyConnection();

    QJsonObject data;
    data["month"] = m_monthBrigadesEdit->date().toString("yyyy-MM-01");

    QJsonObject resp;
    QString error;
    if (!m_client.request("algorithms.assign_brigades", data, &resp, &error)) {
        setStatus(error, false);
        QMessageBox::warning(this, "Ошибка", error);
        return;
    }

    const QJsonArray rows = resp.value("data").toArray();

    fillTable(m_brigadesTable,
              {
                  "assignment_id",
                  "request_id",
                  "brigade_number",
                  "start_date",
                  "end_date",
                  "planned_hours",
                  "responsible_person",
                  "assignment_status",
                  "notes"
              },
              rows);

    setStatus("Назначение бригад выполнено", true);
}

void MainWindow::onLoadSnapshot()
{
    applyConnection();

    QJsonObject resp;
    QString error;
    if (!m_client.request("snapshot.get", QJsonObject{}, &resp, &error)) {
        setStatus(error, false);
        QMessageBox::warning(this, "Ошибка", error);
        return;
    }

    const QJsonValue data = resp.value("data");
    if (data.isObject()) {
        m_snapshotEdit->setPlainText(QString::fromUtf8(
            QJsonDocument(data.toObject()).toJson(QJsonDocument::Indented)));
    } else {
        m_snapshotEdit->setPlainText("Snapshot не является JSON-объектом");
    }

    setStatus("Snapshot загружен", true);
}
