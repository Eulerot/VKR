#include "login_dialog.h"

#include <QVBoxLayout>
#include <QFormLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QMessageBox>

#include "net/tcpjsonclient.h"

LoginDialog::LoginDialog(TcpJsonClient* client, QWidget* parent)
    : QDialog(parent),
    m_client(client)
{
    setWindowTitle("Авторизация");
    resize(430, 240);

    auto* root = new QVBoxLayout(this);

    auto* head = new QLabel(
        "<div style='font-size:20px;font-weight:700;'>Авторизация приложения</div>"
        "<div style='color:#666;margin-top:4px;'>ООО «МехЗемСтрой»</div>", this);
    head->setAlignment(Qt::AlignCenter);
    root->addWidget(head);

    auto* form = new QFormLayout();
    m_login = new QLineEdit(this);
    m_password = new QLineEdit(this);
    m_password->setEchoMode(QLineEdit::Password);

    m_login->setPlaceholderText("admin");
    m_password->setPlaceholderText("admin");

    form->addRow("Логин", m_login);
    form->addRow("Пароль", m_password);
    root->addLayout(form);

    auto* hint = new QLabel("Локальная авторизация для диплома: admin / admin", this);
    hint->setStyleSheet("color:#666;");
    root->addWidget(hint);

    auto* buttons = new QHBoxLayout();
    buttons->addStretch();
    auto* enterBtn = new QPushButton("Войти", this);
    auto* closeBtn = new QPushButton("Выход", this);
    buttons->addWidget(enterBtn);
    buttons->addWidget(closeBtn);
    root->addLayout(buttons);

    connect(enterBtn, &QPushButton::clicked, this, &LoginDialog::doLogin);
    connect(closeBtn, &QPushButton::clicked, this, &QDialog::reject);
}

void LoginDialog::doLogin()
{
    const QString u = m_login->text().trimmed();
    const QString p = m_password->text();

    if (u == "admin" && p == "admin") {
        accept();
        return;
    }

    QMessageBox::warning(this, "Ошибка", "Неверный логин или пароль.");
}
