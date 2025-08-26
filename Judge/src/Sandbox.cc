#include "Sandbox.h"

#include <fcntl.h>
#include <linux/sched.h>
#include <pwd.h>
#include <sched.h>
#include <seccomp.h>
#include <sys/io.h>
#include <sys/prctl.h>
#include <sys/resource.h>
#include <sys/syscall.h>
#include <sys/time.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
#include "sys/stat.h"

#include <stack>

#include "CGroup.h"
#include "Logger.h"

namespace fuzoj {
Sandbox::Sandbox(const std::string &path) : path_(path + "/"), valid_(true) {
  if (mkdir(path_.c_str(), 0755) < 0) {
    if (errno != EEXIST) {
      valid_ = false;
    }
  }
}

Sandbox::Sandbox(Sandbox &&other) {
  valid_ = std::move(other.valid_);
  path_ = std::move(other.path_);
  programs_ = std::move(other.programs_);
  other.valid_ = false;
}

Sandbox &Sandbox::operator=(Sandbox &&other) {
  if (this == &other) {
    return *this;
  }

  std::swap(valid_, other.valid_);
  std::swap(path_, other.path_);
  std::swap(programs_, other.programs_);
  other.Destroy();
  return *this;
}

Sandbox::~Sandbox() { Destroy(); }

void Sandbox::Destroy() {
  if (!valid_) {
    return;
  }

  Utils::RemoveDirRecursive(GetPath());
  valid_ = false;
}

void Sandbox::Run() {
  if (unlikely(!valid_)) {
    return;
  }

  for (const auto &program : programs_) {
    RunProgram(program);
  }
}

int Sandbox::AddFile(const std::string &dst, const std::string &src) {
  if (unlikely(!valid_)) {
    return -1;
  }

  std::string real_dst = path_ + dst;
  if (link(src.c_str(), real_dst.c_str()) < 0) {
    LOGGER.error("Fail to create link, {}.", strerror(errno));
    return -1;
  }
  return 0;
}

int Sandbox::CopyFile(const std::string &dst, const std::string &src) {
  if (unlikely(!valid_)) {
    return -1;
  }

  std::string real_dst = path_ + dst;
  return Utils::CopyFile(real_dst, src);
}

void Sandbox::AddProgram(const std::shared_ptr<SandboxProgram> &program) {
  if (unlikely(!valid_)) {
    return;
  }
  programs_.push_back(program);
}

void Sandbox::RunProgram(const std::shared_ptr<SandboxProgram> &program) {
  int cnt = 0;
  auto p = program;

  Excute(p);

  if (!p->normal_exit_ || p->child_.empty()) {
    return;
  }

  std::stack<std::pair<std::shared_ptr<SandboxProgram>, int>> stk;
  stk.push(std::make_pair(p, 0));

  while (!stk.empty()) {
    auto it = stk.top();
    stk.pop();
    p = it.first->child_[it.second++];

    if (it.second < it.first->child_.size()) {
      stk.push(it);
    }

    Excute(p);
    if (!p->normal_exit_ || p->child_.empty()) {
      continue;
    }

    stk.push(std::make_pair(p, 0));
  }
}

void Sandbox::SetOpenFile(const std::shared_ptr<SandboxProgram> &program) {
  if (program->type_ != SandboxProgram::kCompile) {
    if (chroot("./") < 0) {
      perror("chroot");
      exit(1);
    }
    if (chdir("/") < 0) {  // 把 cwd 改到新根目录
      perror("chdir");
      exit(1);
    }
  }

  if (program->input_) {
    close(STDIN_FILENO);
    int fd = open(program->input_->c_str(), O_CREAT | O_RDONLY, 0644);
    if (fd != STDIN_FILENO) {
      if (fd != -1) {
        close(fd);
      }
      exit(1);
    }
  }

  if (program->output_) {
    close(STDOUT_FILENO);
    int fd = open(program->output_->c_str(), O_CREAT | O_WRONLY | O_TRUNC, 0644);
    if (fd != STDOUT_FILENO) {
      if (fd != -1) close(fd);
      exit(1);
    }
  }

  if (program->error_) {
    close(STDERR_FILENO);
    int fd = open(program->error_->c_str(), O_CREAT | O_WRONLY | O_TRUNC, 0644);
    if (fd != STDERR_FILENO) {
      if (fd != -1) close(fd);
      exit(1);
    }
  }
}

void Sandbox::SwitchUser() {
  // switch to nobody.
  struct passwd *pw = getpwnam("nobody");
  if (!pw) {
    perror("getpwnam");
    exit(1);
  }

  if (setgid(pw->pw_gid) != 0) {
    perror("setgid");
    exit(1);
  }

  if (setuid(pw->pw_uid) != 0) {
    perror("setuid");
    exit(1);
  }

  // 防止子进程获得 root 权限
  prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0);
}

