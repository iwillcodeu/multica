import type { NextConfig } from "next";
import { config } from "dotenv";
import path from "path";
import { fileURLToPath } from "url";

const webPackageDir = path.dirname(fileURLToPath(import.meta.url));

// Load root .env so REMOTE_API_URL is available to next.config.ts
config({ path: path.resolve(webPackageDir, "../../.env") });

const remoteApiUrl = process.env.REMOTE_API_URL || "http://localhost:8080";

/** Mac → Linux upload deploy: avoid sharp/platform-specific image binaries in the standalone bundle. */
const standaloneDeploy = process.env.MULTICA_STANDALONE_DEPLOY === "1";

const nextConfig: NextConfig = {
  ...(standaloneDeploy
    ? {
        output: "standalone" as const,
        outputFileTracingRoot: path.join(webPackageDir, "..", ".."),
      }
    : {}),
  images: {
    unoptimized: standaloneDeploy,
    formats: standaloneDeploy ? undefined : ["image/avif", "image/webp"],
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
