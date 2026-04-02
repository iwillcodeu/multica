/** Display name units: Han = 2, other BMP runes = 1. Max allowed total is 12. */
export function displayNameUnits(s: string): number {
  let units = 0;
  for (const ch of s) {
    const code = ch.codePointAt(0) ?? 0;
    // CJK Unified Ideographs (basic multilingual plane)
    const isHan = code >= 0x4e00 && code <= 0x9fff;
    units += isHan ? 2 : 1;
  }
  return units;
}

export function validateDisplayNameInput(s: string): string | null {
  const t = s.trim();
  if (!t) return "Name is required";
  if (displayNameUnits(t) > 20) {
    return "Name is too long (max 20 units; each Chinese character counts as 2)";
  }
  return null;
}
