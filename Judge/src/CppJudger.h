#ifndef FUZOJ_SRC_CPPJUDGER_H_
#define FUZOJ_SRC_CPPJUDGER_H_

#include <memory>

#include "Judger.h"
#include "Solution.h"

namespace fuzoj {
class CppRunner : public Runner {
 public:
  CppRunner(const std::string &id, std::shared_ptr<Problem> problem, std::shared_ptr<Solution> solution)
      : Runner(Language::kCpp, id, problem, solution) {}

  int SetRunner(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *outout_sp_) override;

 private:
  void SetCompileEnv();
  void SetRunnerEnv();

  std::string program_name_;
  std::shared_ptr<SandboxProgram> compile_sp_;
  bool valid_;

  static constexpr size_t kCompileMemLimit = 1024 * 1024 * 1024;
};

class CppGrader : public Grader {
 public:
  int SetGrader(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *outout_sp_) override;
 private:
  void SetGraderEnv();
  std::string grader_name_;
  bool valid_;
};
}  // namespace fuzoj

#endif /*FUZOJ_SRC_CPPJUDGER_H_*/