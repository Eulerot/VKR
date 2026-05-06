#include "tcpjsonclient.h"

#include <QTcpSocket>
#include <QJsonDocument>
#include <QDebug>

TcpJsonClient::TcpJsonClient(QObject* parent)
    : QObject(parent),
    m_host("127.0.0.1"),
    m_port(8080)
{
}

void TcpJsonClient::setHost(const QString& host, quint16 port)
{
    m_host = host;
    m_port = port;
}

QJsonObject TcpJsonClient::request(const QString& action,
                                   const QJsonObject& payload,
                                   QString* error,
                                   int timeoutMs)
{
    QTcpSocket sock;
    sock.connectToHost(m_host, m_port);
    if (!sock.waitForConnected(timeoutMs)) {
        if (error) *error = "TCP connect failed: " + sock.errorString();
        return {};
    }

    QJsonObject req;
    req["action"] = action;
    if (!payload.isEmpty())
        req["payload"] = payload;

    // toJson всегда возвращает UTF-8
    const QByteArray line = QJsonDocument(req).toJson(QJsonDocument::Compact) + "\n";

    // Отладочный вывод (можно убрать после проверки)
    qDebug() << "Sending hex:" << line.toHex();
    qDebug() << "Sending utf8:" << QString::fromUtf8(line);

    if (sock.write(line) < 0 || !sock.waitForBytesWritten(timeoutMs)) {
        if (error) *error = "TCP write failed: " + sock.errorString();
        return {};
    }

    if (!sock.waitForReadyRead(timeoutMs)) {
        if (error) *error = "TCP read timeout";
        return {};
    }

    QByteArray respLine = sock.readLine();
    if (respLine.isEmpty())
        respLine = sock.readAll();

    const auto doc = QJsonDocument::fromJson(respLine.trimmed());
    if (!doc.isObject()) {
        if (error) *error = "Invalid JSON response";
        return {};
    }

    QJsonObject obj = doc.object();
    if (!obj.value("ok").toBool()) {
        if (error) *error = obj.value("error").toString("Unknown server error");
    }
    return obj;
}
