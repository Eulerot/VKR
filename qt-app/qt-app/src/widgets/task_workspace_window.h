#pragma once

#include <QMainWindow>
#include <QJsonArray>

#include "common/api_defs.h"
#include "models/lookup_store.h"

class QLabel;
class QPushButton;
class QTabWidget;
class QSpinBox;
class QComboBox;
class QWidget;
class GenericTablePage;
class TcpJsonClient;

class TaskWorkspaceWindow : public QMainWindow
{
    Q_OBJECT
public:
    explicit TaskWorkspaceWindow(TcpJsonClient* client,
                                 const QString& taskCode,
                                 QWidget* parent = nullptr);

private slots:
    void solveCurrentTask();
    void clearOutputDocument();
    void exportPdf();
    void exportExcelCompatible();

    void refreshLookups();
    void onAnyDataChanged();

private:
    void buildUi();
    void applyConfig();
    void loadDataPages();
    void refreshAllViews();
    void setOutputRowsFromResponse(const QJsonArray& arr);
    QJsonArray requestArray(const QString& action, const QJsonObject& payload = {});
    void setBusy(bool busy);

private:
    TcpJsonClient* m_client = nullptr;
    QString m_taskCode;
    WorkspaceConfig m_cfg;
    LookupStore m_lookup;

    QWidget* m_central = nullptr;
    QLabel* m_title = nullptr;
    QTabWidget* m_tabs = nullptr;

    QWidget* m_outputContainer = nullptr;
    GenericTablePage* m_outputPage = nullptr;

    QTabWidget* m_inputTabs = nullptr;
    QTabWidget* m_refTabs = nullptr;

    QList<GenericTablePage*> m_inputPages;
    QList<GenericTablePage*> m_refPages;

    QSpinBox* m_yearSpin = nullptr;
    QComboBox* m_monthCombo = nullptr;
    QLabel* m_yearLabel = nullptr;
    QLabel* m_monthLabel = nullptr;

    QPushButton* m_solveBtn = nullptr;
    QPushButton* m_clearBtn = nullptr;
    QPushButton* m_exportPdfBtn = nullptr;
    QPushButton* m_exportCsvBtn = nullptr;

    bool m_internalRefresh = false;
};
