export function buildRecentCooperationWorks(cooperations: any[], limit = 3) {
  return cooperations
    .map(cooperation => {
      const links = cooperationLinks(cooperation);
      const coverUrl =
        cooperation.contentCoverUrl ||
        cooperation.contentCoverLocalUrl ||
        cooperation.contentCoverRemoteUrl ||
        "";
      const postUrl = links[0] || "";

      if (!coverUrl && !postUrl) return null;

      return {
        id: `cooperation-${cooperation.id}`,
        cooperationId: cooperation.id,
        postUrl,
        coverUrl,
        title: cooperation.creativeName || cooperation.projectName || "",
        publishedAt:
          cooperation.publishTime ||
          cooperation.releaseDate ||
          cooperation.updatedAt ||
          0,
        durationSeconds: 0,
        viewCount: cooperation.views ?? cooperation.impressions ?? 0,
        interactionCount:
          cooperation.engagementCount ?? cooperation.commentsCount ?? 0,
        sortTime: workTimestamp(
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

function workTimestamp(value: unknown) {
  if (typeof value === "number") return value;
  const parsed = Date.parse(String(value || ""));
  return Number.isNaN(parsed) ? 0 : parsed;
}
