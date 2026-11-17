import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { ApiError, api, clearTokens, getAccessToken, setTokens } from "../../lib/api";

interface TokensRes {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

interface RegisterPayload {
  name: string;
  email: string;
  phone?: string;
  password: string;
  school_name: string;
  school_code: string;
  level: string;
}

interface AuthContextValue {
  authenticated: boolean;
  /** Bekerja (memuat / mencoba sesi ulang) saat aplikasi pertama dibuka. */
  booting: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (payload: RegisterPayload) => Promise<{ verify_token?: string }>;
  verifyEmail: (token: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [authenticated, setAuthenticated] = useState(() => getAccessToken() !== null);
  const [booting, setBooting] = useState(() => getAccessToken() === null && hasRefresh());

  useEffect(() => {
    // Saat reload dengan refresh token tersimpan: coba salurkan sesi baru.
    if (getAccessToken() !== null || !hasRefresh()) return;
    (async () => {
      try {
        const tok = await api<TokensRes>("/auth/refresh", {
          method: "POST",
          body: { refresh_token: localStorage.getItem("flowcbt:refresh") },
          skipAuth: true,
          retry: false,
        });
        setTokens(tok.access_token, tok.refresh_token);
      } catch {
        clearTokens();
      } finally {
        setAuthenticated(getAccessToken() !== null);
        setBooting(false);
      }
    })();
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const tok = await api<TokensRes>("/auth/login", {
      method: "POST",
      body: { email, password },
      skipAuth: true,
    });
    setTokens(tok.access_token, tok.refresh_token);
    setAuthenticated(true);
  }, []);

  const register = useCallback(async (payload: RegisterPayload) => {
    return api<{ verify_token?: string }>("/auth/register", {
      method: "POST",
      body: payload,
      skipAuth: true,
    });
  }, []);

  const verifyEmail = useCallback(async (token: string) => {
    await api<{ message: string }>("/auth/verify-email", {
      method: "POST",
      body: { token },
      skipAuth: true,
    });
  }, []);

  const logout = useCallback(() => {
    clearTokens();
    setAuthenticated(false);
  }, []);

  return (
    <AuthContext.Provider value={{ authenticated, booting, login, register, verifyEmail, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

function hasRefresh() {
  return localStorage.getItem("flowcbt:refresh") !== null;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth harus dipakai di dalam AuthProvider");
  return ctx;
}

export const errorOf = (e: unknown): string =>
  e instanceof ApiError ? e.message : "Terjadi kesalahan. Coba lagi.";
