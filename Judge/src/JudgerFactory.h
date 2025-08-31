#ifndef JUDGER_SRC_JUDGERFACTORY_H_
#define JUDGER_SRC_JUDGERFACTORY_H_

#include "Judger.h"
#include <optional>
#include "Utils.h"

namespace fuzoj {
class JudgerFactory {
 public:
  JudgerFactory() = default;
  UNCOPYABLE(JudgerFactory);

  std::optional<Judger> GetJudger(const std::shared_ptr<Problem> &problem, const std::shared_ptr<Solution> &solution);
 private:
};
}  // namespace fuzoj
#endif /*JUDGER_SRC_JUDGERFACTORY_H_*/