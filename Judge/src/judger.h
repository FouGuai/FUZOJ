#ifndef FUZOJ_SRC_JUDFER_H_
#define FUZOJ_SRC_JUDFER_H_

#include <future>
#include <memory>
#include "problem.h"
#include "solution.h"
#include "utils.h"

namespace fuzoj {
class Sandbox;
class SandboxProgram;

class RGBaser {
 public:
  RGBaser(Language language, const std::string &id, std::shared_ptr<Problem> problem,
          std::shared_ptr<Solution> solution)
      : language_(language), id_(id), problem_(std::move(problem)), solution_(std::move(solution)) {}
  virtual ~RGBaser() = default;

  UNCOPYABLE(RGBaser);

  Language GetLanguage() const { return language_; }
  virtual std::vector<TestCaseResult> GetResult() = 0;
  const std::shared_ptr<Problem> &GetProblem() { return problem_; }
  const std::shared_ptr<Solution> &GetSolution() { return solution_; }
  bool Valid() const noexcept { return valid_; }
 protected:
  Sandbox *sandbox_;
  std::vector<std::shared_ptr<SandboxProgram>> *output_sp_;
  bool valid_;
  std::shared_ptr<Problem> problem_;
  std::shared_ptr<Solution> solution_;
  const std::string id_;

 private:
  Language language_;
};

class Runner : public RGBaser {
 public:
  Runner(Language language, const std::string &id, std::shared_ptr<Problem> problem, std::shared_ptr<Solution> solution)
      : RGBaser(language, id, std::move(problem), std::move(solution)) {}

  virtual int SetRunner(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *outout_sp_) = 0;
};

class Grader : public RGBaser {
 public:
  Grader(Language language, const std::string &id, std::shared_ptr<Problem> problem, std::shared_ptr<Solution> solution)
      : RGBaser(language, id, std::move(problem), std::move(solution)) {}

  virtual int SetGrader(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *outout_sp_) = 0;

 protected:
  std::vector<std::shared_ptr<SandboxProgram>> grade_sp_;
};

class Judger {
 public:
  Judger(std::shared_ptr<Runner> runner, std::shared_ptr<Grader> grader)
      : runner_(std::move(runner)), grader_(std::move(grader)) {}

  UNCOPYABLE(Judger);
  Judger(Judger &&judger);
  Judger &operator=(Judger &&judger);

  std::shared_ptr<Result> Judge();
  ~Judger() = default;
  std::shared_ptr<Result> GetResult();
  std::shared_ptr<Result> Converge(std::vector<TestCaseResult> &runner_result,
                                   std::vector<TestCaseResult> &grader_result);

 private:
  std::shared_ptr<Runner> runner_;
  std::shared_ptr<Grader> grader_;
  std::promise<bool> ok_;
  bool valid_;
  Sandbox *sandbox_;
};

}  // namespace fuzoj
#endif