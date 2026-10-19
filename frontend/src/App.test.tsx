import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";

// A tiny fake server: maps "METHOD /path" to a JSON answer. Anything unknown is a 404.
function fakeApi(routes: Record<string, { status?: number; body?: unknown }>) {
  const calls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${url}`;
      calls.push(key);
      const hit = routes[key];
      const status = hit?.status ?? (hit ? 200 : 404);
      return new Response(status === 204 ? null : JSON.stringify(hit?.body ?? { error: "not found" }), { status });
    }),
  );
  return calls;
}

const customer = (id: number, name: string, stage: string) => ({
  id, name, email: "", phone: "", company: "Acme", stage,
  created_at: "2026-10-01T10:00:00Z", updated_at: "2026-10-01T10:00:00Z",
});

const stats = { total: 2, by_stage: { lead: 1, contacted: 0, negotiation: 1, won: 0, lost: 0 } };

beforeEach(() => {
  localStorage.clear();
  window.history.pushState({}, "", "/");
});
afterEach(cleanup);

describe("App", () => {
  it("shows the login page when logged out", () => {
    fakeApi({});
    render(<App />);
    expect(screen.getByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("shows an error for a wrong password and stays on the login page", async () => {
    fakeApi({ "POST /api/auth/login": { status: 401, body: { error: "wrong email or password" } } });
    const user = userEvent.setup();
    render(<App />);

    await user.type(screen.getByLabelText("Email"), "a@b.com");
    await user.type(screen.getByLabelText("Password"), "badpassword");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect((await screen.findByRole("alert")).textContent).toBe("wrong email or password");
    expect(screen.queryByText("Customers")).toBeNull();
  });

  it("logs in, lists customers with their stage, and filters by clicking a stage", async () => {
    const calls = fakeApi({
      "POST /api/auth/login": { body: { token: "t", user: { id: 1, name: "Ana", email: "a@b.com" } } },
      "GET /api/stats": { body: stats },
      "GET /api/customers": { body: [customer(2, "Budi Santoso", "negotiation"), customer(1, "Citra Dewi", "lead")] },
      "GET /api/customers?stage=lead": { body: [customer(1, "Citra Dewi", "lead")] },
    });
    const user = userEvent.setup();
    render(<App />);

    await user.type(screen.getByLabelText("Email"), "a@b.com");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("Budi Santoso")).toBeTruthy();
    expect(screen.getByText("Citra Dewi")).toBeTruthy();
    expect(screen.getByText("Ana")).toBeTruthy(); // user name in the top bar

    // the "lead" tile in the stats row
    await user.click(screen.getByTitle("Show only lead"));
    await waitFor(() => expect(screen.queryByText("Budi Santoso")).toBeNull());
    expect(screen.getByText("Citra Dewi")).toBeTruthy();
    expect(calls).toContain("GET /api/customers?stage=lead");
  });

  it("logs the user out when the server says the token is invalid", async () => {
    localStorage.setItem("crm_token", "old-token");
    localStorage.setItem("crm_user", JSON.stringify({ id: 1, name: "Ana", email: "a@b.com" }));
    fakeApi({
      "GET /api/stats": { status: 401, body: { error: "invalid or expired token" } },
      "GET /api/customers": { status: 401, body: { error: "invalid or expired token" } },
    });
    render(<App />);

    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
    expect(localStorage.getItem("crm_token")).toBeNull();
  });
});
