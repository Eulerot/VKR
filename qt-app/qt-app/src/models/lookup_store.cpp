#include "lookup_store.h"

#include <QRegularExpression>
#include <algorithm>

QString LookupStore::strFromJson(const QJsonValue& v)
{
    if (v.isNull() || v.isUndefined()) return {};
    if (v.isString()) return v.toString();
    if (v.isDouble()) {
        const double d = v.toDouble();
        if (qFuzzyCompare(d + 1.0, 1.0))
            return QString::number(static_cast<qint64>(d));
        return QString::number(d, 'f', 3).remove(QRegularExpression("\\.?0+$"));
    }
    return {};
}

QString LookupStore::firstWord(const QString& s)
{
    const QString t = s.trimmed();
    if (t.isEmpty()) return {};
    const auto parts = t.split(QRegularExpression("[,;\\s]+"), Qt::SkipEmptyParts);
    return parts.isEmpty() ? t : parts.first();
}

void LookupStore::clear()
{
    m_machineModel.clear();
    m_materialName.clear();
    m_materialUnitId.clear();
    m_unitSymbolByIdMap.clear();
    m_unitIdBySymbolMap.clear();
    m_brigadePerson.clear();
}

void LookupStore::setMachines(const QJsonArray& rows)
{
    m_machineModel.clear();
    for (const auto& v : rows) {
        const QJsonObject o = v.toObject();
        const QString id = strFromJson(o.value("machine_id"));
        const QString model = strFromJson(o.value("model"));
        if (!id.isEmpty())
            m_machineModel[id] = model;
    }
}

void LookupStore::setMaterials(const QJsonArray& rows)
{
    m_materialName.clear();
    m_materialUnitId.clear();
    for (const auto& v : rows) {
        const QJsonObject o = v.toObject();
        const QString code = strFromJson(o.value("material_code"));
        const QString name = strFromJson(o.value("material_name"));
        const QString unitId = strFromJson(o.value("unit_id"));
        if (!code.isEmpty()) {
            m_materialName[code] = name;
            if (!unitId.isEmpty())
                m_materialUnitId[code] = unitId;
        }
    }
}

void LookupStore::setUnits(const QJsonArray& rows)
{
    m_unitSymbolByIdMap.clear();
    m_unitIdBySymbolMap.clear();

    for (const auto& v : rows) {
        const QJsonObject o = v.toObject();
        const QString id = strFromJson(o.value("unit_id"));
        const QString sym = strFromJson(o.value("unit_symbol"));
        if (!id.isEmpty())
            m_unitSymbolByIdMap[id] = sym;
        if (!sym.isEmpty())
            m_unitIdBySymbolMap[sym] = id;
    }
}

void LookupStore::setBrigades(const QJsonArray& rows)
{
    m_brigadePerson.clear();
    for (const auto& v : rows) {
        const QJsonObject o = v.toObject();
        const QString num = strFromJson(o.value("brigade_number"));
        const QString comp = strFromJson(o.value("team_composition"));
        if (!num.isEmpty())
            m_brigadePerson[num] = firstWord(comp);
    }
}

QVector<LookupOption> LookupStore::machineOptions() const
{
    QVector<LookupOption> out;
    out.reserve(m_machineModel.size());
    for (auto it = m_machineModel.begin(); it != m_machineModel.end(); ++it)
        out.push_back({it.key() + " — " + it.value(), it.key()});
    std::sort(out.begin(), out.end(), [](const LookupOption& a, const LookupOption& b) {
        return a.value < b.value;
    });
    return out;
}

QVector<LookupOption> LookupStore::materialOptions() const
{
    QVector<LookupOption> out;
    out.reserve(m_materialName.size());
    for (auto it = m_materialName.begin(); it != m_materialName.end(); ++it)
        out.push_back({it.key() + " — " + it.value(), it.key()});
    std::sort(out.begin(), out.end(), [](const LookupOption& a, const LookupOption& b) {
        return a.value < b.value;
    });
    return out;
}

QVector<LookupOption> LookupStore::unitOptions() const
{
    QVector<LookupOption> out;
    out.reserve(m_unitIdBySymbolMap.size());
    for (auto it = m_unitIdBySymbolMap.begin(); it != m_unitIdBySymbolMap.end(); ++it)
        out.push_back({it.key(), it.key()}); // символ -> символ
    std::sort(out.begin(), out.end(), [](const LookupOption& a, const LookupOption& b) {
        return a.value < b.value;
    });
    return out;
}

QVector<LookupOption> LookupStore::brigadeOptions() const
{
    QVector<LookupOption> out;
    out.reserve(m_brigadePerson.size());
    for (auto it = m_brigadePerson.begin(); it != m_brigadePerson.end(); ++it)
        out.push_back({it.key() + " — " + it.value(), it.key()});
    std::sort(out.begin(), out.end(), [](const LookupOption& a, const LookupOption& b) {
        return a.value < b.value;
    });
    return out;
}

QString LookupStore::machineModel(const QString& machineId) const
{
    return m_machineModel.value(machineId.trimmed());
}

QString LookupStore::materialName(const QString& materialCode) const
{
    return m_materialName.value(materialCode.trimmed());
}

QString LookupStore::materialUnitId(const QString& materialCode) const
{
    return m_materialUnitId.value(materialCode.trimmed());
}

QString LookupStore::materialUnitSymbol(const QString& materialCode) const
{
    const QString unitId = materialUnitId(materialCode);
    if (unitId.isEmpty())
        return {};
    return unitSymbolById(unitId);
}

QString LookupStore::unitSymbolById(const QString& unitId) const
{
    return m_unitSymbolByIdMap.value(unitId.trimmed());
}

QString LookupStore::unitIdBySymbol(const QString& symbol) const
{
    return m_unitIdBySymbolMap.value(symbol.trimmed());
}

QString LookupStore::brigadePerson(const QString& brigadeNumber) const
{
    return m_brigadePerson.value(brigadeNumber.trimmed());
}
