#pragma once

#include <QObject>
#include <QJsonObject>
#include <QString>

class TcpClient : public QObject
{
    Q_OBJECT

public:
    explicit TcpClient(QObject *parent = nullptr);

    void setHost(const QString &host);
    void setPort(quint16 port);

    bool ping(QString *error = nullptr);
    bool request(const QString &action,
                 const QJsonObject &data,
                 QJsonObject *response,
                 QString *error);

private:
    QString m_host = "127.0.0.1";
    quint16 m_port = 8080;
};
