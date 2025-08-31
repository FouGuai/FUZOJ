#ifndef FUZOJ_SRC_CGROUP_H_
#define FUZOJ_SRC_CGROUP_H_

#include <mutex>
#include <optional>
#include <string>
#include <unordered_set>
#include "Utils.h"
namespace fuzoj {
class CGroupFactory;

// RAII style
class CGroup {
  friend class CGroupFactory;

 public:
  ~CGroup();
  CGroup(CGroup &&cgroup);
  CGroup &operator=(CGroup &&cgroup);

  int AddProcess(pid_t pid);
  long long GetRunTimems();
  long long GetRunTimeus();
  size_t GetRunMem();
  bool IsCgroupOom();
  int SetTimeLimitms(long long time_ms);
  int SetTimeLimit(long long time_us);
  int SetMemLimit(size_t mem_limit);
  const std::string &GetPath() const { return path_; }
  void Destroy();

 private:
  CGroup(const std::string &name);

  UNCOPYABLE(CGroup);
  std::string name_;
  std::string path_;

  bool valid_ = false;

  // null represent no limit.
  std::optional<long long> time_limit_;
  std::optional<size_t> mem_limit_;

  static const std::string kCgroupBase;
};

class CGroupFactory {
  friend class CGroup;

 public:
  SINGLE_INSTANCE(CGroupFactory);
  static std::optional<CGroup> GetCGroup(const std::string &name);

 private:
  static void RemoveCGroup(const std::string &name);
  static std::mutex mtx_;
  static std::unordered_set<std::string> curr_cgroups_;
};
}  // namespace fuzoj
#endif