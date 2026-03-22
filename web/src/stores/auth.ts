import { computed, ref } from "vue";
import { defineStore } from "pinia";
import * as authApi from "@/api/auth";
import type { AuthPayload, UserInfo } from "@/api/types";

const STORAGE_KEY = "fuzoj-web-auth";

type SessionState = {
  accessToken: string;
  refreshToken: string;
  accessExpiresAt: string;
  refreshExpiresAt: string;
  user: UserInfo | null;
};

type RawUser = Partial<UserInfo> & {
  Id?: number | string;
  Username?: string;
  Role?: string;
};

function emptySession(): SessionState {
  return {
    accessToken: "",
    refreshToken: "",
    accessExpiresAt: "",
    refreshExpiresAt: "",
    user: null,
  };
}

function normalizeUser(user: RawUser | null | undefined): UserInfo | null {
  if (!user) {
    return null;
  }
  const idRaw = user.id ?? user.Id;
  const id = typeof idRaw === "string" ? Number(idRaw) : idRaw;
  const username = user.username ?? user.Username;
  const role = user.role ?? user.Role;
  if (!id || Number.isNaN(id) || id <= 0 || !username || !role) {
    return null;
  }
  return {
    id,
    username,
    role,
  };
}

export const useAuthStore = defineStore("auth", () => {
  const initialized = ref(false);
  const session = ref<SessionState>(emptySession());

  const accessToken = computed(() => session.value.accessToken);
  const refreshToken = computed(() => session.value.refreshToken);
  const user = computed(() => session.value.user);
  const isAuthenticated = computed(() => Boolean(session.value.accessToken && session.value.user));

  function initialize() {
    if (initialized.value || typeof window === "undefined") {
      return;
    }

    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw) {
      try {
        const parsed = JSON.parse(raw) as SessionState & { user?: RawUser | null };
        session.value = {
          ...emptySession(),
          ...parsed,
          user: normalizeUser(parsed.user),
        };
      } catch {
        session.value = emptySession();
      }
    }

    initialized.value = true;
  }

  function persist() {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(session.value));
  }

  function applyPayload(payload: AuthPayload) {
    session.value = {
      accessToken: payload.access_token,
      refreshToken: payload.refresh_token,
      accessExpiresAt: payload.access_expires_at,
      refreshExpiresAt: payload.refresh_expires_at,
      user: normalizeUser(payload.user),
    };
    persist();
  }

  async function login(username: string, password: string) {
    const payload = await authApi.login({ username, password });
    applyPayload(payload);
    return payload;
  }

  async function register(username: string, password: string) {
    const payload = await authApi.register({ username, password });
    applyPayload(payload);
    return payload;
  }

  async function refreshSession() {
    if (!session.value.refreshToken) {
      throw new Error("Refresh token missing");
    }
    const payload = await authApi.refreshToken(session.value.refreshToken);
    applyPayload(payload);
  }

  async function logout() {
    if (session.value.refreshToken) {
      await authApi.logout(session.value.refreshToken);
    }
    clearSession();
  }

  function clearSession() {
    session.value = emptySession();
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(STORAGE_KEY);
    }
  }

  return {
    accessToken,
    refreshToken,
    user,
    isAuthenticated,
    initialize,
    login,
    register,
    refreshSession,
    logout,
    clearSession,
  };
});
