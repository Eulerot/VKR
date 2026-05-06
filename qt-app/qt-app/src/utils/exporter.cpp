#include "exporter.h"

#include <QTableWidget>
#include <QFile>
#include <QTextStream>
#include <QTextDocument>
#include <QPrinter>
#include <QPageLayout>
#include <QMarginsF>
#include <QStringConverter>
#include <QDate>

static QString escHtml(QString s)
{
    s.replace("&", "&amp;");
    s.replace("<", "&lt;");
    s.replace(">", "&gt;");
    s.replace("\"", "&quot;");
    return s;
}

static QString escCsv(QString s)
{
    s.replace("\"", "\"\"");
    if (s.contains(';') || s.contains('"') || s.contains('\n'))
        s = "\"" + s + "\"";
    return s;
}

static QString cellText(const QTableWidget* table, int row, int col)
{
    auto* it = table->item(row, col);
    if (!it) return {};
    return it->text();
}

bool Exporter::exportTableToCsv(const QTableWidget* table, const QString& filePath)
{
    if (!table) return false;

    QFile f(filePath);
    if (!f.open(QIODevice::WriteOnly | QIODevice::Truncate | QIODevice::Text))
        return false;

    QTextStream out(&f);
    out.setEncoding(QStringConverter::Utf8);

    QStringList headers;
    for (int c = 0; c < table->columnCount(); ++c)
        headers << escCsv(table->horizontalHeaderItem(c) ? table->horizontalHeaderItem(c)->text() : QString());

    out << headers.join(';') << "\n";

    for (int r = 0; r < table->rowCount(); ++r) {
        QStringList row;
        for (int c = 0; c < table->columnCount(); ++c)
            row << escCsv(cellText(table, r, c));
        out << row.join(';') << "\n";
    }

    return true;
}

bool Exporter::exportTableToPdf(const QTableWidget* table,
                                const QString& documentTitle,
                                const QString& filePath)
{
    if (!table) return false;

    QString html;
    html += "<html><head><meta charset='utf-8'>";
    html += "<style>";
    html += "body{font-family:Arial,sans-serif;font-size:11pt;margin:0;padding:0;}";
    html += ".title{font-size:16pt;font-weight:700;text-align:center;margin-bottom:6px;}";
    html += ".org{font-size:11pt;text-align:center;margin-bottom:18px;}";
    html += ".dept{margin-bottom:10px;}";
    html += "table{border-collapse:collapse;width:100%;margin-top:10px;}";
    html += "th,td{border:1px solid #000;padding:5px;vertical-align:top;font-size:9pt;}";
    html += "th{font-weight:700;text-align:center;}";
    html += ".footer{margin-top:22px;line-height:1.85;}";
    html += "</style></head><body>";

    html += "<div class='title'>" + escHtml(documentTitle) + "</div>";
    html += "<div class='org'>ООО «МехЗемСтрой»</div>";
    html += "<div class='dept'>Подразделение: ______________________________</div>";
    html += "<div class='dept'>Дата: «____» ______________ 20___ г.</div>";

    html += "<table><thead><tr>";
    for (int c = 0; c < table->columnCount(); ++c) {
        const QString h = table->horizontalHeaderItem(c) ? table->horizontalHeaderItem(c)->text() : QString();
        html += "<th>" + escHtml(h) + "</th>";
    }
    html += "</tr></thead><tbody>";

    for (int r = 0; r < table->rowCount(); ++r) {
        html += "<tr>";
        for (int c = 0; c < table->columnCount(); ++c)
            html += "<td>" + escHtml(cellText(table, r, c)) + "</td>";
        html += "</tr>";
    }

    html += "</tbody></table>";

    html += "<div class='footer'>";
    html += "Ответственный: _____________________________ /_____________/ <br>";
    html += "Должность: _________________________________ <br>";
    html += "Подпись: ___________________    Дата: «____» ______________ 20___ г.";
    html += "</div>";

    html += "</body></html>";

    QTextDocument doc;
    doc.setHtml(html);

    QPrinter printer(QPrinter::HighResolution);
    printer.setOutputFormat(QPrinter::PdfFormat);
    printer.setOutputFileName(filePath);
    printer.setPageOrientation(QPageLayout::Landscape);
    printer.setPageMargins(QMarginsF(10, 10, 10, 10), QPageLayout::Millimeter);

    doc.print(&printer);
    return true;
}
