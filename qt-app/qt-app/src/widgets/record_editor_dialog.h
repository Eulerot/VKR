#pragma once

#include <QDialog>
#include <QJsonObject>
#include <QMap>

#include "common/api_defs.h"
#include "models/lookup_store.h"

class QFormLayout;
class QWidget;

class RecordEditorDialog : public QDialog
{
    Q_OBJECT
public:
    explicit RecordEditorDialog(const TableDef& def,
                                const QJsonObject& existing,
                                LookupStore* lookup,
                                QWidget* parent = nullptr);

    QJsonObject data() const;

private:
    QWidget* createEditor(const FieldDef& field);
    void setEditorValue(QWidget* editor, const FieldDef& field, const QJsonValue& value);
    QString readEditor(QWidget* editor, const FieldDef& field) const;
    QWidget* editorFor(const QString& key) const;
    void setFieldText(const QString& key, const QString& value);
    void wireDerivedFields();

private:
    TableDef m_def;
    QJsonObject m_existing;
    LookupStore* m_lookup = nullptr;
    QMap<QString, QWidget*> m_editors;
};
