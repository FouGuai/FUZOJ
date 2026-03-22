import { http, unwrapResponse } from "./http";
import type { ApiResponse, JudgeStatusData, SourceData, SubmissionCreatePayload } from "./types";

export async function createSubmission(payload: {
  problem_id: number;
  user_id: number;
  language_id: string;
  source_code: string;
  contest_id: string;
  scene: string;
  extra_compile_flags: string[];
}, idempotencyKey?: string) {
  const response = await http.post<ApiResponse<SubmissionCreatePayload>>("/api/v1/submissions", payload, {
    headers: idempotencyKey
      ? {
          "Idempotency-Key": idempotencyKey,
        }
      : undefined,
  });
  return unwrapResponse(response);
}

export async function getSubmissionStatus(submissionId: string, include: "details" | "log" | "" = "details") {
  const response = await http.get<ApiResponse<JudgeStatusData>>(`/api/v1/status/submissions/${submissionId}`, {
    params: include ? { include } : undefined,
  });
  return unwrapResponse(response);
}

export async function getSubmissionSource(submissionId: string) {
  const response = await http.get<ApiResponse<SourceData>>(`/api/v1/submissions/${submissionId}/source`);
  return unwrapResponse(response);
}
