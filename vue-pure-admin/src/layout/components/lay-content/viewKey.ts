// Session/result selection changes content inside the IP workspace. Other
// query parameters still identify distinct contexts, as do all other pages.
export function contentViewKey(
  route: { path: string; query: Record<string, unknown>; hash?: string },
  fullPath: string
) {
  if (route.path !== "/business/ip/recommendations") return fullPath;
  const context = Object.entries(route.query)
    .filter(([key]) => !["subscriptionId", "resultId"].includes(key))
    .sort(([a], [b]) => a.localeCompare(b));
  return `${route.path}:${JSON.stringify(context)}:${route.hash || ""}`;
}
