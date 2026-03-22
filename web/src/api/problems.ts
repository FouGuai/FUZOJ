import { http, unwrapResponse } from "./http";
import type {
  ApiResponse,
  CompleteUploadPayload,
  CreateProblemPayload,
  LatestMetaPayload,
  PrepareUploadPayload,
  ProblemListData,
  SignPartsPayload,
  StatementPayload,
} from "./types";

export async function listProblems(params: { limit?: number; cursor?: string }) {
  const response = await http.get<ApiResponse<ProblemListData>>("/api/v1/problems", { params });
  return unwrapResponse(response);
}

export async function getStatement(problemId: number) {
  const response = await http.get<ApiResponse<StatementPayload>>(`/api/v1/problems/${problemId}/statement`);
  return unwrapResponse(response);
}

export async function getLatest(problemId: number) {
  const response = await http.get<ApiResponse<LatestMetaPayload>>(`/api/v1/problems/${problemId}/latest`);
  return unwrapResponse(response);
}

export async function createProblem(payload: { title: string; owner_id: number }) {
  const response = await http.post<ApiResponse<CreateProblemPayload>>("/api/v1/problems", payload);
  return unwrapResponse(response);
}

export async function updateStatement(problemId: number, version: number, statementMd: string) {
  await http.put(`/api/v1/problems/${problemId}/versions/${version}/statement`, {
    statement_md: statementMd,
  });
}

export async function prepareUpload(
  problemId: number,
  payload: {
    expected_size_bytes: number;
    expected_sha256: string;
    content_type: string;
    created_by: number;
    client_type: string;
    upload_strategy: string;
  },
  idempotencyKey: string,
) {
  const response = await http.post<ApiResponse<PrepareUploadPayload>>(
    `/api/v1/problems/${problemId}/data-pack/uploads:prepare`,
    payload,
    {
      headers: {
        "Idempotency-Key": idempotencyKey,
      },
    },
  );
  return unwrapResponse(response);
}

export async function signParts(problemId: number, uploadId: number, partNumbers: number[]) {
  const response = await http.post<ApiResponse<SignPartsPayload>>(
    `/api/v1/problems/${problemId}/data-pack/uploads/${uploadId}/sign`,
    { part_numbers: partNumbers },
  );
  return unwrapResponse(response);
}

export async function completeUpload(
  problemId: number,
  uploadId: number,
  payload: {
    parts: Array<{ part_number: number; etag: string }>;
    manifest_json: string;
    config_json: string;
    manifest_hash: string;
    data_pack_hash: string;
  },
) {
  const response = await http.post<ApiResponse<CompleteUploadPayload>>(
    `/api/v1/problems/${problemId}/data-pack/uploads/${uploadId}/complete`,
    payload,
  );
  return unwrapResponse(response);
}

export async function abortUpload(problemId: number, uploadId: number) {
  await http.post(`/api/v1/problems/${problemId}/data-pack/uploads/${uploadId}/abort`);
}

export async function publishVersion(problemId: number, version: number) {
  await http.post(`/api/v1/problems/${problemId}/versions/${version}/publish`);
}
