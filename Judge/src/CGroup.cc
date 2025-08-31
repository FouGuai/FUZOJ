#include "CGroup.h"
#include <fcntl.h>
#include <ftw.h>
#include <sys/io.h>
#include <sys/stat.h>
#include <unistd.h>
#include <fstream>
#include <sstream>

#include "Logger.h"
// #include "Types.h"
namespace fuzoj {
const std::string CGroup::kCgroupBase = "/sys/fs/cgroup/";

std::mutex CGroupFactory::mtx_;
std::unordered_set<std::string> CGroupFactory::curr_cgroups_;

CGroup::CGroup(const std::string &name) : name_(Utils::GetFileName(name)), valid_(true) {
  // create cgroup.
  path_ = kCgroupBase + "FUZOJ_" + name_ + "/";
  int cnt = 0;
  while (true) {
    if (mkdir(GetPath().c_str(), 0755) < 0) {
      // try to fix.
      LOGGER.warn("Fail to create cgroup, {}. error: {}.", path_, strerror(errno));
      if (errno == EEXIST && cnt <= 3) {
        if (rmdir(GetPath().c_str()) < 0) {
          LOGGER.warn("Fail to clean cgroup, {}. error {}.", path_, strerror(errno));
        } else {
          continue;
        }
      }
      valid_ = false;
      ++cnt;
      return;
    }
    break;
  }

  LOGGER.info("Create a cgroup named {}.", name_);
}

CGroup::CGroup(CGroup &&other) {
  name_ = std::move(other.name_);
  path_ = std::move(other.path_);
  valid_ = other.valid_;
  time_limit_ = std::move(other.time_limit_);
  mem_limit_ = std::move(other.mem_limit_);
  other.valid_ = false;
}

CGroup &CGroup::operator=(CGroup &&other) {
  if (&other == this) {
    return *this;
  }

  std::swap(name_, other.name_);
  std::swap(path_, other.path_);
  std::swap(valid_, other.valid_);
  std::swap(time_limit_, other.time_limit_);
  std::swap(mem_limit_, other.mem_limit_);
  other.Destroy();
  return *this;
}

// static int unlink_cb(const char *fpath, const struct stat *sb, int typeflag, struct FTW *ftwbuf) {
//   if (typeflag == FTW_D || typeflag == FTW_DP) {
//     return rmdir(fpath);  // 删除目录
//   } else {
//     return unlink(fpath);  // 删除文件
//   }
// }

void CGroup::Destroy() {
  if (!valid_) {
    return;
  }

  // sleep(1);

  // TODO: remove cgroup dictnary recursively.
  if (rmdir(GetPath().c_str()) < 0) {
    LOGGER.warn("Fail to delete cgroup. error: {}.", strerror(errno));
  } else {
    LOGGER.info("Delte cgroup named {}.", name_);
  }

  CGroupFactory::RemoveCGroup(name_);
  valid_ = false;
}

CGroup::~CGroup() { Destroy(); }

int CGroup::AddProcess(pid_t pid) {
  std::string process_path = GetPath() + "cgroup.procs";
  std::ofstream ofs(process_path, std::ios_base::app);
  if (!ofs.is_open()) {
    return -1;
  }

  ofs << pid << '\n';

  if (!ofs) {
    return -1;
  }

  return 0;
}

int CGroup::SetMemLimit(size_t mem_limit) {
  mem_limit_ = mem_limit;

  // 打开 memory.max 文件
  std::ofstream ofs(GetPath() + "memory.max");
  if (!ofs.is_open()) {
    return -1;
  }

  ofs << *mem_limit_ << "\n";

  if (!ofs) {
    return -1;
  }

  return 0;
}

int CGroup::SetTimeLimitms(long long time_ms) { return SetTimeLimit(time_ms * 1000LL); }

int CGroup::SetTimeLimit(long long time_us) {
  if (!valid_) {
    return -1;
  }

  long long period_us = 100000;
  long long quota_us = time_us;

  std::ofstream ofs(GetPath() + "cpu.max");
  if (!ofs.is_open()) {
    return -1;  // 打开失败
  }

  ofs << quota_us << " " << period_us << "\n";

  if (!ofs) {
    return -1;
  }

  return 0;
}

long long CGroup::GetRunTimems() {
  long long us = GetRunTimeus();
  return us == -1 ? -1 : us / 1000LL;
}

long long CGroup::GetRunTimeus() {
  if (!valid_) {
    return -1;
  }

  long long runtime = -1;

  std::ifstream ifs(GetPath() + "cpu.stat");
  if (!ifs.is_open()) return runtime;

  std::string line;
  while (std::getline(ifs, line)) {
    if (line.rfind("usage_usec", 0) == 0) {
      std::istringstream iss(line);
      std::string key;
      long long usec;
      if (iss >> key >> usec) {
        runtime = usec;
      }
      break;
    }
  }

  return runtime;
}

size_t CGroup::GetRunMem() {
  if (!valid_) {
    return 0;
  }

  size_t mem = 0;
  std::ifstream ifs(GetPath() + "memory.peak");
  if (!ifs.is_open()) return mem;

  ifs >> mem;
  return mem;
}

bool CGroup::IsCgroupOom() {
  std::ifstream ifs(GetPath() + "memory.events");
  if (!ifs.is_open()) {
    return false;  // 读取失败就当作不是 OOM
  }

  std::string line;
  while (std::getline(ifs, line)) {
    std::istringstream iss(line);
    std::string key;
    long long value;
    if (iss >> key >> value) {
      if ((key == "oom" || key == "oom_kill") && value > 0) {
        return true;  // 说明发生过 OOM
      }
    }
  }
  return false;
}

void CGroupFactory::RemoveCGroup(const std::string &name) {
  std::unique_lock<std::mutex> lock(mtx_);
  curr_cgroups_.erase(name);
}

std::optional<CGroup> CGroupFactory::GetCGroup(const std::string &name) {
  CGroup cgroup(name);

  // {
  //   std::unique_lock<std::mutex> lock;
  //   if (curr_cgroups_.find(name) != curr_cgroups_.end()) {
  //     return std::nullopt;
  //   }
  //   curr_cgroups_.insert(name);
  // }

  if (!cgroup.valid_) {
    return std::nullopt;
  }

  return std::move(cgroup);
}
}  // namespace fuzoj