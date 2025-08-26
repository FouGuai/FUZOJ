#ifndef JUDGER_SRC_JUDGE_
#define JUDGER_SRC_JUDGE_
#include "Judger.h"
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

  auto result = grader_->GetGrade();
  ok_.set_value(true);
  return std::move(result);
}

}  // namespace fuzoj

#endif /*SRC_JUDGE_*/