import { NextRequest, NextResponse } from "next/server";

function randomRequestId8(): string {
  try {
    return crypto.randomUUID().replace(/-/g, "").slice(0, 12);
  } catch {
    return `req${Math.random().toString(36).slice(2, 10)}`;
  }
}

/** Same default as `next.config.ts` rewrites — always the Go server, never the Next dev port. */
export function backendOriginForProxy(): string {
  return (process.env.REMOTE_API_URL || "http://localhost:8080").replace(/\/+$/, "");
}

/**
 * POST JSON to the Go API from a Route Handler (browser hits Next first; we forward to :8080).
 */
export async function proxyPostToBackend(
  req: NextRequest,
  backendPath: string,
): Promise<NextResponse> {
  const origin = backendOriginForProxy();
  const body = await req.text();

  const headers = new Headers();
  headers.set("Content-Type", req.headers.get("content-type") ?? "application/json");
  const authHeader = req.headers.get("authorization");
  if (authHeader) headers.set("Authorization", authHeader);
  const wid = req.headers.get("x-workspace-id");
  if (wid) headers.set("X-Workspace-ID", wid);
  const cookie = req.headers.get("cookie");
  if (cookie) headers.set("Cookie", cookie);
  headers.set("X-Request-ID", req.headers.get("x-request-id") ?? randomRequestId8());

  const res = await fetch(`${origin}${backendPath}`, {
    method: "POST",
    headers,
    body: body || "{}",
    redirect: "manual",
  });

  const text = await res.text();
  const out = new NextResponse(text, { status: res.status });
  const ct = res.headers.get("content-type");
  if (ct) out.headers.set("Content-Type", ct);

  const cookies = typeof res.headers.getSetCookie === "function" ? res.headers.getSetCookie() : [];
  for (const c of cookies) {
    out.headers.append("Set-Cookie", c);
  }

  return out;
}
