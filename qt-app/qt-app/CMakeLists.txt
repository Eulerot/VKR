cmake_minimum_required(VERSION 3.16)
project(qt-app LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC OFF)

find_package(Qt6 REQUIRED COMPONENTS Widgets Network PrintSupport)

add_executable(qt-app
    src/main.cpp
    src/common/api_defs.h
    src/net/tcpjsonclient.h
    src/net/tcpjsonclient.cpp
    src/utils/exporter.h
    src/utils/exporter.cpp
    src/widgets/record_editor_dialog.h
    src/widgets/record_editor_dialog.cpp
    src/widgets/generic_table_page.h
    src/widgets/generic_table_page.cpp
    src/widgets/login_dialog.h
    src/widgets/login_dialog.cpp
    src/widgets/task_selection_window.h
    src/widgets/task_selection_window.cpp
    src/widgets/task_workspace_window.h
    src/widgets/task_workspace_window.cpp
    src/models/lookup_store.h
    src/models/lookup_store.cpp
    form.ui
)

target_include_directories(qt-app PRIVATE src)
target_link_libraries(qt-app PRIVATE Qt6::Widgets Qt6::Network Qt6::PrintSupport)
