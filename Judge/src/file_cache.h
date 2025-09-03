#ifndef FUZOJ_SRC_FILECACHE_H_
#define FUZOJ_SRC_FILECACHE_H_

#include <atomic>
#include <list>
#include <memory>
#include <shared_mutex>
#include <string>
#include <unordered_map>

#include <problem.h>

namespace fuzoj {

using version_t = std::string;
using uuid_t = std::string;

/**
 * cache a problem's file from distrobute object storage.
 */
struct ProblemFile {
  bool CheckUpToDate();

  Problem meta_;
  uuid_t uuid;

  // file's path
  std::string path_;

  // ref_count
  std::atomic_int pin_ = 0;
  std::shared_mutex mtx_;

  // use for check whether file is up-to-date
  version_t version_;
};

class ProblemFileProxy {
 public:
  void close();

 private:
  std::shared_ptr<ProblemFile> file_;
  bool read_;
};
/**
 * File Cache
 * Restore problem's file.
 */
class FileCache {
 public:
  FileCache(const std::string path = "./problem_cache") : path_(path) {}

  ProblemFileProxy GetFileForRead(const std::string &problem_id);
  ProblemFileProxy GetFile(const std::string &problem_id);

 private:
  void UpdateFile(const std::string &problem_id);

  std::unordered_map<std::string, std::shared_ptr<ProblemFile>> file_caches_;
  std::list<std::string> lru_list_;
  std::mutex mtx_;
  std::string path_;
};
}  // namespace fuzoj

#endif
