#pragma once

#include <QObject>
#include <QString>
#include <QJsonObject>
#include <QJsonArray>

class TcpJsonClient : public QObject {
    Q_OBJECT
public:
    explicit TcpJsonClient(QObject* parent = nullptr);

    void setHost(const QString& host, quint16 port);

    QJsonObject request(const QString& action,
                        const QJsonObject& payload = QJsonObject(),
                        QString* error = nullptr,
                        int timeoutMs = 5000);

private:
    QString m_host;
    quint16 m_port = 8080;
};
