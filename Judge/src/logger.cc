#include "logger.h"
#include <spdlog/sinks/stdout_color_sinks.h>

namespace fuzoj {
std::shared_ptr<spdlog::logger> Logger::logger_ = nullptr;

void Logger::InitLogger() { logger_ = spdlog::stdout_color_mt("console"); }

const std::shared_ptr<spdlog::logger> &Logger::GetInstance() {
  std::call_once(once_flag_, []() { InitLogger(); });
  return logger_;
}
}  // namespace fuzoj