static decltype(SCMP_SYS(socket)) kKilledProgramSyscalls[] = {
    // network
    SCMP_SYS(socket),
    SCMP_SYS(connect),
    SCMP_SYS(accept),
    SCMP_SYS(bind),
    SCMP_SYS(listen),
    SCMP_SYS(sendto),
    SCMP_SYS(recvfrom),
    SCMP_SYS(sendmsg),
    SCMP_SYS(recvmsg),
    SCMP_SYS(shutdown),

    // file
    SCMP_SYS(mknod),
    SCMP_SYS(mkdir),
    SCMP_SYS(rmdir),
    SCMP_SYS(unlink),
    SCMP_SYS(link),
    SCMP_SYS(symlink),
    SCMP_SYS(rename),
    SCMP_SYS(chmod),
    SCMP_SYS(chown),
    SCMP_SYS(fchmod),
    SCMP_SYS(fchown),
    SCMP_SYS(truncate),
    SCMP_SYS(ftruncate),

    // process
    SCMP_SYS(fork),
    SCMP_SYS(vfork),
    SCMP_SYS(clone),
    SCMP_SYS(kill),
    SCMP_SYS(tkill),
    SCMP_SYS(tgkill),
    SCMP_SYS(prctl),
    SCMP_SYS(setpriority),
    SCMP_SYS(setpgid),
    SCMP_SYS(setgid),
    SCMP_SYS(setuid),
    SCMP_SYS(setresuid),
    SCMP_SYS(setresgid),
    SCMP_SYS(setreuid),
    SCMP_SYS(setregid),

    // // kernel
    SCMP_SYS(ptrace),
    SCMP_SYS(syslog),
    SCMP_SYS(reboot),
    SCMP_SYS(swapon),
    SCMP_SYS(swapoff),
    SCMP_SYS(init_module),
    SCMP_SYS(finit_module),
    SCMP_SYS(delete_module),

    // other
    SCMP_SYS(mount),
    SCMP_SYS(umount2),
    SCMP_SYS(chroot),
};

void Sandbox::AvoidSyscall(const std::shared_ptr<SandboxProgram> &program) {
  if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)) {
    perror("prctl");
    exit(1);
  }

  scmp_filter_ctx ctx = seccomp_init(SCMP_ACT_ALLOW);
  if (!ctx) {
    perror("seccomp_init");
    exit(1);
  }

  for (int sc : kKilledProgramSyscalls) {
    if (seccomp_rule_add(ctx, SCMP_ACT_KILL, sc, 0) < 0) {
      perror("seccomp_rule_add");
      exit(1);
    }
  }

  if (seccomp_load(ctx) < 0) {
    perror("seccomp_load");
    exit(1);
  }
  seccomp_release(ctx);
}

