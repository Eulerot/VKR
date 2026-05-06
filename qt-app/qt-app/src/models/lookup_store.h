#pragma once

#include <QHash>
#include <QString>
#include <QStringList>
#include <QVector>
#include <QJsonArray>
#include <QJsonObject>
#include <QJsonValue>

struct LookupOption {
    QString text;
    QString value;
};

class LookupStore
{
public:
    void clear();

    void setMachines(const QJsonArray& rows);
    void setMaterials(const QJsonArray& rows);
    void setUnits(const QJsonArray& rows);
    void setBrigades(const QJsonArray& rows);

    QVector<LookupOption> machineOptions() const;
    QVector<LookupOption> materialOptions() const;
    QVector<LookupOption> unitOptions() const;
    QVector<LookupOption> brigadeOptions() const;

    QString machineModel(const QString& machineId) const;
    QString materialName(const QString& materialCode) const;
    QString materialUnitId(const QString& materialCode) const;
    QString materialUnitSymbol(const QString& materialCode) const;
    QString unitSymbolById(const QString& unitId) const;
    QString unitIdBySymbol(const QString& symbol) const;
    QString brigadePerson(const QString& brigadeNumber) const;

private:
    static QString strFromJson(const QJsonValue& v);
    static QString firstWord(const QString& s);

    QHash<QString, QString> m_machineModel;
    QHash<QString, QString> m_materialName;
    QHash<QString, QString> m_materialUnitId;
    QHash<QString, QString> m_unitSymbolByIdMap;
    QHash<QString, QString> m_unitIdBySymbolMap;
    QHash<QString, QString> m_brigadePerson;
};
