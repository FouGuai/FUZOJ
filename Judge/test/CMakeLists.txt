cmake_minimum_required(VERSION 3.13)

include_directories(${CMAKE_CURRENT_SOURCE_DIR})


file(GLOB TESTOURCES "${CMAKE_CURRENT_SOURCE_DIR}/*.cc")

add_executable(Sandbox_test ${TESTOURCES})

target_link_libraries(Sandbox_test PRIVATE ${GTEST_LIBRARIES} pthread)
target_link_libraries(Sandbox_test PRIVATE  ${PROJECT_NAME})
target_link_libraries(Sandbox_test PRIVATE gtest gtest_main pthread fuzoj_judge)