export type ResultFilter = "all" | "creator" | "media";
export type ResultSort = "match" | "cost";

export function isMediaResource(item: Record<string, any>) {
  return /媒体|media|publisher|publication/i.test(
    String(item.resourceType || "")
  );
}

export function matchScore(item: Record<string, any>): number | null {
  if (
    item.matchScore === null ||
    item.matchScore === undefined ||
    item.matchScore === ""
  )
    return null;
  const value = Number(item.matchScore);
  return Number.isFinite(value) ? Math.min(100, Math.max(0, value)) : null;
}

export function visibleRecommendations<T extends Record<string, any>>(
  items: T[],
  filter: ResultFilter,
  sort: ResultSort
): T[] {
  return items
    .filter(
      item =>
        filter === "all" ||
        (filter === "media" ? isMediaResource(item) : !isMediaResource(item))
    )
    .sort((left, right) => {
      if (sort === "cost") {
        const cost = (item: T) =>
          Number(item.estimatedCost) > 0
            ? Number(item.estimatedCost)
            : Infinity;
        const difference = cost(left) - cost(right);
        if (!Number.isNaN(difference) && difference !== 0) return difference;
      }
      return (matchScore(right) ?? -1) - (matchScore(left) ?? -1);
    });
}

export function recommendationReasons(item: Record<string, any>): string[] {
  const values =
    Array.isArray(item.hitRules) && item.hitRules.length
      ? item.hitRules
      : String(item.reason || "").split(/[；;\n]/);
  return values.map(value => String(value).trim()).filter(Boolean);
}
