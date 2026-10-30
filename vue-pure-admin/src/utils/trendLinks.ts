// Build content links from identifiers, never from uploaded arbitrary URLs.
export function trendTikTokPlayerURL(itemID: unknown): string {
  if (typeof itemID !== "string" || !/^[0-9]{1,24}$/.test(itemID)) return "";
  return `https://www.tiktok.com/player/v1/${itemID}?music_info=1&description=0&autoplay=1`;
}

export function trendTikTokVideoURL(itemID: unknown, handle?: unknown): string {
  if (typeof itemID !== "string" || !/^[0-9]{1,24}$/.test(itemID)) return "";
  const username =
    typeof handle === "string" ? handle.trim().replace(/^@/, "") : "";
  if (/^[A-Za-z0-9._]{1,24}$/.test(username)) {
    return `https://www.tiktok.com/@${username}/video/${itemID}`;
  }
  // Tag detail samples contain an item ID but may omit the creator handle.
  return `https://www.tiktok.com/share/video/${itemID}`;
}

export function trendTikTokTagURL(name: unknown): string {
  if (typeof name !== "string") return "";
  const tag = name.trim().replace(/^#/, "");
  if (!tag || tag === "." || tag === "..") return "";
  try {
    return `https://www.tiktok.com/tag/${encodeURIComponent(tag)}`;
  } catch {
    return "";
  }
}
