#pragma once

#include <QDialog>

class QLineEdit;
class TcpJsonClient;

class LoginDialog : public QDialog {
    Q_OBJECT
public:
    explicit LoginDialog(TcpJsonClient* client, QWidget* parent = nullptr);

private slots:
    void doLogin();

private:
    TcpJsonClient* m_client = nullptr;
    QLineEdit* m_login = nullptr;
    QLineEdit* m_password = nullptr;
};
