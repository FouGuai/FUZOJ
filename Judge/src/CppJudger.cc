#include "CppJudger.h"

#include <assert.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <unistd.h>
#include <fstream>
#include <memory>
#include <sstream>
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

  sandbox_->AddProgram(compile_sp_);
  compile_sp_ = std::move(sp);
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
  if (unlikely(!valid_)) {
    return {};
  }

  std::vector<TestCaseResult> rls(output_sp_->size());
  // compile error.
  if (!compile_sp_->normal_exit_) {
    std::ifstream ifs(sandbox_->GetPath() + kCompileLogFile);
    std::stringstream ss;
    ss << ifs.rdbuf();
    rls[0].info_ = ss.str();

    for (TestCaseResult &result : rls) {
      result.score_ = 0;
      result.state_ = JudgeState::kCE;
    }

    return std::move(rls);
  }

  for (int i = 0; output_sp_->size(); ++i) {
    auto &sp = (*output_sp_)[i];
    GetState(sp, rls[i].state_, rls[i].info_);
  }

  return std::move(rls);
}

void CppRunner::GetState(const std::shared_ptr<SandboxProgram> &sp, JudgeState &state, std::string &info) {
  if (!sp->normal_exit_) {
    return;
  }

  if (sp->time_ms_ > sp->time_limit_) {
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

    grade_sp->input_ = *sp->output_;
    grade_sp->args_ = {std::to_string(id)};
    grade_sp->output_ = "./" + std::to_string(id++) + ".res";
    grade_sp->exe_ = grader_name_;
    grade_sp->type_ = SandboxProgram::kJudger;

    sp->child_.push_back(grade_sp);
    grade_sp_.push_back(std::move(sp));
  }
}

}  // namespace fuzoj