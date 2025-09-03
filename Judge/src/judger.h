#ifndef FUZOJ_SRC_JUGDER_H_
#define FUZOJ_SRC_JUGDER_H_

#include <judge_executor_factory.h>
#include <string>

namespace fuzoj {
struct JudgeInput {
  std::string problem_id_;
  std::string solution_id_;

  std::string solution_path_;
  fuzoj::Language language_;
};

class Judger {
 public:

 private:
  JudgerExecutorFactory factory_;
};
}  // namespace fuzoj

#endif