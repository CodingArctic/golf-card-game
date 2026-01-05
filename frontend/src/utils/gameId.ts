/**
 * Formats a full UUID into a short display format
 * Shows only the first 8 characters of the UUID for display purposes
 * @param publicId - The full UUID string
 * @returns The shortened UUID (first 8 chars) or empty string if invalid
 * @example
 * shortGameId("a3f4b2c1-1234-5678-9abc-def012345678") // Returns "a3f4b2c1"
 */
export function shortGameId(publicId: string | undefined | null): string {
  if (!publicId) return "";
  return publicId.substring(0, 8);
}

/**
 * Formats a game ID for display in the UI
 * @param publicId - The full UUID string
 * @returns A formatted string like "Game #a3f4b2c1"
 */
export function formatGameId(publicId: string | undefined | null): string {
  const short = shortGameId(publicId);
  return short ? `Game #${short}` : "Game";
}
