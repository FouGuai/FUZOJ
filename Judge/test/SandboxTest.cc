#include <gtest/gtest.h>
#include <iostream>
#include "../src/Sandbox.h"

using namespace fuzoj;

TEST(SANDBOX, test_running) {
  Sandbox sandbox("./sandbox");

  auto sp = std::make_shared<SandboxProgram>();
  sp->exe_ = "python";
  sp->args_ = {{"hello_world.py"}};
  sandbox.AddFile("hello_world.py", "/home/foushen/project/fuzoj/Judge/test/hello_world.py");
  sp->output_ = "result.out";

  // sp->memory_limit_ =;
  sp->type_ = SandboxProgram::kCompile;
  sandbox.AddProgram(sp);
  sandbox.Run();

  std::cout << sp->state_ << std::endl;
  std::cout << sp->time_ms_ << ' ' << sp->mem_byte_ << std::endl;

  ASSERT_TRUE(WIFEXITED(sp->state_));
  ASSERT_EQ(WEXITSTATUS(sp->state_), 0);
  ASSERT_TRUE(sp->normal_exit_);
}

TEST(SANDBOX, time_limit) {
  Sandbox sandbox("./sandbox");
  auto sp = std::make_shared<SandboxProgram>();
  sp->exe_ = "python";
  sp->args_ = {{"hello_world.py"}};
  sandbox.AddFile("hello_world.py", "/home/foushen/project/fuzoj/Judge/test/hello_world.py");
  sp->output_ = "result.out";

  sp->time_limit_ = 100;
  sp->memory_limit_ = 1024 * 1024 * 1024LL;
  sp->type_ = SandboxProgram::kCompile;
  sandbox.AddProgram(sp);
  sandbox.Run();

  std::cout << sp->state_ << std::endl;
  std::cout << sp->time_ms_ << ' ' << sp->mem_byte_ << std::endl;

  ASSERT_TRUE(WIFEXITED(sp->state_));
  ASSERT_TRUE(sp->normal_exit_);
}

TEST(SANDBOX, compile_) {
  Sandbox sandbox("cpp");
  auto compile = std::make_shared<SandboxProgram>();
  compile->exe_ = "g++";
  compile->args_ = {"-static", "-o2", "test.cpp", "-o", "_test"};
  compile->type_ = SandboxProgram::kCompile;

  sandbox.AddFile("test.cpp", "/home/foushen/project/fuzoj/Judge/test/test.cpp");

  auto run = std::make_shared<SandboxProgram>();
  run->exe_ = "./_test";
  run->type_ = SandboxProgram::kProgram;
  compile->child_.push_back(run);
  run->output_ = "./output.out";
  run->memory_limit_ = 1024 * 1024 * 1024;

  sandbox.AddProgram(compile);
  sandbox.Run();

  std::cout << "compile: " << compile->time_ms_ << "ms ," << compile->mem_byte_ << "B" << std::endl;

  std::cout << WEXITSTATUS(run->state_) << std::endl;
  std::cout << "run " << run->time_ms_ << "ms ," << run->mem_byte_ << "B" << std::endl;

  ASSERT_TRUE(compile->normal_exit_);
  ASSERT_TRUE(run->normal_exit_);
  ASSERT_TRUE(WIFEXITED(run->state_));
}
