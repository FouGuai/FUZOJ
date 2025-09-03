#ifndef JUDGER_SRC_JUDGE_EXECUTOR_FACTORY_H_
#define JUDGER_SRC_JUDGE_EXECUTOR_FACTORY_H_

#include <optional>
#include "fuzoj_utils.h"
#include "judge_executor.h"

namespace fuzoj {
class JudgerExecutorFactory {
 public:
  JudgerExecutorFactory() = default;
  UNCOPYABLE(JudgerExecutorFactory);

  std::optional<JudgerExecutor> GetJudger(const std::shared_ptr<Problem> &problem,
                                          const std::shared_ptr<Solution> &solution);

 private:
};
}  // namespace fuzoj
#endif /*JUDGER_SRC_JUDGE_EXECUTOR_FACTORY_H_*/