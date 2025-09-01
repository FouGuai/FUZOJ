#include <gtest/gtest.h>
#include <thread>
#include "../src/JudgerFactory.h"

TEST(TestCppJudger, Judger) {
  std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
  solution->id_ = "sadasdasd";
  solution->language_ = fuzoj::Language::kCpp;
  solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/test.cpp";

  std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
  problem->id_ = "problem1";
  problem->checker_language_ = fuzoj::Language::kCpp;
  problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

  for (int i = 0; i < 3; ++i) {
    fuzoj::TestCase case1;
    case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
    case1.time_limit_ = 1000;
    case1.mem_limit_ = 1024 * 1024 * 1024;
    case1.score_ = 33;
    problem->test_case_.push_back(std::move(case1));
  }

  fuzoj::JudgerFactory judger_factory;
  auto judger = judger_factory.GetJudger(problem, solution);
  auto result = judger->Judge();
  ASSERT_TRUE(!!result);

  ASSERT_EQ(result->state_, fuzoj::JudgeState::kAC);
  for (const auto &test_case_rel : result->testcase_rel_) {
    ASSERT_EQ(test_case_rel.state_, fuzoj::JudgeState::kAC);

    std::cout << "info:" << test_case_rel.info_ << std::endl;
    std::cout << "score:" << test_case_rel.score_ << std::endl;
    std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
    std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

    std::cout << std::endl;
  }
}

TEST(TestCppJudger, JudgerTLE) {
  std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
  solution->id_ = "sadasdasdtle";
  solution->language_ = fuzoj::Language::kCpp;
  solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/testtle.cpp";

  std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
  problem->id_ = "problem1";
  problem->checker_language_ = fuzoj::Language::kCpp;
  problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

  for (int i = 0; i < 3; ++i) {
    fuzoj::TestCase case1;
    case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
    case1.time_limit_ = 1000;
    case1.mem_limit_ = 1024 * 1024;
    case1.score_ = 33;
    problem->test_case_.push_back(std::move(case1));
  }

  fuzoj::JudgerFactory judger_factory;
  auto judger = judger_factory.GetJudger(problem, solution);
  auto result = judger->Judge();
  ASSERT_TRUE(!!result);

  ASSERT_EQ(result->state_, fuzoj::JudgeState::kTLE);
  for (const auto &test_case_rel : result->testcase_rel_) {
    ASSERT_EQ(test_case_rel.state_, fuzoj::JudgeState::kTLE);

    std::cout << "info:" << test_case_rel.info_ << std::endl;
    std::cout << "score:" << test_case_rel.score_ << std::endl;
    std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
    std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

    std::cout << std::endl;
  }
}

TEST(TestCppJudger, JudgerMLE) {
  std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
  solution->id_ = "sadasdasdmle";
  solution->language_ = fuzoj::Language::kCpp;
  solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/testmle.cpp";

  std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
  problem->id_ = "problem1";
  problem->checker_language_ = fuzoj::Language::kCpp;
  problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

  for (int i = 0; i < 3; ++i) {
    fuzoj::TestCase case1;
    case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
    case1.time_limit_ = 1000;
    case1.mem_limit_ = 1024 * 1024 * 1024;
    case1.score_ = 33;
    problem->test_case_.push_back(std::move(case1));
  }

  fuzoj::JudgerFactory judger_factory;
  auto judger = judger_factory.GetJudger(problem, solution);
  auto result = judger->Judge();
  ASSERT_TRUE(!!result);

  ASSERT_EQ(result->state_, fuzoj::JudgeState::kTLE);
  for (const auto &test_case_rel : result->testcase_rel_) {
    ASSERT_EQ(test_case_rel.state_, fuzoj::JudgeState::kTLE);

    std::cout << "info:" << test_case_rel.info_ << std::endl;
    std::cout << "score:" << test_case_rel.score_ << std::endl;
    std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
    std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

    std::cout << std::endl;
  }
}

TEST(TestCppJudger, JudgerCE) {
  std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
  solution->id_ = "sadasdasdmle";
  solution->language_ = fuzoj::Language::kCpp;
  solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/testce.cpp";

  std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
  problem->id_ = "problem1";
  problem->checker_language_ = fuzoj::Language::kCpp;
  problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

  for (int i = 0; i < 3; ++i) {
    fuzoj::TestCase case1;
    case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
    case1.time_limit_ = 1000;
    case1.mem_limit_ = 1024 * 1024 * 1024;
    case1.score_ = 33;
    problem->test_case_.push_back(std::move(case1));
  }

  fuzoj::JudgerFactory judger_factory;
  auto judger = judger_factory.GetJudger(problem, solution);
  auto result = judger->Judge();
  ASSERT_TRUE(!!result);

  ASSERT_EQ(result->state_, fuzoj::JudgeState::kCE);
  for (const auto &test_case_rel : result->testcase_rel_) {
    ASSERT_EQ(test_case_rel.state_, fuzoj::JudgeState::kCE);

    std::cout << "info:" << test_case_rel.info_ << std::endl;
    std::cout << "score:" << test_case_rel.score_ << std::endl;
    std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
    std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

    std::cout << std::endl;
  }
}

