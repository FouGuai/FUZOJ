#ifndef FUZOJ_SRC_UTILS_H_
#define FUZOJ_SRC_UTILS_H_

#include <string>

namespace fuzoj {
#define UNCOPYABLE(name)       \
  name(const name &) = delete; \
  name &operator=(const name &) = delete

#define SINGLE_INSTANCE(name) \
  name() = delete;            \
  UNCOPYABLE(name)

#define likely(x) __builtin_expect(!!(x), 1)
#define unlikely(x) __builtin_expect(!!(x), 0)

class Utils {
 public:
  SINGLE_INSTANCE(Utils);
  static std::string GetFileName(const std::string &path);
  static int CopyFile(const std::string &dst, const std::string &src);
  static int RemoveDirRecursive(const std::string &path);
};

}  // namespace fuzoj
#endif /*FUZOJ_SRC_UTILS_H_*/