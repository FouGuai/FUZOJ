#ifndef FUZOJ_SRC_PROBLEM_H_
#define FUZOJ_SRC_PROBLEM_H_

#pragma once

#include <string>
#include <vector>
#include "Types.h"
#include "Solution.h"

namespace fuzoj {

struct TestCase {
  int id_;
  // input data
  std::string data_path_;

  // use for is internal checker.
  std::string answer_path_;

  long long time_limit_;
  size_t mem_limit_;
  int score_;
};

struct Problem {
  std::string id_;
  std::string name_;

  std::string checker_path_;
  Language checker_language_;

  std::vector<TestCase> test_case_;

  int score_;
  int diffculty_;
};

}  // namespace fuzoj
#endif /*FUZOJ_SRC_PROBLEM_H_*/