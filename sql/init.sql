-- 创建数据库
CREATE DATABASE fuzoj;

-- \c fuzoj; -- 进入数据库
-- 枚举类型
CREATE TYPE Difficulty AS ENUM ('easy', 'medium', 'hard', 'very hard');

CREATE TYPE Mode AS ENUM ('acm', 'ioi', 'noi');

-- 题目表
CREATE TABLE
  problems (
    id UUID PRIMARY KEY NOT NULL UNIQUE,
    problem_name VARCHAR(255) NOT NULL,
    updator_id BIGINT NOT NULL,
    difficulty Difficulty,
    mode Mode NOT NULL,
    create_time TIMESTAMP NOT NULL DEFAULT NOW ()
  );

-- 测试点表
CREATE TABLE
  test_cases (
    id UUID PRIMARY KEY NOT NULL UNIQUE,
    problem_id UUID NOT NULL,
    score INT NOT NULL
  );

CREATE TYPE JudgeState AS ENUM ('AC', 'WA', 'TLE', 'MLE', 'RE');

-- 结果表
CREATE TABLE
  judge_result (
    id UUID PRIMARY KEY NOT NULL UNIQUE,
    problem_id UUID NOT NULL,
    mode Mode NOT NULL,
    score INT NOT NULL,
    judge_state JudgeState NOT NULL,
    info TEXT NOT NULL,
    judge_time TIMESTAMP NOT NULL DEFAULT NOW ()
  );

-- 测试点结果表
CREATE TABLE
  judge_testcase_result (
    id UUID PRIMARY KEY NOT NULL UNIQUE,
    result_id UUID NOT NULL,
    score int NOT NULL,
    judge_state JudgeState NOT NULL,
    info TEXT NOT NULL,
    runtime_ms BIGINT,
    memory_ms BIGINT
  );

CREATE TABLE
  USER ();