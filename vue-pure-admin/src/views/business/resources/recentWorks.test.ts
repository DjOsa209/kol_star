import assert from "node:assert/strict";
import test from "node:test";
// Native Node test execution resolves the TypeScript source directly.
// @ts-expect-error -- the app build uses Vite's extensionless resolver.
import { buildRecentCooperationWorks } from "./recentWorks.ts";

test("uses a cooperation cover when the platform post is unavailable", () => {
  const works = buildRecentCooperationWorks(
    [
      {
        id: 344,
        finalLink: "https://www.stuff.tv/hot-stuff/example/",
        contentCoverUrl:
          "/api/uploads/resource-images/1246/project-content/344/website.png",
        releaseDate: "2026-02-18",
        views: 152000,
        engagementCount: 9600
      }
    ],
    []
  );

  assert.equal(works.length, 1);
  assert.equal(
    works[0].coverUrl,
    "/api/uploads/resource-images/1246/project-content/344/website.png"
  );
  assert.equal(works[0].viewCount, 152000);
  assert.equal(works[0].interactionCount, 9600);
});

test("enriches cooperation data with a matching platform post", () => {
  const works = buildRecentCooperationWorks(
    [
      {
        id: 340,
        finalLink: "https://www.youtube.com/shorts/YyLyg4COWp0",
        contentCoverUrl: "/stored-cover.jpg",
        views: 100
      }
    ],
    [
      {
        id: 99,
        postUrl: "https://www.youtube.com/shorts/YyLyg4COWp0/",
        coverUrl: "/post-cover.jpg",
        durationSeconds: 48,
        viewCount: 200
      }
    ]
  );

  assert.equal(works.length, 1);
  assert.equal(works[0].coverUrl, "/post-cover.jpg");
  assert.equal(works[0].durationSeconds, 48);
  assert.equal(works[0].viewCount, 200);
});
