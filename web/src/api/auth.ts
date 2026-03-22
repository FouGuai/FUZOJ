import { http, unwrapResponse } from "./http";
import type { ApiResponse, AuthPayload } from "./types";

export async function login(payload: { username: string; password: string }) {
  const response = await http.post<ApiResponse<AuthPayload>>("/api/v1/user/login", payload);
  return unwrapResponse(response);
}

export async function register(payload: { username: string; password: string }) {
  const response = await http.post<ApiResponse<AuthPayload>>("/api/v1/user/register", payload);
  return unwrapResponse(response);
}

export async function refreshToken(refreshTokenValue: string) {
  const response = await http.post<ApiResponse<AuthPayload>>("/api/v1/user/refresh-token", {
    refresh_token: refreshTokenValue,
  });
  return unwrapResponse(response);
}

export async function logout(refreshTokenValue: string) {
  await http.post("/api/v1/user/logout", {
    refresh_token: refreshTokenValue,
  });
}
