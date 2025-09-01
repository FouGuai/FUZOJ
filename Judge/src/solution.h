#ifndef FUZOJ_SRC_SOLUTION_H_
#define FUZOJ_SRC_SOLUTION_H_

#pragma once

#include <string>
#include <vector>
#include "types.h"
namespace fuzoj {

class Solution {
 public:
  std::string id_;
  std::string text_path_;
  Language language_;
};


struct TestCaseResult {
  JudgeState state_;
  int id_;
  int score_;
  std::string info_;
  long time_ms_;
  size_t mem_byte_;
};

class Result {
 public:
  std::vector<TestCaseResult> testcase_rel_;
  JudgeState state_;
  std::string problem_id_;
  std::string id_;
  int score_;
  std::string info_;
};

}  // namespace fuzoj
#endif /*FUZOJ_SRC_PROBLEM_H_*/