void Sandbox::SetSandbox(const std::shared_ptr<SandboxProgram> &program) {
  if (chdir(path_.c_str()) < 0) {
    perror("chdir");
    exit(1);
  }

  SetOpenFile(program);
  if (program->type_ != SandboxProgram::kCompile) {
    // SwitchUser();
    AvoidSyscall(program);
  }
}

static void ParserArgs(const std::shared_ptr<SandboxProgram> &program, std::vector<char *> &argv,
                       std::vector<char *> &envp) {
  argv.push_back(const_cast<char *>(program->exe_.c_str()));
  for (auto &arg : program->args_) {
    argv.push_back(const_cast<char *>(arg.c_str()));
  }
  argv.push_back(nullptr);

  if (program->env_) {
    for (auto &e : *program->env_) {
      envp.push_back(const_cast<char *>(e.c_str()));
    }
  }
  envp.push_back(nullptr);
}

void Sandbox::Excute(const std::shared_ptr<SandboxProgram> &program) {
  int pipes[2];
  if (pipe(pipes) < 0) {
    return;
  }

  // use linux namespace to isolate network, pid and so on.
  struct clone_args args = {0};
  args.flags = CLONE_NEWPID | CLONE_NEWNET | CLONE_NEWUTS;
  args.exit_signal = SIGCHLD;
  pid_t pid = syscall(SYS_clone3, &args, sizeof(args));

  if (pid < 0) {
    LOGGER.error("Faid to fork new program, error {}.", strerror(errno));
    close(pipes[0]);
    close(pipes[1]);
    return;
  }

  if (pid == 0) {
    SetSandbox(program);

    // convert arg.
    std::vector<char *> argv;
    std::vector<char *> envp;
    ParserArgs(program, argv, envp);

    // make sure child is in cgroup.
    int pipe_val;
    if (read(pipes[0], &pipe_val, sizeof(pipe_val)) != sizeof(pipe_val)) {
      perror("pipe read");
      close(pipes[0]);
      close(pipes[1]);
      exit(1);
    }

    close(pipes[0]);
    close(pipes[1]);

    if (program->env_) {
      execvpe(program->exe_.c_str(), argv.data(), envp.data());
    } else {
      execvp(program->exe_.c_str(), argv.data());
    }

    perror("execve failed");
    exit(1);
  } else {
    auto cgroup = CGroupFactory::GetCGroup(program->exe_);

    if (!cgroup) {
      kill(pid, SIGKILL);
      close(pipes[0]);
      close(pipes[1]);
      return;
    }

    if (cgroup->AddProcess(pid) != 0) {
      kill(pid, SIGKILL);
      close(pipes[0]);
      close(pipes[1]);
      return;
    }

    if (program->memory_limit_) {
      if (cgroup->SetMemLimit(*program->memory_limit_) != 0) {
        kill(pid, SIGKILL);
        close(pipes[0]);
        close(pipes[1]);
        return;
      }
    }

    int pipe_val = 0;

    if (write(pipes[1], &pipe_val, sizeof(pipe_val)) != sizeof(pipe_val)) {
      close(pipes[0]);
      close(pipes[1]);
      kill(pid, SIGKILL);
      return;
    }

    close(pipes[0]);
    close(pipes[1]);

    time_t start = time(nullptr);
    int state;
    while (true) {
      int result = waitpid(pid, &state, WNOHANG);
      if (result > 0) {
        break;
      }
      if (program->time_limit_ && cgroup->GetRunTimems() > *program->time_limit_ ||
          time(nullptr) - start > kMaxProcessTime) {
        kill(pid, SIGKILL);
        LOGGER.info("Program {}, timelimt.", pid);
        continue;
      }
      usleep(1000 * 100);
    }

    program->state_ = state;

    if (WIFEXITED(state)) {
      int exit_code = WEXITSTATUS(state);
      program->normal_exit_ = exit_code == 0;
    }

    program->time_ms_ = cgroup->GetRunTimems();
    program->mem_byte_ = cgroup->GetRunMem();
  }
}
}  // namespace fuzoj