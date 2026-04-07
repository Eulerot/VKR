#pragma once

#include <QMainWindow>
#include <QTableWidget>
#include <QPlainTextEdit>
#include <QLineEdit>
#include <QSpinBox>
#include <QDateEdit>
#include <QPushButton>
#include <QLabel>
#include <QTabWidget>

#include "tcpclient.h"

class MainWindow : public QMainWindow
{
    Q_OBJECT

public:
    explicit MainWindow(QWidget *parent = nullptr);

private slots:
    void onPing();
    void onLoadRegistry();
    void onGenerateYearPlan();
    void onCalculateMaterialDemand();
    void onAssignBrigades();
    void onLoadSnapshot();

private:
    void buildUi();
    void applyConnection();
    void setStatus(const QString &text, bool ok = true);

    void fillTable(QTableWidget *table,
                   const QStringList &headers,
                   const QJsonArray &rows);

    void fillTextWithJson(QPlainTextEdit *edit, const QJsonValue &value);
    QString jsonValueToString(const QJsonValue &v) const;

private:
    TcpClient m_client;

    QLineEdit *m_hostEdit{};
    QSpinBox *m_portSpin{};
    QLabel *m_statusLabel{};

    QTableWidget *m_registryTable{};
    QSpinBox *m_yearSpin{};
    QTableWidget *m_yearPlanTable{};

    QDateEdit *m_monthMaterialsEdit{};
    QTableWidget *m_materialsTable{};

    QDateEdit *m_monthBrigadesEdit{};
    QTableWidget *m_brigadesTable{};

    QPlainTextEdit *m_snapshotEdit{};
};