TEST(TestCppJudger, JudgerRE) {
  std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
  solution->id_ = "sadasdasdmle";
  solution->language_ = fuzoj::Language::kCpp;
  solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/testre.cpp";

  std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
  problem->id_ = "problem1";
  problem->checker_language_ = fuzoj::Language::kCpp;
  problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

  for (int i = 0; i < 3; ++i) {
    fuzoj::TestCase case1;
    case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
    case1.time_limit_ = 1000;
    case1.mem_limit_ = 1024 * 1024 * 1024;
    case1.score_ = 33;
    problem->test_case_.push_back(std::move(case1));
  }

  fuzoj::JudgerFactory judger_factory;
  auto judger = judger_factory.GetJudger(problem, solution);
  auto result = judger->Judge();
  ASSERT_TRUE(!!result);

  ASSERT_EQ(result->state_, fuzoj::JudgeState::kRE);
  for (const auto &test_case_rel : result->testcase_rel_) {
    ASSERT_TRUE(test_case_rel.state_ == fuzoj::JudgeState::kRE);

    std::cout << "info:" << test_case_rel.info_ << std::endl;
    std::cout << "score:" << test_case_rel.score_ << std::endl;
    std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
    std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

    std::cout << std::endl;
  }
}

TEST(TestCppJudger, JudgerFPE) {
  std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
  solution->id_ = "sadasdasdfpe";
  solution->language_ = fuzoj::Language::kCpp;
  solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/testfpe.cpp";

  std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
  problem->id_ = "problem1";
  problem->checker_language_ = fuzoj::Language::kCpp;
  problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

  for (int i = 0; i < 3; ++i) {
    fuzoj::TestCase case1;
    case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
    case1.time_limit_ = 1000;
    case1.mem_limit_ = 1024 * 1024 * 1024;
    case1.score_ = 33;
    problem->test_case_.push_back(std::move(case1));
  }

  fuzoj::JudgerFactory judger_factory;
  auto judger = judger_factory.GetJudger(problem, solution);
  auto result = judger->Judge();
  ASSERT_TRUE(!!result);

  ASSERT_EQ(result->state_, fuzoj::JudgeState::kFPE);
  for (const auto &test_case_rel : result->testcase_rel_) {
    ASSERT_TRUE(test_case_rel.state_ == fuzoj::JudgeState::kFPE);

    std::cout << "info:" << test_case_rel.info_ << std::endl;
    std::cout << "score:" << test_case_rel.score_ << std::endl;
    std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
    std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

    std::cout << std::endl;
  }
}


TEST(TestCppJudger, JudgerWa) {
  std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
  solution->id_ = "sadasdasdwa";
  solution->language_ = fuzoj::Language::kCpp;
  solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/testwa.cpp";

  std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
  problem->id_ = "problem1";
  problem->checker_language_ = fuzoj::Language::kCpp;
  problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

  for (int i = 0; i < 3; ++i) {
    fuzoj::TestCase case1;
    case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
    case1.time_limit_ = 1000;
    case1.mem_limit_ = 1024 * 1024 * 1024;
    case1.score_ = 33;
    problem->test_case_.push_back(std::move(case1));
  }

  fuzoj::JudgerFactory judger_factory;
  auto judger = judger_factory.GetJudger(problem, solution);
  auto result = judger->Judge();
  ASSERT_TRUE(!!result);

  ASSERT_EQ(result->state_, fuzoj::JudgeState::kWA);
  for (const auto &test_case_rel : result->testcase_rel_) {
    ASSERT_TRUE(test_case_rel.state_ == fuzoj::JudgeState::kWA);

    std::cout << "info:" << test_case_rel.info_ << std::endl;
    std::cout << "score:" << test_case_rel.score_ << std::endl;
    std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
    std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

    std::cout << std::endl;
  }
}

TEST(TestCppJudger, MultiJudger) {
  auto judge = [](int i) {
    std::shared_ptr<fuzoj::Solution> solution = std::make_shared<fuzoj::Solution>();
    solution->id_ = "sadasdasd" + std::to_string(i);
    solution->language_ = fuzoj::Language::kCpp;
    solution->text_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/test.cpp";

    std::shared_ptr<fuzoj::Problem> problem = std::make_shared<fuzoj::Problem>();
    problem->id_ = "problem1" + std::to_string(i);
    problem->checker_language_ = fuzoj::Language::kCpp;
    problem->checker_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/judge";

    for (int i = 0; i < 5; ++i) {
      fuzoj::TestCase case1;
      case1.data_path_ = "/home/foushen/project/fuzoj/Judge/test/TestCppJudger/" + std::to_string(i) + ".in";
      case1.time_limit_ = 1000;
      case1.mem_limit_ = 1024 * 1024 * 1024;
      case1.score_ = 33;
      problem->test_case_.push_back(std::move(case1));
    }

    fuzoj::JudgerFactory judger_factory;
    auto judger = judger_factory.GetJudger(problem, solution);
    auto result = judger->Judge();
    ASSERT_TRUE(!!result);
    ASSERT_EQ(result->state_, fuzoj::JudgeState::kAC);
    for (const auto &test_case_rel : result->testcase_rel_) {
      ASSERT_EQ(test_case_rel.state_, fuzoj::JudgeState::kAC);

      std::cout << "info:" << test_case_rel.info_ << std::endl;
      std::cout << "score:" << test_case_rel.score_ << std::endl;
      std::cout << "time:" << test_case_rel.time_ms_ << std::endl;
      std::cout << "mem:" << test_case_rel.mem_byte_ << std::endl;

      std::cout << std::endl;
    }
  };

  std::vector<std::thread> threads;
  for (int i = 0; i < 50; ++i) {
    threads.emplace_back(judge, i);
  }

  for (int i = 0; i < threads.size(); ++i) {
    if (threads[i].joinable()) {
      threads[i].join();
    }
  }
}