#pragma once

#include <QString>

class QTableWidget;

class Exporter
{
public:
    static bool exportTableToCsv(const QTableWidget* table, const QString& filePath);
    static bool exportTableToPdf(const QTableWidget* table,
                                 const QString& documentTitle,
                                 const QString& filePath);
};
