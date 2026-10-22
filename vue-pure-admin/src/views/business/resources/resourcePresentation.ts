export type RecentMetricKey = "avgViews" | "avgInteractions" | "engagementRate";

export interface ResourceTagReference {
  id: number;
  name: string;
  status?: string;
}

export function resolveResourceTagIds(
  name: string,
  originalTags: ResourceTagReference[],
  enabledOptions: ResourceTagReference[]
): number[] {
  const normalized = name.trim().toLowerCase();
  if (!normalized) return [];
  const matches = (tag: ResourceTagReference) =>
    tag.name.trim().toLowerCase() === normalized &&
    Number.isSafeInteger(Number(tag.id)) &&
    Number(tag.id) > 0;
  // Previously selected tags may now be disabled. Preserve their identity
  // without offering those tags as new choices or recreating the same name.
  const originalIds = originalTags.filter(matches).map(tag => Number(tag.id));
  if (originalIds.length) return [...new Set(originalIds)];
  const option = enabledOptions.find(
    tag => tag.status === "启用" && matches(tag)
  );
  return option ? [Number(option.id)] : [];
}

export function buildTopicTags(
  rows: Array<{ title?: unknown; description?: unknown }>
) {
  if (!rows.length) return [];
  const counts = new Map<string, number>();
  rows.forEach(post => {
    const source = `${post.title || ""} ${post.description || ""}`;
    const postTags = new Set(
      Array.from(source.matchAll(/[#＃]([\p{L}\p{N}_-]{2,30})/gu), match =>
        match[1].trim().toLowerCase()
      )
    );
    postTags.forEach(tag => counts.set(tag, (counts.get(tag) || 0) + 1));
  });
  return Array.from(counts.entries())
    .sort((left, right) => right[1] - left[1])
    .slice(0, 5)
    .map(([name, count], index) => ({
      name,
      count,
      share: (count / rows.length) * 100,
      scale: 1.45 - index * 0.13
    }));
}

export function recentSampleCount(row: any): number {
  const metrics = row?.recentMetrics;
  if (metrics?.source !== "collected_posts" || metrics.windowDays !== 30)
    return 0;
  const count = Number(metrics.sampleCount);
  return Number.isFinite(count) && count > 0 ? Math.floor(count) : 0;
}

export function recentMetric(row: any, key: RecentMetricKey): number | null {
  if (recentSampleCount(row) === 0) return null;
  const raw = row.recentMetrics[key];
  if (raw === null || raw === undefined || raw === "") return null;
  const value = Number(raw);
  return Number.isFinite(value) && value >= 0 ? value : null;
}

export function sampleMetricText(
  row: any,
  key: RecentMetricKey,
  locale = "zh-CN"
) {
  const value = recentMetric(row, key);
  if (value === null) return locale === "en" ? "Unknown" : "未知";
  if (key === "engagementRate") return `${(value * 100).toFixed(2)}%`;
  return new Intl.NumberFormat(locale === "en" ? "en-US" : "zh-CN", {
    notation: "compact",
    maximumFractionDigits: 1
  }).format(value);
}

export function sampleSummary(row: any, locale = "zh-CN") {
  const count = recentSampleCount(row);
  return locale === "en"
    ? `Collected posts from the last 30 days · ${count} samples`
    : `近30天已采集作品样本 · ${count} 条`;
}

export function resourceAvatarCandidates(row: any): string[] {
  const urls: string[] = [];
  for (const value of [row?.avatarUrl, row?.avatarRemoteUrl]) {
    const url = typeof value === "string" ? value.trim() : "";
    if (!url) continue;
    if (/^\/api\/uploads\/resource-images\//.test(url)) {
      urls.push(url);
      continue;
    }
    try {
      const parsed = new URL(url);
      if (
        ["https:", "http:"].includes(parsed.protocol) &&
        !parsed.username &&
        !parsed.password
      )
        urls.push(parsed.href);
    } catch {
      // Invalid and unsafe URLs leave the existing initials placeholder visible.
    }
  }
  return [...new Set(urls)];
}
