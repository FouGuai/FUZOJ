#ifndef FUZOJ_SRC_PROBLEM_H_
#define FUZOJ_SRC_PROBLEM_H_

#pragma once

#include <string>
#include <vector>
#include "Solution.h"

namespace fuzoj {

struct TestCase {
 public:
  int id_;
  std::string data_path_;
  std::string answer_path_;
  long long time_limit_;
  size_t mem_limit_;
};

struct Problem {
 public:
  std::string id_;
  std::string name_;

  std::string checker_path_;
  std::vector<TestCase> test_case_;
  Language checker_language_;

  int score_;
  int diffculty_;
};

}  // namespace fuzoj
#endif /*FUZOJ_SRC_PROBLEM_H_*/