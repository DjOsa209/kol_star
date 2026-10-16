export type IPContentTranslation = {
  state: "none" | "pending" | "completed" | "failed" | "unavailable";
  total?: number;
  pending?: number;
  failed?: number;
  values?: Record<string, Record<string, string>>;
};
export type IPTranslationUpdate = {
  type: string;
  id: number;
  translation: IPContentTranslation;
};
export type IPTranslatedRecord = {
  id?: number | string;
  translation?: IPContentTranslation;
  [key: string]: any;
};

/** Read-only projection. Editing and saving always use the untouched original record. */
export function translatedIPRecord<T extends IPTranslatedRecord>(
  record: T,
  language: string,
  original = false
): T {
  const values =
    record?.translation?.values?.[language.startsWith("en") ? "en" : "zh"];
  if (!values || original) return record;
  const copy: T = JSON.parse(JSON.stringify(record));
  for (const [path, text] of Object.entries(values)) {
    const keys = path.split(".");
    if (
      keys.some(key =>
        ["__proto__", "prototype", "constructor", "translation"].includes(key)
      )
    )
      continue;
    let parent: any = copy;
    for (const key of keys.slice(0, -1)) {
      if (typeof parent[key] === "string") {
        try {
          parent[key] = JSON.parse(parent[key]);
        } catch {
          parent = null;
          break;
        }
      }
      parent = parent[key];
      if (!parent || typeof parent !== "object") break;
    }
    const key = keys[keys.length - 1];
    if (parent && typeof parent[key] === "string") parent[key] = text;
  }
  return copy;
}
export function applyIPTranslationUpdates(
  records: IPTranslatedRecord[],
  kind: string,
  updates: IPTranslationUpdate[]
) {
  for (const record of records) {
    const update = updates.find(
      update => update.type === kind && Number(update.id) === Number(record.id)
    );
    if (update) record.translation = update.translation;
  }
}
