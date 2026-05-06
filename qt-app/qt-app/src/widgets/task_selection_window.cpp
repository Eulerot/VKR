#include "task_selection_window.h"

#include <QWidget>
#include <QVBoxLayout>
#include <QGridLayout>
#include <QPushButton>
#include <QLabel>
#include <QMessageBox>

#include "net/tcpjsonclient.h"
#include "widgets/task_workspace_window.h"

TaskSelectionWindow::TaskSelectionWindow(TcpJsonClient* client, QWidget* parent)
    : QMainWindow(parent),
    m_client(client)
{
    setWindowTitle("Выбор задачи");
    resize(920, 560);

    auto* central = new QWidget(this);
    auto* root = new QVBoxLayout(central);
    root->setContentsMargins(18, 18, 18, 18);
    root->setSpacing(14);
    setCentralWidget(central);

    auto* title = new QLabel(
        "<div style='font-size:26px;font-weight:800;color:#111;'>Выбор задачи</div>"
        "<div style='color:#666;margin-top:4px;'>Выберите один из модулей системы</div>",
        this);
    title->setAlignment(Qt::AlignCenter);
    root->addWidget(title);

    auto* grid = new QGridLayout();
    grid->setHorizontalSpacing(14);
    grid->setVerticalSpacing(14);

    const QVector<QPair<QString, QString>> tasks = {
        {"6.1", "Формирование реестра состояния техники"},
        {"6.2", "Календарное планирование ремонтов"},
        {"6.3", "Ведомость потребности материалов"},
        {"6.4", "Назначение бригад на ремонты"}
    };

    int idx = 0;
    for (const auto& t : tasks) {
        auto* btn = new QPushButton(t.second, this);
        btn->setMinimumHeight(88);
        btn->setProperty("taskCode", t.first);
        btn->setStyleSheet(R"(
            QPushButton {
                font-size: 15px;
                font-weight: 700;
                padding: 14px 18px;
                border-radius: 16px;
                background: #111827;
                color: #ffffff;
                border: 1px solid #111827;
                text-align: left;
            }
            QPushButton:hover {
                background: #1f2937;
            }
        )");
        connect(btn, &QPushButton::clicked, this, [this, btn]() {
            openTask(btn->property("taskCode").toString());
        });
        grid->addWidget(btn, idx / 2, idx % 2);
        ++idx;
    }

    root->addLayout(grid);

    auto* pingBtn = new QPushButton("Проверить соединение с сервером", this);
    pingBtn->setMinimumHeight(42);
    pingBtn->setStyleSheet(R"(
        QPushButton {
            border-radius: 12px;
            background: #f7f9fb;
            color: #111;
            border: 1px solid #cfd6df;
            font-weight: 600;
        }
        QPushButton:hover { background: #eef3f8; }
    )");
    connect(pingBtn, &QPushButton::clicked, this, [this]() {
        QString err;
        const QJsonObject resp = m_client->request("ping", {}, &err);
        if (!err.isEmpty()) {
            QMessageBox::warning(this, "Ошибка", err);
            return;
        }
        QMessageBox::information(this, "Ответ сервера",
                                 resp.value("data").toObject().value("message").toString());
    });
    root->addWidget(pingBtn);
}

void TaskSelectionWindow::openTask(const QString& taskCode)
{
    auto* w = new TaskWorkspaceWindow(m_client, taskCode);
    w->setAttribute(Qt::WA_DeleteOnClose);
    w->show();
}
