#include "CppJudger.h"

#include <assert.h>
#include <fcntl.h>
#include <signal.h>
#include <sys/stat.h>
#include <unistd.h>
#include <fstream>
#include <memory>
#include <sstream>

#include "Logger.h"
#include "Sandbox.h"

namespace fuzoj {

const std::string CppRunner::kCompileLogFile = "./compile.log";

void CppRunner::SetCompileEnv() {
  if (unlikely(!valid_)) {
    return;
  }

  program_name_ = "./" + id_ + "_solution";
  if (unlikely(sandbox_->AddFile(program_name_ + ".cc", solution_->text_path_, 0744) < 0)) {
    valid_ = false;
    return;
  }

  auto sp = std::make_shared<SandboxProgram>();

  sp->exe_ = "g++";
  sp->args_ = {"-static", "-o2", program_name_ + ".cc", "-o", program_name_};
  sp->memory_limit_ = kCompileMemLimit;
  sp->error_ = kCompileLogFile;
  sp->type_ = SandboxProgram::kCompile;

  compile_sp_ = std::move(sp);
  sandbox_->AddProgram(compile_sp_);
}

int CppRunner::SetRunner(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *output_sp) {
  assert(sandbox);
  assert(output_sp);

  valid_ = true;
  sandbox_ = sandbox;
  output_sp_ = output_sp;

  SetCompileEnv();
  SetRunnerEnv();
  return valid_ ? 0 : -1;
}

void CppRunner::SetRunnerEnv() {
  if (unlikely(!valid_)) {
    return;
  }

  int id = 0;
  output_sp_->reserve(problem_->test_case_.size());

  for (const TestCase &test_case : problem_->test_case_) {
    std::string base_name = "./" + std::to_string(id++);
    std::string input_file = base_name + ".in";
    std::string output_file = base_name + ".out";

    if (unlikely(sandbox_->AddFile(input_file, test_case.data_path_, 0744)) < 0) {
      valid_ = false;
      return;
    }

    auto sp = std::make_shared<SandboxProgram>();

    sp->exe_ = program_name_;
    sp->memory_limit_ = test_case.mem_limit_;
    sp->time_limit_ = test_case.time_limit_;
    sp->input_ = input_file;
    sp->output_ = output_file;
    sp->type_ = SandboxProgram::kProgram;

    compile_sp_->child_.push_back(sp);
    output_sp_->push_back(std::move(sp));
  }
}

std::vector<TestCaseResult> CppRunner::GetResult() {
  assert(output_sp_->size() == problem_->test_case_.size());

  if (unlikely(!valid_)) {
    return {};
  }

  std::vector<TestCaseResult> rls(output_sp_->size());
  // compile error.
  if (!compile_sp_->normal_exit_) {
    LOGGER.error("Faild to compile, ret: {}.", compile_sp_->state_);
    if (WIFSIGNALED(compile_sp_->state_)) {
      // 子进程被信号杀死
      int sig = WTERMSIG(compile_sp_->state_);
      LOGGER.error("Faild to compile, sig: {}.", sig);
      // 例如 SIGKILL=9, SIGSEGV=11
    }
    // read compile log.
    std::ifstream ifs(sandbox_->GetPath() + kCompileLogFile);
    std::stringstream ss;
    ss << ifs.rdbuf();
    rls[0].info_ = ss.str();

    // all testcast is also CE.
    for (TestCaseResult &result : rls) {
      result.score_ = 0;
      result.state_ = JudgeState::kCE;
    }

    return std::move(rls);
  }

  for (int i = 0; i < output_sp_->size(); ++i) {
    auto &sp = (*output_sp_)[i];
    GetState(sp, rls[i]);
  }

  return rls;
}

void CppRunner::GetState(const std::shared_ptr<SandboxProgram> &sp, TestCaseResult &test_case_result) {
  test_case_result.state_ = JudgeState::kAC;
  test_case_result.mem_byte_ = sp->mem_byte_;
  test_case_result.time_ms_ = sp->time_ms_;
  test_case_result.score_ = 0;

  if (!sp->normal_exit_) {
    if (WIFEXITED(sp->state_)) {
      int ret = WEXITSTATUS(sp->state_);
      if (ret != 0) {
        test_case_result.state_ = JudgeState::kRE;
        test_case_result.info_ = "return value is not zero.";
        return;
      }
    } else if (WIFSIGNALED(sp->state_)) {
      int sig = WTERMSIG(sp->state_);
      // segment fault.
      if (sig == SIGSEGV) {
        test_case_result.state_ = JudgeState::kRE;
        test_case_result.info_ = "segment fault.";
        return;
      }

      if (sig == SIGFPE) {
        test_case_result.state_ = JudgeState::kFPE;
        test_case_result.info_ = "Float error.";
        return;
      }

      if (sig == SIGKILL) {
        if (sp->cgroup_oom_) {
          test_case_result.state_ = JudgeState::kMLE;
          test_case_result.info_ = "MLE";
          return;
        }

        test_case_result.state_ = JudgeState::kRE;
      }
    }
  }

  if (sp->time_limit_ && sp->time_ms_ > *sp->time_limit_) {
    test_case_result.state_ = JudgeState::kTLE;
    return;
  }

  if (sp->memory_limit_ && sp->mem_byte_ > *sp->memory_limit_) {
    test_case_result.state_ = JudgeState::kMLE;
    return;
  }
}

int CppGrader::SetGrader(Sandbox *sandbox, std::vector<std::shared_ptr<SandboxProgram>> *output_sp) {
  assert(sandbox);
  assert(output_sp);
  valid_ = true;
  sandbox_ = sandbox;
  output_sp_ = output_sp;

  SetGraderEnv();

  return valid_ ? 0 : -1;
}

void CppGrader::SetGraderEnv() {
  int id = 0;
  grade_sp_.reserve(output_sp_->size());

  for (const auto &sp : *output_sp_) {
    assert(sp->output_);
    auto grade_sp = std::make_shared<SandboxProgram>();
    grader_name_ = "./" + solution_->id_ + "_judger";
    sandbox_->AddFile(grader_name_, problem_->checker_path_);

    grade_sp->input_ = *sp->output_;
    grade_sp->args_ = {std::to_string(id)};
    grade_sp->output_ = "./" + std::to_string(id++) + ".res";
    grade_sp->exe_ = grader_name_;
    grade_sp->type_ = SandboxProgram::kJudger;

    sp->child_.push_back(grade_sp);
    grade_sp_.push_back(std::move(grade_sp));
  }
}

std::vector<TestCaseResult> CppGrader::GetResult() {
  std::vector<TestCaseResult> rls(grade_sp_.size());
  for (int i = 0; i < grade_sp_.size(); ++i) {
    GetScore(grade_sp_[i], rls[i], i);
  }
  return rls;
}

void CppGrader::GetScore(const std::shared_ptr<SandboxProgram> &sp, TestCaseResult &test_case_result, int id) {
  assert(sp->output_);
  test_case_result.score_ = 0;

  if (!sp->normal_exit_) {
    test_case_result.info_ = "judge error";
    test_case_result.state_ = JudgeState::kUKN;
    return;
  }

  std::ifstream ifs(sandbox_->GetPath() + *sp->output_);
  if (!ifs.is_open()) {
    test_case_result.info_ = "judge error";
    test_case_result.state_ = JudgeState::kUKN;
    return;
  }

  int cnt = 0;
  std::string line;
  if (std::getline(ifs, line)) {
    std::istringstream iss(line);
    if (!(iss >> test_case_result.score_)) {
      test_case_result.score_ = 0;
    }
  }

  if (std::getline(ifs, line)) {
    test_case_result.info_ = std::move(line);
  }

  if (test_case_result.score_ > problem_->test_case_[id].score_ || test_case_result.score_ < -1) {
    test_case_result.score_ = 0;
    test_case_result.state_ = JudgeState::kUKN;
    test_case_result.info_ = "judge error";
    return;
  }

  // -1 mean ac
  if (test_case_result.score_ == problem_->test_case_[id].score_ || test_case_result.score_ == -1) {
    test_case_result.score_ = problem_->test_case_[id].score_;
    test_case_result.state_ = JudgeState::kAC;
    return;
  }

  test_case_result.state_ = JudgeState::kWA;
}

}  // namespace fuzoj