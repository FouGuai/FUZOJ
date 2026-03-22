export type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
  details?: Record<string, string>;
  trace_id?: string;
};

export type UserInfo = {
  id: number;
  username: string;
  role: string;
};

export type AuthPayload = {
  access_token: string;
  refresh_token: string;
  access_expires_at: string;
  refresh_expires_at: string;
  user: UserInfo;
};

export type ProblemListItem = {
  problem_id: number;
  title: string;
  version: number;
  updated_at: string;
};

export type ProblemListData = {
  items: ProblemListItem[];
  next_cursor?: string;
  has_more: boolean;
};

export type StatementPayload = {
  problem_id: number;
  version: number;
  statement_md: string;
  updated_at: string;
};

export type LatestMetaPayload = {
  problem_id: number;
  version: number;
  manifest_hash: string;
  data_pack_key: string;
  data_pack_hash: string;
  updated_at: string;
};

export type SubmissionCreatePayload = {
  submission_id: string;
  status: string;
  received_at: number;
};

export type Progress = {
  total_tests: number;
  done_tests: number;
};

export type CompileResult = {
  OK: boolean;
  ExitCode: number;
  TimeMs: number;
  MemoryKB: number;
  Log: string;
  Error: string;
};

export type TestcaseResult = {
  TestID: string;
  Verdict: string;
  TimeMs: number;
  MemoryKB: number;
  OutputKB: number;
  ExitCode: number;
  RuntimeLog: string;
  CheckerLog: string;
  Stdout: string;
  Stderr: string;
  Score: number;
  SubtaskID: string;
};

export type SummaryStat = {
  TotalTimeMs: number;
  MaxMemoryKB: number;
  TotalScore: number;
  FailedTestID: string;
};

export type Timestamps = {
  ReceivedAt: number;
  FinishedAt: number;
};

export type JudgeStatusData = {
  submission_id: string;
  status: string;
  verdict: string;
  score: number;
  language: string;
  summary: SummaryStat;
  compile?: CompileResult;
  tests?: TestcaseResult[];
  timestamps: Timestamps;
  progress: Progress;
  error_code?: number;
  error_message?: string;
};

export type SourceData = {
  submission_id: string;
  problem_id: number;
  user_id: number;
  contest_id?: string;
  language_id: string;
  source_code: string;
  created_at: string;
};

export type CreateProblemPayload = {
  id: number;
};

export type PrepareUploadPayload = {
  upload_id: number;
  problem_id: number;
  version: number;
  bucket: string;
  object_key: string;
  multipart_upload_id: string;
  part_size_bytes: number;
  expires_at: string;
};

export type SignPartsPayload = {
  urls: Record<string, string>;
  expires_in_seconds: number;
};

export type CompleteUploadPayload = {
  problem_id: number;
  version: number;
  manifest_hash: string;
  data_pack_key: string;
  data_pack_hash: string;
};

export type SuccessPayload = Record<string, never>;
