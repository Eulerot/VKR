cmake_minimum_required(VERSION 3.16)

project(qt-app LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTOUIC ON)
set(CMAKE_AUTORCC ON)

find_package(Qt6 REQUIRED COMPONENTS Widgets Network)

qt_add_executable(qt-app
    main.cpp
    mainwindow.h
    mainwindow.cpp
    tcpclient.h
    tcpclient.cpp
)

target_link_libraries(qt-app PRIVATE Qt6::Widgets Qt6::Network)
