#include "JudgerFactory.h"
#include <optional>

#include "Judger/CppJudger.h"
#include "Judger/PythonJudger.h"

namespace fuzoj {

std::optional<Judger> JudgerFactory::GetJudger(const std::shared_ptr<Problem> &problem,
                                               const std::shared_ptr<Solution> &solution) {
  std::shared_ptr<Runner> runner;
  std::shared_ptr<Grader> grader;
  switch (problem->checker_language_) {
    case Language::kCpp: {
      grader = std::make_shared<CppGrader>(solution->id_, problem, solution);
    } break;
    case Language::kPython: {
      // grader = std::make_shared<PythonGrader>(solution->id_, problem, solution);
    }
    default:
      break;
  }

  switch (solution->language_) {
    case Language::kCpp: {
      runner = std::make_shared<CppRunner>(solution->id_, problem, solution);
    } break;
    case Language::kPython: {
      // runner = std::make_shared<PythonRunner>(solution->id_, problem, solution);
    } break;
    default:
      break;
  }

  std::optional<Judger> judger = Judger(runner, grader);
  return std::move(judger);
}

}  // namespace fuzoj