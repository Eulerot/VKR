#pragma once

#include <QWidget>
#include <QJsonArray>
#include <QJsonObject>

#include "common/api_defs.h"
#include "models/lookup_store.h"

class QLabel;
class QLineEdit;
class QComboBox;
class QPushButton;
class QTableWidget;
class TcpJsonClient;

class GenericTablePage : public QWidget
{
    Q_OBJECT
public:
    explicit GenericTablePage(const TableDef& def,
                              TcpJsonClient* client,
                              LookupStore* lookup,
                              QWidget* parent = nullptr);

    QTableWidget* table() const { return m_table; }

    void reload();
    void setRows(const QJsonArray& rows);
    void clearData();
    void refreshView();

signals:
    void changed();

private slots:
    void applyFilters();
    void onAdd();
    void onEdit();
    void onDelete();

private:
    QString displayValue(const QJsonObject& row, const FieldDef& field) const;
    QString filterValue(const QJsonObject& row, const FieldDef& field) const;
    bool rowMatchesFilters(const QJsonObject& row) const;
    void rebuildTable();
    void rebuildFilterControls();
    void sortVisibleRows(QJsonArray& rows) const;
    QJsonObject selectedRowObject() const;
    QString searchColumnKey() const;

private:
    TableDef m_def;
    TcpJsonClient* m_client = nullptr;
    LookupStore* m_lookup = nullptr;

    QLabel* m_title = nullptr;
    QLineEdit* m_searchEdit = nullptr;
    QComboBox* m_searchColumn = nullptr;
    QComboBox* m_sortColumn = nullptr;
    QComboBox* m_sortOrder = nullptr;

    QPushButton* m_refreshBtn = nullptr;
    QPushButton* m_addBtn = nullptr;
    QPushButton* m_editBtn = nullptr;
    QPushButton* m_deleteBtn = nullptr;

    QTableWidget* m_table = nullptr;

    QJsonArray m_rows;
    QJsonArray m_visibleRows;
    QList<FieldDef> m_visibleFields;
};
