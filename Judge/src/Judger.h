#ifndef FUZOJ_SRC_JUDFER_H_
#define FUZOJ_SRC_JUDFER_H_

#include <future>
#include <memory>
#include "Problem.h"
#include "Solution.h"
#include "Utils.h"

namespace fuzoj {
class Sandbox;
class SandboxProgram;

class RGBaser {
 public:
  RGBaser(Language language, const std::string &id, std::shared_ptr<Problem> problem,
          std::shared_ptr<Solution> solution)
      : language_(language), id_(id), problem_(std::move(problem)) {}
  virtual ~RGBaser() = default;

  UNCOPYABLE(RGBaser);

  Language GetLanguage() const { return language_; }

 protected:
  Sandbox *sandbox_;
  std::vector<std::shared_ptr<SandboxProgram>> *output_sp_;

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
  virtual std::shared_ptr<Result> GetGrade();
};

class Judger {
 public:
  Judger(std::shared_ptr<Runner> runner_, std::shared_ptr<Grader> &grader)
      : runner_(std::move(runner_)), grader_(std::move(grader)) {}

  UNCOPYABLE(Judger);

  std::shared_ptr<Result> Judge();
  ~Judger() = default;
  std::shared_ptr<Result> GetResult();

 protected:
  std::shared_ptr<Runner> runner_;
  std::shared_ptr<Grader> grader_;
  std::string program_name_;
  std::shared_ptr<Problem> problem_;
  std::shared_ptr<Solution> solution_;
  const std::string id_;
  std::promise<bool> ok_;
  bool valid_;
  Sandbox *sandbox_;

 private:
};

}  // namespace fuzoj
#endif