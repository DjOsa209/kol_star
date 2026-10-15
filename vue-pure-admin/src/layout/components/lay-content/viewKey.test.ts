import assert from "node:assert/strict";
import test from "node:test";
// @ts-expect-error -- Native Node tests resolve the TypeScript source directly.
import { contentViewKey } from "./viewKey.ts";

test("IP session/result navigation preserves the workspace instance", () => {
  const path = "/business/ip/recommendations";
  assert.equal(
    contentViewKey(
      { path, query: { subscriptionId: "1", resultId: "11" } },
      `${path}?subscriptionId=1&resultId=11`
    ),
    contentViewKey(
      { path, query: { subscriptionId: "2", resultId: "22" } },
      `${path}?subscriptionId=2&resultId=22`
    )
  );
});
test("different project draft contexts still create distinct workspace instances", () => {
  const path = "/business/ip/recommendations";
  assert.notEqual(
    contentViewKey({ path, query: { requestId: "1" } }, ""),
    contentViewKey({ path, query: { requestId: "2" } }, "")
  );
  assert.notEqual(
    contentViewKey({ path, query: { selectionKey: "draft-a" } }, ""),
    contentViewKey({ path, query: { selectionKey: "draft-b" } }, "")
  );
  assert.equal(
    contentViewKey({ path, query: { requestId: "1", selectionKey: "a" } }, ""),
    contentViewKey({ path, query: { selectionKey: "a", requestId: "1" } }, "")
  );
});
test("other routes retain their existing full URL identity", () => {
  assert.equal(
    contentViewKey(
      { path: "/business/ip/requests", query: { id: "1" } },
      "/business/ip/requests?id=1"
    ),
    "/business/ip/requests?id=1"
  );
});
