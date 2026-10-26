/** One URL per line; blank lines do not create entries. */
export function parseIPAssetLinks(value: unknown): string[] {
  return typeof value === "string"
    ? value
        .split(/\r?\n/)
        .map(line => line.trim())
        .filter(Boolean)
    : [];
}

export function isIPAssetLinkURL(value: string): boolean {
  if (!/^https?:\/\//i.test(value) || /^https?:\/\/[^/?#]*@/i.test(value))
    return false;
  if (value.length > 2048 || /[\s\u0000-\u001f\u007f\\]/u.test(value))
    return false;
  try {
    const url = new URL(value);
    return (
      ["http:", "https:"].includes(url.protocol) &&
      !!url.hostname &&
      !url.username &&
      !url.password
    );
  } catch {
    return false;
  }
}

export function ipAssetLinksIssue(value: unknown): string {
  if (value === undefined) return "";
  if (typeof value !== "string") return "素材链接应为文本，每行填写一个URL";
  const links = parseIPAssetLinks(value);
  if (links.length > 20) return "最多填写20个素材链接";
  return links.every(isIPAssetLinkURL)
    ? ""
    : "素材链接请输入有效的HTTP或HTTPS地址，每行一个，单条不超过2048个字符";
}
