cmake_minimum_required(VERSION 3.13)

include_directories(${CMAKE_CURRENT_SOURCE_DIR})

set(TEST_FILE)
list(APPEND TEST_FILE "${CMAKE_CURRENT_SOURCE_DIR}/TestSandbox/sandbox_test.cc")
list(APPEND TEST_FILE "${CMAKE_CURRENT_SOURCE_DIR}/TestCppJudger/test_cpp_judger.cc")

foreach(test_file IN LISTS TEST_FILE)
  get_filename_component(test_name ${test_file} NAME_WE)
  add_executable(${test_name} ${test_file})
  target_link_libraries(${test_name}
      PRIVATE
      ${GTEST_LIBRARIES}
      gtest
      gtest_main
      pthread
      fuzoj_judge
      ${PROJECT_NAME}
  )
endforeach()
