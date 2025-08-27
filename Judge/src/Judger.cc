#ifndef JUDGER_SRC_JUDGE_
#define JUDGER_SRC_JUDGE_
#include "Judger.h"
#include <cassert>

#include "Sandbox.h"

namespace fuzoj {

std::shared_ptr<Result> Judger::Judge() {
  valid_ = true;
  std::vector<std::shared_ptr<SandboxProgram>> output_sp;
  Sandbox sandbox("CPP_" + id_);

  if (unlikely(!sandbox.Valid())) {
    return nullptr;
  }

  runner_->SetRunner(&sandbox, &output_sp);

  if (likely(valid_)) {
    sandbox.Run();
  }

  auto runner_result = runner_->GetResult();
  auto grader_result = grader_->GetResult();
  auto result = Converge(runner_result, grader_result);
  ok_.set_value(true);
  return std::move(result);
}

std::shared_ptr<Result> Judger::Converge(std::vector<TestCaseResult> &runner_result,
                                         std::vector<TestCaseResult> &grader_result) {
  assert(runner_result.size() == grader_result.size());
  auto result = std::make_shared<Result>();
  result->testcase_rel_.resize(runner_result.size());
  result->id_ = solution_->id_;
  result->state_ = JudgeState::kAC;

  for (int i = 0; i < runner_result.size(); ++i) {
    if (runner_result[i].state_ != JudgeState::kAC) {
      result->testcase_rel_[i] = std::move(grader_result[i]);
    } else {
      result->testcase_rel_[i] = std::move(runner_result[i]);
    }
  }

  for (const TestCaseResult &test_case_result : result->testcase_rel_) {
    if (test_case_result.state_ != JudgeState::kAC) {
      // kCE represent all test_case is kCE;
      if (test_case_result.state_ == JudgeState::kCE) {
        result->state_ = JudgeState::kCE;
        result->info_ = test_case_result.info_;
        break;
      }

      if (result->state_ != JudgeState::kAC) {
        result->state_ = JudgeState::kMUL;
      } else {
        result->state_ = test_case_result.state_;
      }
    }

    result->score_ += test_case_result.score_;
  }

  return std::move(result);
}
// std::shared_ptr<Result> Grader::GetGrade() {
//   Result result;
//   for (const auto &grade_sp : grade_sp_) {
//     if (grade_sp->normal_exit_) {
//     }
//   }
// }

}  // namespace fuzoj

#endif /*SRC_JUDGE_*/