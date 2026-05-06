#pragma once

#include <QMainWindow>

class TcpJsonClient;

class TaskSelectionWindow : public QMainWindow {
    Q_OBJECT
public:
    explicit TaskSelectionWindow(TcpJsonClient* client, QWidget* parent = nullptr);

private:
    void openTask(const QString& taskCode);

    TcpJsonClient* m_client = nullptr;
};
