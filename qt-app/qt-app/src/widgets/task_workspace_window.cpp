#include "task_workspace_window.h"

#include <QWidget>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QTabWidget>
#include <QLabel>
#include <QPushButton>
#include <QSpinBox>
#include <QComboBox>
#include <QMessageBox>
#include <QFileDialog>
#include <QDir>
#include <QJsonDocument>
#include <QJsonValue>
#include <QDate>
#include <QRegularExpression>

#include "net/tcpjsonclient.h"
#include "widgets/generic_table_page.h"
#include "utils/exporter.h"

static QString normalizeValue(const QJsonValue& v)
{
    if (v.isNull() || v.isUndefined()) return QString();
    if (v.isString()) {
        const QString s = v.toString().trimmed();
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

TaskWorkspaceWindow::TaskWorkspaceWindow(TcpJsonClient* client,
                                         const QString& taskCode,
                                         QWidget* parent)
    : QMainWindow(parent),
    m_client(client),
    m_taskCode(taskCode)
{
    m_cfg = workspaceForTask(taskCode);
    buildUi();
    applyConfig();
    refreshLookups();
    loadDataPages();
}

void TaskWorkspaceWindow::buildUi()
{
    setWindowTitle(m_cfg.title);
    resize(1440, 920);

    m_central = new QWidget(this);
    setCentralWidget(m_central);

    auto* root = new QVBoxLayout(m_central);
    root->setContentsMargins(10, 10, 10, 10);
    root->setSpacing(10);

    m_title = new QLabel(
        QString("<div style='font-size:24px;font-weight:800;color:#111;'>%1</div>")
            .arg(m_cfg.title),
        this);
    root->addWidget(m_title);

    auto* controls = new QHBoxLayout();
    controls->setSpacing(8);

    m_yearLabel = new QLabel("Год:", this);
    m_yearSpin = new QSpinBox(this);
    m_yearSpin->setRange(2000, 2100);
    m_yearSpin->setValue(QDate::currentDate().year());

    m_monthLabel = new QLabel("Месяц:", this);
    m_monthCombo = new QComboBox(this);
    const QStringList months = {
        "Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
        "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"
    };
    for (int i = 0; i < months.size(); ++i)
        m_monthCombo->addItem(months[i], i + 1);

    m_solveBtn = new QPushButton("Сформировать документ", this);
    m_clearBtn = new QPushButton("Очистить документ", this);
    m_exportPdfBtn = new QPushButton("Экспорт PDF", this);
    m_exportCsvBtn = new QPushButton("Экспорт CSV", this);

    controls->addWidget(m_yearLabel);
    controls->addWidget(m_yearSpin);
    controls->addWidget(m_monthLabel);
    controls->addWidget(m_monthCombo);
    controls->addSpacing(10);
    controls->addWidget(m_solveBtn);
    controls->addWidget(m_clearBtn);
    controls->addWidget(m_exportPdfBtn);
    controls->addWidget(m_exportCsvBtn);
    controls->addStretch();

    root->addLayout(controls);

    m_tabs = new QTabWidget(this);
    root->addWidget(m_tabs, 1);

    m_outputContainer = new QWidget(this);
    auto* outLayout = new QVBoxLayout(m_outputContainer);
    outLayout->setContentsMargins(0, 0, 0, 0);

    m_outputPage = new GenericTablePage(m_cfg.outputTable, m_client, &m_lookup, this);
    outLayout->addWidget(m_outputPage);
    m_tabs->addTab(m_outputContainer, "Выходной документ");

    m_inputTabs = new QTabWidget(this);
    m_tabs->addTab(m_inputTabs, "Входные документы");

    m_refTabs = new QTabWidget(this);
    m_tabs->addTab(m_refTabs, "Справочники");

    connect(m_solveBtn, &QPushButton::clicked, this, &TaskWorkspaceWindow::solveCurrentTask);
    connect(m_clearBtn, &QPushButton::clicked, this, &TaskWorkspaceWindow::clearOutputDocument);
    connect(m_exportPdfBtn, &QPushButton::clicked, this, &TaskWorkspaceWindow::exportPdf);
    connect(m_exportCsvBtn, &QPushButton::clicked, this, &TaskWorkspaceWindow::exportExcelCompatible);

    setStyleSheet(R"(
        QWidget { background: #ffffff; color: #111; }
        QLabel { color: #111; }
        QPushButton {
            min-height: 36px;
            padding: 8px 16px;
            border-radius: 10px;
            border: 1px solid #cfd6df;
            background: #111827;
            color: #ffffff;
        }
        QPushButton:hover { background: #1f2937; }
        QTabWidget::pane { border: 1px solid #d8dde3; border-radius: 12px; top: -1px; }
        QTabBar::tab {
            padding: 10px 16px;
            margin-right: 4px;
            border: 1px solid #d8dde3;
            border-bottom: none;
            border-top-left-radius: 10px;
            border-top-right-radius: 10px;
            background: #f7f9fb;
            color: #111;
        }
        QTabBar::tab:selected { background: #ffffff; font-weight: 600; }
    )");
}

void TaskWorkspaceWindow::applyConfig()
{
    if (m_cfg.controlMode == ControlMode::None) {
        m_yearLabel->hide();
        m_yearSpin->hide();
        m_monthLabel->hide();
        m_monthCombo->hide();
    } else if (m_cfg.controlMode == ControlMode::Year) {
        m_yearLabel->show();
        m_yearSpin->show();
        m_monthLabel->hide();
        m_monthCombo->hide();
    } else {
        m_yearLabel->hide();
        m_yearSpin->hide();
        m_monthLabel->show();
        m_monthCombo->show();
    }

    for (const auto& def : m_cfg.inputTables) {
        auto* p = new GenericTablePage(def, m_client, &m_lookup, this);
        m_inputPages << p;
        m_inputTabs->addTab(p, def.title);
        connect(p, &GenericTablePage::changed, this, &TaskWorkspaceWindow::onAnyDataChanged);
    }

    for (const auto& def : m_cfg.referenceTables) {
        auto* p = new GenericTablePage(def, m_client, &m_lookup, this);
        m_refPages << p;
        m_refTabs->addTab(p, def.title);
        connect(p, &GenericTablePage::changed, this, &TaskWorkspaceWindow::onAnyDataChanged);
    }

    if (m_outputPage)
        m_outputPage->clearData();
}

QJsonArray TaskWorkspaceWindow::requestArray(const QString& action, const QJsonObject& payload)
{
    QString err;
    const QJsonObject resp = m_client->request(action, payload, &err);
    if (!err.isEmpty()) {
        QMessageBox::warning(this, "Ошибка", err);
        return {};
    }

    const QJsonValue data = resp.value("data");
    if (!data.isArray()) {
        QMessageBox::warning(this, "Ошибка", "Сервер вернул не массив.");
        return {};
    }

    return data.toArray();
}

void TaskWorkspaceWindow::setBusy(bool busy)
{
    if (m_solveBtn) m_solveBtn->setEnabled(!busy);
    if (m_clearBtn) m_clearBtn->setEnabled(!busy);
    if (m_exportPdfBtn) m_exportPdfBtn->setEnabled(!busy);
    if (m_exportCsvBtn) m_exportCsvBtn->setEnabled(!busy);
}

void TaskWorkspaceWindow::refreshLookups()
{
    const auto machines = requestArray("machines.list");
    const auto materials = requestArray("materials.list");
    const auto units = requestArray("units.list");
    const auto brigades = requestArray("brigades.list");

    m_lookup.setMachines(machines);
    m_lookup.setMaterials(materials);
    m_lookup.setUnits(units);
    m_lookup.setBrigades(brigades);
}

void TaskWorkspaceWindow::loadDataPages()
{
    for (auto* p : m_inputPages)
        if (p) p->reload();

    for (auto* p : m_refPages)
        if (p) p->reload();

    refreshAllViews();
}

void TaskWorkspaceWindow::refreshAllViews()
{
    if (m_outputPage)
        m_outputPage->refreshView();

    for (auto* p : m_inputPages)
        if (p) p->refreshView();

    for (auto* p : m_refPages)
        if (p) p->refreshView();
}

void TaskWorkspaceWindow::onAnyDataChanged()
{
    if (m_internalRefresh)
        return;

    m_internalRefresh = true;
    refreshLookups();
    refreshAllViews();
    m_internalRefresh = false;
}

void TaskWorkspaceWindow::solveCurrentTask()
{
    setBusy(true);

    QJsonObject payload;

    if (m_taskCode == "6.3") {
        payload["target_month"] = m_monthCombo->currentData().toInt();
    } else if (m_taskCode == "6.4") {
        payload["month"] = m_monthCombo->currentData().toInt();
    }

    QString err;
    const QJsonObject resp = m_client->request(m_cfg.solveAction, payload, &err);
    setBusy(false);

    if (!err.isEmpty()) {
        QMessageBox::warning(this, "Ошибка", err);
        return;
    }

    const QJsonValue data = resp.value("data");
    if (!data.isArray()) {
        QMessageBox::warning(this, "Ошибка", "Сервер вернул не массив.");
        return;
    }

    setOutputRowsFromResponse(data.toArray());
    m_tabs->setCurrentWidget(m_outputContainer);
}

void TaskWorkspaceWindow::setOutputRowsFromResponse(const QJsonArray& arr)
{
    if (!m_outputPage) return;
    m_outputPage->setRows(arr);
}

void TaskWorkspaceWindow::clearOutputDocument()
{
    if (!m_outputPage) return;
    m_outputPage->clearData();
    m_tabs->setCurrentWidget(m_outputContainer);
}

void TaskWorkspaceWindow::exportPdf()
{
    if (!m_outputPage || !m_outputPage->table())
        return;

    const QString file = QFileDialog::getSaveFileName(
        this,
        "Сохранить PDF",
        QDir::homePath() + "/document.pdf",
        "PDF (*.pdf)"
        );
    if (file.isEmpty())
        return;

    if (!Exporter::exportTableToPdf(m_outputPage->table(), m_cfg.outputTable.title, file)) {
        QMessageBox::warning(this, "Ошибка", "Не удалось сохранить PDF.");
        return;
    }

    QMessageBox::information(this, "Готово", "PDF-документ сформирован.");
}

void TaskWorkspaceWindow::exportExcelCompatible()
{
    if (!m_outputPage || !m_outputPage->table())
        return;

    const QString file = QFileDialog::getSaveFileName(
        this,
        "Сохранить таблицу для Excel",
        QDir::homePath() + "/table.csv",
        "CSV (*.csv)"
        );
    if (file.isEmpty())
        return;

    if (!Exporter::exportTableToCsv(m_outputPage->table(), file)) {
        QMessageBox::warning(this, "Ошибка", "Не удалось сохранить CSV.");
        return;
    }

    QMessageBox::information(this, "Готово", "CSV-файл сохранён.");
}
