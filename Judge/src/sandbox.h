#ifndef FUZOJ_SRC_SANDBOX_H_
#define FUZOJ_SRC_SANDBOX_H_

#pragma once

#include "solution.h"

#include <memory>
#include <optional>
#include <string>
#include <vector>
#include "fuzoj_utils.h"

namespace fuzoj {
struct SandboxProgram {
  enum Type {
    kProgram,
    kCompile,
    kInterprete,
    kJudger,
  } type_;

  std::string exe_;
  std::vector<std::string> args_;
  std::optional<std::vector<std::string>> env_;

  std::vector<std::shared_ptr<SandboxProgram>> child_;

  // use for standard io.
  std::optional<std::string> input_;
  std::optional<std::string> output_;
  std::optional<std::string> error_;

  std::optional<long long> time_limit_;
  std::optional<size_t> memory_limit_;

  int state_ = 0;

  long time_ms_ = 0;
  size_t mem_byte_ = 0;

  bool normal_exit_ = false;
  // MLE
  bool cgroup_oom_ = false;
  // bool follow_last_;
};

class Sandbox {
 public:
  Sandbox(const std::string &path);
  ~Sandbox();
  Sandbox(Sandbox &&other);
  Sandbox &operator=(Sandbox &&other);
  UNCOPYABLE(Sandbox);

  std::string GetPath() const {
    if (unlikely(!valid_)) {
      return std::string();
    }
    return path_;
  }

  bool Valid() const { return valid_; }
  int AddFile(const std::string &dst, const std::string &src, __mode_t mode = 0777);
  int CopyFile(const std::string &dst, const std::string &src, __mode_t mode = 0777);
  int MoveFile(const std::string &dst, const std::string &src, __mode_t mode = 0777);
  void AddProgram(const std::shared_ptr<SandboxProgram> &program);
  void Run();
  void Destroy();

 private:
  void RunProgram(const std::shared_ptr<SandboxProgram> &program);
  void Excute(const std::shared_ptr<SandboxProgram> &program);
  void SetSandbox(const std::shared_ptr<SandboxProgram> &program);
  void SetOpenFile(const std::shared_ptr<SandboxProgram> &program);
  void LimitTimeAndMem(const std::shared_ptr<SandboxProgram> &program);
  void AvoidSyscall(const std::shared_ptr<SandboxProgram> &program);
  void SwitchUser();

  // main dictory path
  bool valid_ = false;
  std::string path_;
  std::string name_;
  std::vector<std::shared_ptr<SandboxProgram>> programs_;
  static constexpr int kMaxProcessTime = 10;
};
}  // namespace fuzoj
#endif