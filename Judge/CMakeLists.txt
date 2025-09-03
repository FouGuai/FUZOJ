cmake_minimum_required(VERSION 3.13)

# 项目配置
project(fuzoj_judge LANGUAGES C CXX)

# 基础编译选项
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_BUILD_TYPE Debug)

# 导出 compile_commands.json，方便 clangd 等工具使用
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# =====================
# 第三方依赖
# =====================

# spdlog
find_package(spdlog REQUIRED)

# libseccomp
find_package(PkgConfig REQUIRED)
pkg_check_modules(SECCOMP REQUIRED libseccomp)

# gtest
find_package(GTest REQUIRED)
include_directories(${GTEST_INCLUDE_DIRS})

# nlohmann_json
find_package(nlohmann_json REQUIRED)

# PostgreSQL
find_package(PostgreSQL REQUIRED)

# =====================
# Sanitizer 配置 (仅 GCC/Clang)
# =====================
if (CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    message(STATUS "Enabling AddressSanitizer")
    add_compile_options(-fsanitize=address -fno-omit-frame-pointer -g)
    add_link_options(-fsanitize=address)
endif()

# =====================
# 子目录
# =====================
add_subdirectory(${CMAKE_SOURCE_DIR}/src)
add_subdirectory(${CMAKE_SOURCE_DIR}/test)

# =====================
# 工具支持
# =====================
# 将 compile_commands.json 复制到源码目录
execute_process(
    COMMAND ${CMAKE_COMMAND} -E copy_if_different
            ${CMAKE_BINARY_DIR}/compile_commands.json
            ${CMAKE_SOURCE_DIR}/compile_commands.json
)
