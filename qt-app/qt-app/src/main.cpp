#include <QApplication>

#include "net/tcpjsonclient.h"
#include "widgets/login_dialog.h"
#include "widgets/task_selection_window.h"

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    QApplication::setApplicationName("MekhZemStroy Client");
    QApplication::setOrganizationName("MekhZemStroy");

    TcpJsonClient client;
    client.setHost("127.0.0.1", 8080);

    LoginDialog login(&client);
    if (login.exec() != QDialog::Accepted)
        return 0;

    TaskSelectionWindow w(&client);
    w.show();

    return app.exec();
}
