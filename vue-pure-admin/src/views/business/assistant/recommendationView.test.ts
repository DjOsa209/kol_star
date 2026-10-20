import assert from "node:assert/strict";
import test from "node:test";
// @ts-expect-error -- Native Node tests resolve TypeScript sources directly.
import * as view from "./recommendationView.ts";
const { matchScore, recommendationReasons, visibleRecommendations } = view;

test("filters media and creators without mutating the original recommendation ranking", () => {
  const rows = [
    { id: 1, resourceType: "KOL", matchScore: 72 },
    { id: 2, resourceType: "媒体", matchScore: 84 },
    { id: 3, resourceType: "创作者", matchScore: 89 },
    { id: 4, resourceType: "Media", matchScore: 0 }
  ];
  assert.deepEqual(
    visibleRecommendations(rows, "creator", "match").map(row => row.id),
    [3, 1]
  );
  assert.deepEqual(
    visibleRecommendations(rows, "media", "match").map(row => row.id),
    [2, 4]
  );
  assert.deepEqual(
    rows.map(row => row.id),
    [1, 2, 3, 4]
  );
});

test("unknown costs sort after quoted candidates and ties retain match order", () => {
  const rows = [
    { id: 1, estimatedCost: 0, matchScore: 100 },
    { id: 2, estimatedCost: 2500, matchScore: 80 },
    { id: 3, estimatedCost: 2500, matchScore: 90 },
    { id: 4, estimatedCost: null, matchScore: 50 },
    { id: 5, estimatedCost: 1500, matchScore: 70 }
  ];
  assert.deepEqual(
    visibleRecommendations(rows, "all", "cost").map(row => row.id),
    [5, 3, 2, 1, 4]
  );
});

test("does not mistake an internal resource score for a recommendation match score", () => {
  assert.equal(matchScore({ score: 95 }), null);
  assert.equal(matchScore({ matchScore: "" }), null);
  assert.equal(matchScore({ matchScore: "invalid" }), null);
  assert.equal(matchScore({ matchScore: 0 }), 0);
  assert.equal(matchScore({ matchScore: 120 }), 100);
});

test("uses actual hit rules or falls back to saved match reasons", () => {
  assert.deepEqual(
    recommendationReasons({ hitRules: ["市场匹配", " "], reason: "其他" }),
    ["市场匹配"]
  );
  assert.deepEqual(
    recommendationReasons({ reason: "市场匹配；科技内容相关\n费用在预算内" }),
    ["市场匹配", "科技内容相关", "费用在预算内"]
  );
  assert.deepEqual(recommendationReasons({}), []);
});
