/**
 * 8-character hex id for X-Request-ID.
 * `crypto.randomUUID()` is unavailable in non-secure contexts (e.g. plain http://), so we fall back to getRandomValues.
 */
export function randomRequestId8(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === "function") {
    return c.randomUUID().slice(0, 8);
  }
  if (c && typeof c.getRandomValues === "function") {
    const buf = new Uint8Array(4);
    c.getRandomValues(buf);
    let hex = "";
    for (let i = 0; i < buf.length; i++) {
      hex += buf[i]!.toString(16).padStart(2, "0");
    }
    return hex;
  }
  return Math.random().toString(16).slice(2, 10).padEnd(8, "0");
}
