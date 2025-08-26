#ifndef FUZOJ_SRC_SOLUTION_H_
#define FUZOJ_SRC_SOLUTION_H_

#pragma once

#include <string>
#include <vector>
namespace fuzoj {
enum Language {
  kCpp,
  kPython,
  kJava,
  kGolang,
  kJavaScript,
  kCSharp,
  kSQL,
};

class Solution {
 public:
  std::string id_;
  std::string text_path_;
  Language language_;
};

class Result {
 public:
  enum Status {
    kAC,
    kWA,
    kCE,
    kTLE,
    kMLE,
    kUKN,
  };
  int id_;
  int score_;
  std::string info_;
};
}  // namespace fuzoj
#endif /*FUZOJ_SRC_PROBLEM_H_*/