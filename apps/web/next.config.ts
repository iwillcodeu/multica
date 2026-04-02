import type { NextConfig } from "next";
import { config } from "dotenv";
import { resolve } from "path";

// Load root .env so REMOTE_API_URL is available to next.config.ts
config({ path: resolve(__dirname, "../../.env") });

const remoteApiUrl = process.env.REMOTE_API_URL || "http://localhost:8080";

const nextConfig: NextConfig = {
  images: {
    formats: ["image/avif", "image/webp"],
    qualities: [75, 80, 85],
  },
  async rewrites() {
    // Array form = afterFiles: runs before App Router dynamic routes, so a catch-all
    // `/api/*` would proxy to Go and never hit `app/api/**/route.ts`. Use `fallback`
    // so Route Handlers (e.g. login-password, change-password) run first; unmatched
    // `/api/*` then proxies to the Go server.
    return {
      fallback: [
        {
          source: "/api/:path*",
          destination: `${remoteApiUrl}/api/:path*`,
        },
        {
          source: "/ws",
          destination: `${remoteApiUrl}/ws`,
        },
        {
          source: "/auth/:path*",
          destination: `${remoteApiUrl}/auth/:path*`,
        },
      ],
    };
  },
};

export default nextConfig;
