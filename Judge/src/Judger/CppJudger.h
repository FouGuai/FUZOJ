#ifndef FUZOJ_SRC_JUDGER_CPPJUDGER_H_
#define FUZOJ_SRC_JUDGER_CPPJUDGER_H_

#include <memory>

#include "Judger.h"
#include "Solution.h"

namespace fuzoj {
class CppRunner : public Runner {
 public:
  CppRunner(const std::string &id, std::shared_ptr<Problem> problem, std::shared_ptr<Solution> solution)
      : Runner(Language::kCpp, id, problem, solution) {}

  int SetRunner(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *outout_sp_) override;
  std::vector<TestCaseResult> GetResult() override;

 private:
  void SetCompileEnv();
  void SetRunnerEnv();
  void GetState(const std::shared_ptr<SandboxProgram> &sp, TestCaseResult &test_case_result);

  std::string program_name_;
  std::shared_ptr<SandboxProgram> compile_sp_;

  static constexpr size_t kCompileMemLimit = 1024 * 1024 * 1024;
  static const std::string kCompileLogFile;
};

class CppGrader : public Grader {
 public:
  CppGrader(const std::string &id, std::shared_ptr<Problem> problem, std::shared_ptr<Solution> solution)
      : Grader(Language::kCpp, id, problem, solution) {}
  int SetGrader(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *outout_sp_) override;
  std::vector<TestCaseResult> GetResult() override;

 private:
  void SetGraderEnv();
  void GetScore(const std::shared_ptr<SandboxProgram> &sp, TestCaseResult &test_case_result, int idx);

  std::string grader_name_;
};
}  // namespace fuzoj

#endif /*FUZOJ_SRC_CPPJUDGER_H_*/