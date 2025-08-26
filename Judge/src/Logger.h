#ifndef FUZOJ_SRC_LOGGER_H_
#define FUZOJ_SRC_LOGGER_H_

#include <spdlog/spdlog.h>
#include <memory>
#include "Utils.h"
namespace fuzoj {
class Logger {
 public:
  static const std::shared_ptr<spdlog::logger> &GetInstance();
  static void InitLogger();

 private:
  SINGLE_INSTANCE(Logger);
  static std::shared_ptr<spdlog::logger> logger_;
};

#define LOGGER (*Logger::GetInstance())
}  // namespace fuzoj
#endif