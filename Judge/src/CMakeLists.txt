cmake_minimum_required(VERSION 3.13)

include_directories(${CMAKE_CURRENT_SOURCE_DIR})

file(GLOB SOURCES "${CMAKE_CURRENT_SOURCE_DIR}/*.cc")
file(GLOB JUDGER_SRC "${CMAKE_CURRENT_SOURCE_DIR}/Judger/*.cc")

add_library(fuzoj_judge ${SOURCES} ${JUDGER_SRC})
target_link_libraries(fuzoj_judge PRIVATE spdlog::spdlog ${SECCOMP_LIBRARIES} nlohmann_json::nlohmann_json ${PostgreSQL_LIBRARIES})
target_include_directories(fuzoj_judge PRIVATE ${PostgreSQL_INCLUDE_DIRS})