import assert from "node:assert/strict";
import test from "node:test";
import {
  trendTikTokPlayerURL,
  trendTikTokTagURL,
  trendTikTokVideoURL
} from "../src/utils/trendLinks.ts";

test("video links support both ranked videos and samples without a handle", () => {
  const id = "6718335390845095173";
  assert.equal(
    trendTikTokVideoURL(id, "@creator"),
    `https://www.tiktok.com/@creator/video/${id}`
  );
  assert.equal(
    trendTikTokVideoURL(id),
    `https://www.tiktok.com/share/video/${id}`
  );
  assert.equal(
    trendTikTokVideoURL(id, "//evil.test"),
    `https://www.tiktok.com/share/video/${id}`
  );
});

test("untrusted or imprecise video identifiers cannot become links", () => {
  for (const value of [
    undefined,
    null,
    {},
    6718335390845095173,
    "",
    "1".repeat(25),
    "123?next=https://evil.test",
    "javascript:alert(1)",
    "123/../../evil"
  ]) {
    assert.equal(trendTikTokVideoURL(value), "");
  }
});

test("hashtag names are encoded inside a fixed TikTok content URL", () => {
  assert.equal(
    trendTikTokTagURL(" #音乐 "),
    "https://www.tiktok.com/tag/%E9%9F%B3%E4%B9%90"
  );
  const url = new URL(trendTikTokTagURL("x/?next=https://evil.test#fragment"));
  assert.equal(url.origin, "https://www.tiktok.com");
  assert.equal(url.search, "");
  assert.equal(url.hash, "");
  for (const value of [undefined, {}, "", " # ", ".", "..", "\ud800"]) {
    assert.equal(trendTikTokTagURL(value), "");
  }
});

test("official player links reject non-video identifiers", () => {
  assert.equal(
    new URL(trendTikTokPlayerURL("6718335390845095173")).hostname,
    "www.tiktok.com"
  );
  for (const value of [
    undefined,
    "",
    "123/../../evil",
    "javascript:alert(1)",
    "1".repeat(25)
  ]) {
    assert.equal(trendTikTokPlayerURL(value), "");
  }
});
