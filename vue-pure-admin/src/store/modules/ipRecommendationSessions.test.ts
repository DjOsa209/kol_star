import assert from "node:assert/strict";
import test from "node:test";
import { createPinia, setActivePinia } from "pinia";
// @ts-expect-error -- Native Node tests resolve the TypeScript source directly.
import { useIPRecommendationSessionsStore } from "./ipRecommendationSessions.ts";

const item = (id: number, name = `Project ${id}`) => ({
  id,
  name,
  domain: "ip" as const,
  query: "public brief",
  markets: [],
  active: false
});
function setup() {
  setActivePinia(createPinia());
  const store = useIPRecommendationSessionsStore();
  store.bindOwner("user-a:session-a");
  return store;
}

test("page exit and session switching reuse the warm list without another request", async () => {
  const store = setup();
  let requests = 0;
  const loader = async () => {
    requests++;
    return [item(1), item(2)];
  };
  await store.refresh(loader);
  store.selectedId = 1;
  store.navigate({ action: "select", id: 2 });
  assert.equal(requests, 1);
  assert.equal(store.subscriptions.length, 2);
  store.releaseView();
  assert.equal(store.navigation, null);
  assert.equal(store.loaded, true);
  await store.refresh(loader);
  assert.equal(requests, 1);
});

test("overlapping requests share a loader and stale refresh keeps the existing menu visible", async () => {
  const store = setup();
  await store.refresh(async () => [item(1)]);
  store.fetchedAt = Date.now() - 61_000;
  let finish!: (items: ReturnType<typeof item>[]) => void;
  let requests = 0;
  const loader = () => {
    requests++;
    return new Promise<ReturnType<typeof item>[]>(resolve => {
      finish = resolve;
    });
  };
  const a = store.refresh(loader);
  const b = store.refresh(loader);
  await Promise.resolve();
  assert.equal(requests, 1);
  assert.equal(store.loaded, true);
  assert.equal(store.subscriptions[0].id, 1);
  finish([item(2)]);
  await Promise.all([a, b]);
  assert.equal(store.subscriptions[0].id, 2);
});

test("switching login owners discards cached data and ignores the old response", async () => {
  const store = setup();
  let finish!: (items: ReturnType<typeof item>[]) => void;
  const pending = store.refresh(
    () =>
      new Promise<ReturnType<typeof item>[]>(resolve => {
        finish = resolve;
      })
  );
  await Promise.resolve();
  store.bindOwner("user-b:session-b");
  await store.refresh(async () => [item(20)]);
  finish([item(10)]);
  await pending;
  assert.deepEqual(
    store.subscriptions.map(value => value.id),
    [20]
  );
  store.clearCache();
  assert.equal(store.loaded, false);
  assert.equal(store.ownerKey, "");
  assert.equal(store.subscriptions.length, 0);
});

test("local conversation updates and deletion cannot be overwritten by an old list request", async () => {
  const store = setup();
  await store.refresh(async () => [item(1), item(2)]);
  let finish!: (items: ReturnType<typeof item>[]) => void;
  const pending = store.refresh(
    () =>
      new Promise<ReturnType<typeof item>[]>(resolve => {
        finish = resolve;
      }),
    true
  );
  await Promise.resolve();
  store.upsert(item(1, "Updated conversation"));
  store.remove(2);
  finish([item(1, "Old conversation"), item(2)]);
  await pending;
  assert.equal(store.subscriptions[0].name, "Updated conversation");
  assert.equal(store.subscriptions.length, 1);
});

test("a failed refresh preserves the warm list and a forced retry can recover", async () => {
  const store = setup();
  await store.refresh(async () => [item(1)]);
  await assert.rejects(
    store.refresh(async () => {
      throw new Error("offline");
    }, true),
    /offline/
  );
  assert.equal(store.loaded, true);
  assert.equal(store.loadError, true);
  assert.equal(store.subscriptions.length, 1);
  await store.refresh(async () => [item(2)], true);
  assert.equal(store.loadError, false);
  assert.equal(store.subscriptions[0].id, 2);
});

test("deletion navigation respects the live list and busy guard", () => {
  const store = setup();
  store.upsert(item(1));
  store.navigate({ action: "delete", id: 99 });
  assert.equal(store.navigation, null);
  store.busy = true;
  store.navigate({ action: "delete", id: 1 });
  assert.equal(store.navigation, null);
  store.busy = false;
  store.navigate({ action: "delete", id: 1 });
  assert.deepEqual(store.navigation, { action: "delete", id: 1 });
  store.selectedId = 1;
  store.remove(1);
  assert.equal(store.selectedId, null);
});
