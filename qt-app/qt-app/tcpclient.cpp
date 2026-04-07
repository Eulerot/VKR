#include "tcpclient.h"

#include <QTcpSocket>
#include <QJsonDocument>
#include <QJsonParseError>
#include <QElapsedTimer>

TcpClient::TcpClient(QObject *parent)
    : QObject(parent)
{
}

void TcpClient::setHost(const QString &host)
{
    m_host = host;
}

void TcpClient::setPort(quint16 port)
{
    m_port = port;
}

bool TcpClient::ping(QString *error)
{
    QJsonObject resp;
    return request("ping", QJsonObject{}, &resp, error);
}

bool TcpClient::request(const QString &action,
                        const QJsonObject &data,
                        QJsonObject *response,
                        QString *error)
{
    QTcpSocket socket;
    socket.connectToHost(m_host, m_port);

    if (!socket.waitForConnected(3000)) {
        if (error) *error = "Не удалось подключиться к серверу: " + socket.errorString();
        return false;
    }

    QJsonObject root;
    root["action"] = action;
    if (!data.isEmpty())
        root["data"] = data;

    QJsonDocument doc(root);
    QByteArray payload = doc.toJson(QJsonDocument::Compact);
    payload.append('\n');

    if (socket.write(payload) == -1 || !socket.waitForBytesWritten(3000)) {
        if (error) *error = "Не удалось отправить запрос: " + socket.errorString();
        return false;
    }

    QByteArray buffer;
    QElapsedTimer timer;
    timer.start();

    while (timer.elapsed() < 5000) {
        if (socket.waitForReadyRead(500)) {
            buffer += socket.readAll();
            if (buffer.contains('\n'))
                break;
        } else if (socket.state() != QAbstractSocket::ConnectedState) {
            break;
        }
    }

    int nl = buffer.indexOf('\n');
    if (nl >= 0)
        buffer = buffer.left(nl);

    if (buffer.isEmpty()) {
        if (error) *error = "Сервер не прислал ответ";
        return false;
    }

    QJsonParseError parseError;
    QJsonDocument replyDoc = QJsonDocument::fromJson(buffer, &parseError);
    if (parseError.error != QJsonParseError::NoError || !replyDoc.isObject()) {
        if (error) *error = "Ошибка разбора JSON ответа: " + parseError.errorString();
        return false;
    }

    if (response)
        *response = replyDoc.object();

    const QJsonObject obj = replyDoc.object();
    if (obj.contains("ok") && !obj["ok"].toBool()) {
        if (error) *error = obj.value("error").toString("Ошибка сервера");
        return false;
    }

    return true;
}
