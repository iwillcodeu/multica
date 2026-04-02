import type { NextRequest } from "next/server";
import { proxyPostToBackend } from "@/lib/backend-proxy";

export async function POST(req: NextRequest) {
  return proxyPostToBackend(req, "/api/auth/login-password");
}
