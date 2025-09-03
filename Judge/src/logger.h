#ifndef FUZOJ_SRC_LOGGER_H_
#define FUZOJ_SRC_LOGGER_H_

#include <spdlog/spdlog.h>
#include <memory>
#include "fuzoj_utils.h"
namespace fuzoj {
class Logger {
 public:
  static const std::shared_ptr<spdlog::logger> &GetInstance();
  static void InitLogger();

 private:
  SINGLE_INSTANCE(Logger);
  static std::shared_ptr<spdlog::logger> logger_;
  static std::once_flag once_flag_;
};

#define LOGGER (*Logger::GetInstance())
}  // namespace fuzoj
#endif