export function buildRecentCooperationWorks(
  cooperations: any[],
  posts: any[],
  limit = 3
) {
  const normalizedPosts = posts.map(post => ({
    post,
    key: normalizedWorkUrl(post.postUrl)
  }));

  return cooperations
    .map(cooperation => {
      const links = cooperationLinks(cooperation);
      const linkKeys = new Set(links.map(normalizedWorkUrl).filter(Boolean));
      const matchedPost = normalizedPosts.find(
        item => item.key && linkKeys.has(item.key)
      )?.post;
      const coverUrl =
        matchedPost?.coverUrl ||
        cooperation.contentCoverUrl ||
        cooperation.contentCoverLocalUrl ||
        cooperation.contentCoverRemoteUrl ||
        "";
      const postUrl = matchedPost?.postUrl || links[0] || "";

      if (!coverUrl && !postUrl) return null;

      return {
        ...matchedPost,
        id: `cooperation-${cooperation.id}`,
        cooperationId: cooperation.id,
        postUrl,
        coverUrl,
        title:
          matchedPost?.title ||
          cooperation.creativeName ||
          cooperation.projectName ||
          "",
        publishedAt:
          matchedPost?.publishedAt ||
          cooperation.publishTime ||
          cooperation.releaseDate ||
          cooperation.updatedAt ||
          0,
        durationSeconds: matchedPost?.durationSeconds || 0,
        viewCount:
          matchedPost?.viewCount ??
          cooperation.views ??
          cooperation.impressions ??
          0,
        interactionCount:
          matchedPost == null
            ? (cooperation.engagementCount ?? cooperation.commentsCount ?? 0)
            : undefined,
        sortTime: workTimestamp(
          matchedPost?.publishedAt ||
            cooperation.publishTime ||
            cooperation.releaseDate ||
            cooperation.updatedAt
        )
      };
    })
    .filter(Boolean)
    .sort((left: any, right: any) => right.sortTime - left.sortTime)
    .slice(0, limit);
}

function cooperationLinks(cooperation: any) {
  return [cooperation.finalLink, cooperation.deliverableLinks]
    .flatMap(value => String(value || "").split(/[\n,;]/))
    .map(value => value.trim())
    .filter(Boolean);
}

function normalizedWorkUrl(value: unknown) {
  const raw = String(value || "").trim();
  if (!raw) return "";
  try {
    const url = new URL(raw);
    const host = url.hostname.toLowerCase().replace(/^www\./, "");
    const path = url.pathname.replace(/\/+$/, "");
    return `${host}${path}`.toLowerCase();
  } catch {
    return raw
      .replace(/[?#].*$/, "")
      .replace(/\/+$/, "")
      .toLowerCase();
  }
}

function workTimestamp(value: unknown) {
  if (typeof value === "number") return value;
  const parsed = Date.parse(String(value || ""));
  return Number.isNaN(parsed) ? 0 : parsed;
}
