#ifndef FUZOJ_SRC_TIMER_TIMER_H_
#define FUZOJ_SRC_TIMER_TIMER_H_

#include <chrono>

namespace fuzoj {
using WallClock = std::chrono::system_clock;
using SteadyClock = std::chrono::steady_clock;

class Timer {};

}  // namespace fuzoj
#endif
