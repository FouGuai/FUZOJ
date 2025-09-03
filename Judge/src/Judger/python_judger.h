#ifndef FUZOJ_SRC_JUDGER_PYTHONJUDGER_H_
#define FUZOJ_SRC_JUDGER_PYTHONJUDGER_H_

#include "judge_executor.h"

namespace fuzoj {
class PythonRunner : public Runner {
 public:
  PythonRunner(const std::string &id, std::shared_ptr<Problem> problem, std::shared_ptr<Solution> solution)
      : Runner(Language::kPython, id, problem, solution) {}

  int SetRunner(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *outout_sp_) override;
};

class PythonGrader : public Grader {
 public:
  PythonGrader(const std::string &id, std::shared_ptr<Problem> problem, std::shared_ptr<Solution> solution)
      : Grader(Language::kPython, id, problem, solution) {}
};

}  // namespace fuzoj

#endif