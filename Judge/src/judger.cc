#ifndef JUDGER_SRC_JUDGE_
#define JUDGER_SRC_JUDGE_
#include "judger.h"
#include <cassert>

#include "sandbox.h"

namespace fuzoj {

Judger::Judger(Judger &&other) {
  runner_ = std::move(other.runner_);
  grader_ = std::move(other.grader_);
  ok_ = std::move(other.ok_);
  valid_ = other.valid_;
  sandbox_ = other.sandbox_;
  other.valid_ = false;
}

Judger &Judger::operator=(Judger &&other) {
  if (this == &other) {
    return *this;
  }

  std::swap(runner_, other.runner_);
  std::swap(grader_, other.grader_);
  std::swap(ok_, other.ok_);
  std::swap(valid_, other.valid_);
  std::swap(sandbox_, other.sandbox_);
  return *this;
}

std::shared_ptr<Result> Judger::Judge() {
  valid_ = true;
  std::vector<std::shared_ptr<SandboxProgram>> output_sp;
  Sandbox sandbox("CPP_" + runner_->GetSolution()->id_);

  if (unlikely(!sandbox.Valid())) {
    return nullptr;
  }

  if (unlikely(runner_->SetRunner(&sandbox, &output_sp) < 0)) {
    valid_ = false;
    return nullptr;
  }

  if (unlikely(grader_->SetGrader(&sandbox, &output_sp) < 0)) {
    valid_ = false;
    return nullptr;
  }

  if (likely(valid_)) {
    sandbox.Run();
  }

  auto runner_result = runner_->GetResult();
  auto grader_result = grader_->GetResult();
  auto result = Converge(runner_result, grader_result);
  ok_.set_value(true);
  return result;
}

std::shared_ptr<Result> Judger::Converge(std::vector<TestCaseResult> &runner_result,
                                         std::vector<TestCaseResult> &grader_result) {
  assert(runner_result.size() == grader_result.size());
  auto result = std::make_shared<Result>();
  result->testcase_rel_.resize(runner_result.size());
  result->id_ = runner_->GetSolution()->id_;
  result->state_ = JudgeState::kAC;

  for (int i = 0; i < runner_result.size(); ++i) {
    result->testcase_rel_[i] = std::move(runner_result[i]);
    if (runner_result[i].state_ == JudgeState::kAC) {
      result->testcase_rel_[i].state_ = grader_result[i].state_;
      result->testcase_rel_[i].info_ = std::move(grader_result[i].info_);
      result->testcase_rel_[i].score_ = grader_result[i].score_;
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

      if (result->state_ != JudgeState::kAC && result->state_ != test_case_result.state_) {
        result->state_ = JudgeState::kMUL;
      } else {
        result->state_ = test_case_result.state_;
      }
    }
    result->score_ += test_case_result.score_;
  }

  return result;
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