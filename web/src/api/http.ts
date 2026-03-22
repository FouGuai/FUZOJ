import axios, { AxiosError } from "axios";
import router from "@/router";
import { useAuthStore } from "@/stores/auth";
import type { ApiResponse } from "./types";

const apiBaseURL = import.meta.env.VITE_API_BASE_URL || "/";

export const http = axios.create({
  baseURL: apiBaseURL,
  timeout: 15000,
});

let refreshPromise: Promise<void> | null = null;

http.interceptors.request.use((config) => {
  const authStore = useAuthStore();
  authStore.initialize();
  config.headers = config.headers || {};

  if (authStore.accessToken) {
    config.headers.Authorization = `Bearer ${authStore.accessToken}`;
  }

  return config;
});

http.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<ApiResponse<unknown>>) => {
    const authStore = useAuthStore();
    const originalRequest = error.config;
    const requestHeaders = originalRequest?.headers;
    const hasRetried =
      typeof requestHeaders?.toJSON === "function"
        ? Boolean((requestHeaders.toJSON() as Record<string, unknown>)["x-retried"])
        : false;

    if (error.response?.status === 401 && originalRequest && !hasRetried) {
      if (!authStore.refreshToken) {
        authStore.clearSession();
        await router.push("/login");
        return Promise.reject(normalizeApiError(error));
      }

      originalRequest.headers.set("x-retried", "1");

      refreshPromise ??= authStore
        .refreshSession()
        .catch(async (refreshError: unknown) => {
          authStore.clearSession();
          await router.push("/login");
          throw refreshError;
        })
        .finally(() => {
          refreshPromise = null;
        });

      await refreshPromise;
      return http.request(originalRequest);
    }

    return Promise.reject(normalizeApiError(error));
  },
);

export function unwrapResponse<T>(response: { data: ApiResponse<T> }): T {
  return response.data.data;
}

export function normalizeApiError(error: unknown): Error {
  if (axios.isAxiosError<ApiResponse<unknown>>(error)) {
    const details = error.response?.data?.details;
    const detailsText =
      details && Object.keys(details).length > 0
        ? Object.entries(details)
            .map(([key, value]) => `${key}: ${value}`)
            .join("; ")
        : "";
    const message =
      (detailsText ? `${error.response?.data?.message || "Request failed"} (${detailsText})` : "") ||
      error.response?.data?.message ||
      error.message ||
      "Request failed";
    return new Error(message);
  }

  return error instanceof Error ? error : new Error("Unknown error");
}
