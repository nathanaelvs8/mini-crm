import type {
  AuthResponse,
  Customer,
  CustomerInput,
  Interaction,
  Kind,
  Stage,
  Stats,
} from "./types";

// Leave VITE_API_URL empty in development (the Vite proxy handles /api).
// In production set it to the backend's URL, e.g. https://my-crm-api.onrender.com
const BASE = import.meta.env.VITE_API_URL ?? "";

const TOKEN_KEY = "crm_token";

export const tokenStore = {
  get: () => localStorage.getItem(TOKEN_KEY),
  set: (t: string) => localStorage.setItem(TOKEN_KEY, t),
  clear: () => localStorage.removeItem(TOKEN_KEY),
};

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

// Called when the server says our token is no longer valid, so the app can log out.
let onUnauthorized: () => void = () => {};
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const token = tokenStore.get();
  if (token) headers["Authorization"] = `Bearer ${token}`;

  let res: Response;
  try {
    res = await fetch(BASE + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "Cannot reach the server. Is the backend running?");
  }

  if (res.status === 204) return undefined as T;

  const data = await res.json().catch(() => null);
  if (!res.ok) {
    // 401 on login means "wrong password", not "session expired", so only log out when we had a token.
    if (res.status === 401 && token) onUnauthorized();
    throw new ApiError(res.status, data?.error ?? `Request failed (${res.status})`);
  }
  return data as T;
}

export const api = {
  register: (name: string, email: string, password: string) =>
    request<AuthResponse>("POST", "/api/auth/register", { name, email, password }),
  login: (email: string, password: string) =>
    request<AuthResponse>("POST", "/api/auth/login", { email, password }),

  stats: () => request<Stats>("GET", "/api/stats"),

  listCustomers: (q: string, stage: string) => {
    const params = new URLSearchParams();
    if (q) params.set("q", q);
    if (stage) params.set("stage", stage);
    const qs = params.toString();
    return request<Customer[]>("GET", "/api/customers" + (qs ? `?${qs}` : ""));
  },
  getCustomer: (id: number) => request<Customer>("GET", `/api/customers/${id}`),
  createCustomer: (input: CustomerInput) => request<Customer>("POST", "/api/customers", input),
  updateCustomer: (id: number, input: CustomerInput) =>
    request<Customer>("PUT", `/api/customers/${id}`, input),
  deleteCustomer: (id: number) => request<void>("DELETE", `/api/customers/${id}`),
  setStage: (id: number, stage: Stage) =>
    request<Customer>("PATCH", `/api/customers/${id}/stage`, { stage }),

  listInteractions: (customerId: number) =>
    request<Interaction[]>("GET", `/api/customers/${customerId}/interactions`),
  addInteraction: (customerId: number, kind: Kind, content: string) =>
    request<Interaction>("POST", `/api/customers/${customerId}/interactions`, { kind, content }),
  deleteInteraction: (id: number) => request<void>("DELETE", `/api/interactions/${id}`),
};
