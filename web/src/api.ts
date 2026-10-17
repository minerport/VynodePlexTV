export interface Session {
  user: { id: string; username: string; role: "OWNER" | "ADMIN" | "USER" };
  csrfToken: string;
  expiresAt: number;
}
export interface Settings {
  displayName: string;
  timezone: string;
}
export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

export async function bootstrap(): Promise<{
  state: "setup" | "login" | "ready";
  session: Session | null;
}> {
  const status = await request<{ setupRequired: boolean }>("/auth/status");
  if (status.setupRequired) return { state: "setup", session: null };
  try {
    return { state: "ready", session: await request<Session>("/auth/session") };
  } catch (error) {
    if (error instanceof APIError && error.status === 401)
      return { state: "login", session: null };
    throw error;
  }
}
export async function request<T>(
  path: string,
  method = "GET",
  body?: unknown,
  csrf?: string,
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/v1${path}`, {
      method,
      credentials: "same-origin",
      headers: {
        ...(body ? { "Content-Type": "application/json" } : {}),
        ...(csrf ? { "X-CSRF-Token": csrf } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new APIError(
      0,
      "Cannot reach Afterglow. Check your connection and try again.",
    );
  }
  const data = await response.json();
  if (!response.ok) {
    if (response.status === 401 && path !== "/auth/login")
      window.dispatchEvent(new Event("session-expired"));
    throw new APIError(
      response.status,
      data.error || "Unable to complete this request.",
    );
  }
  return data as T;
}
