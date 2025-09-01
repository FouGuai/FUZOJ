#ifndef FUZOJ_SRC_TYPES_H_
#define FUZOJ_SRC_TYPES_H_
namespace fuzoj {
const long long kNanoPerSecond = 1000000000LL;

enum class JudgeState {
  kAC,
  kWA,
  kRE,
  kCE,
  kTLE,
  kMLE,
  kMUL,
  kUKN,
  kFPE,
};

enum class Language {
  kCpp,
  kPython,
  kJava,
  kGolang,
  kJavaScript,
  kCSharp,
  kSQL,
  kInternal,
};

}  // namespace fuzoj
#endif