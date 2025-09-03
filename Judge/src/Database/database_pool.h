#ifndef FUZOJ_SRC_DATABASE_DATABASE_POOL_H_
#define FUZOJ_SRC_DATABASE_DATABASE_POOL_H_

#include <libpq-fe.h>
#include <chrono>
#include <condition_variable>
#include <list>
#include <set>
#include "Timer/timer.h"
#include "fuzoj_utils.h"

namespace fuzoj {

class DatabasePool;

// RAII style
struct PGPoolItem {
  UNCOPYABLE(PGPoolItem);
  PGPoolItem(const std::string &conn_info_);
  ~PGPoolItem();

  PGconn *conn_;
  SteadyClock::time_point last_modified_;
};

class PGPoolItemGreater {
 public:
  bool operator()(const std::shared_ptr<PGPoolItem> &lhs, const std::shared_ptr<PGPoolItem> &rhs);
};

class PGconnection {
  friend class DatabasePool;

 public:
  UNCOPYABLE(PGconnection);
  PGconnection(PGconnection &&);
  PGconnection &operator=(PGconnection &&);
  ~PGconnection();

  PGconn &operator*() { return *conn_item_->conn_; }
  const PGconn &operator*() const { return *conn_item_->conn_; }

  bool Valid() const noexcept { return valid_; }
  void Release();

 private:
  // only DatabasePool can construct PGconnection
  PGconnection(DatabasePool *database_pool);

  bool valid_ = false;
  std::shared_ptr<PGPoolItem> conn_item_ = nullptr;
  DatabasePool *database_pool_ = nullptr;
};

class DatabasePool {
  friend class PGconnection;

 public:
  UNCOPYABLE(DatabasePool);
  DatabasePool(const std::string &conn_url, const SteadyClock::duration &max_free_time_, size_t max_conn_cnt = 16);
  ~DatabasePool();
  PGconnection GetPGconn();

 private:
  // in indenpendency thread.
  void SweepWorker();

  std::list<std::shared_ptr<PGPoolItem>> free_list_;
  std::set<std::shared_ptr<PGPoolItem>, PGPoolItemGreater> free_sweep_;
  std::list<std::shared_ptr<PGPoolItem>> busy_list_;
  size_t max_conn_cnt = 16;
  SteadyClock::duration max_free_time_;
  mutable std::mutex mtx_;
  mutable std::condition_variable cv_;
};
}  // namespace fuzoj
#endif