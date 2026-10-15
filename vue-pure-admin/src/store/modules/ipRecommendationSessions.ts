import { defineStore } from "pinia";
import type { RecommendationSubscription } from "@/api/recommendations";

type SessionNavigation =
  | { action: "select"; id: number }
  | { action: "new" }
  | { action: "reload" }
  | { action: "delete"; id: number };

const SESSION_LIST_TTL_MS = 60_000;
const pendingLists = new WeakMap<
  object,
  { owner: string; promise: Promise<void> }
>();

// Page and original sidebar share one reactive list; the sidebar never fetches
// a second copy of the history or retains business callbacks after unmount.
export const useIPRecommendationSessionsStore = defineStore(
  "ip-recommendation-sessions",
  {
    state: () => ({
      subscriptions: [] as RecommendationSubscription[],
      selectedId: null as number | null,
      busy: false,
      loaded: false,
      loadError: false,
      refreshing: false,
      fetchedAt: 0,
      ownerKey: "",
      navigation: null as SessionNavigation | null
    }),
    actions: {
      clearCache() {
        pendingLists.delete(this);
        this.$reset();
      },
      bindOwner(owner: string) {
        if (this.ownerKey === owner) return;
        this.clearCache();
        this.ownerKey = owner;
      },
      releaseView() {
        this.busy = false;
        this.navigation = null;
      },
      async refresh(
        loader: () => Promise<RecommendationSubscription[]>,
        force = false
      ) {
        if (!this.ownerKey) throw new Error("请先登录后查看项目会话");
        if (
          !force &&
          this.loaded &&
          Date.now() - this.fetchedAt < SESSION_LIST_TTL_MS
        )
          return;
        const pending = pendingLists.get(this);
        if (pending?.owner === this.ownerKey) return pending.promise;
        const request = { owner: this.ownerKey, promise: Promise.resolve() };
        const current = () =>
          pendingLists.get(this) === request && this.ownerKey === request.owner;
        this.refreshing = true;
        this.loadError = false;
        request.promise = Promise.resolve()
          .then(loader)
          .then(items => {
            if (!current()) return;
            this.subscriptions = items;
            this.loaded = true;
            this.fetchedAt = Date.now();
          })
          .catch(error => {
            if (!current()) return;
            this.loaded = true;
            this.loadError = true;
            throw error;
          })
          .finally(() => {
            if (pendingLists.get(this) !== request) return;
            pendingLists.delete(this);
            this.refreshing = false;
          });
        pendingLists.set(this, request);
        return request.promise;
      },
      upsert(item: RecommendationSubscription) {
        pendingLists.delete(this);
        this.refreshing = false;
        const index = this.subscriptions.findIndex(
          value => value.id === item.id
        );
        if (index >= 0) this.subscriptions[index] = item;
        else this.subscriptions.unshift(item);
        this.loaded = true;
        this.loadError = false;
      },
      remove(id: number) {
        pendingLists.delete(this);
        this.refreshing = false;
        this.subscriptions = this.subscriptions.filter(item => item.id !== id);
        if (this.selectedId === id) this.selectedId = null;
      },
      navigate(request: SessionNavigation) {
        if (this.busy) return;
        if (
          (request.action === "select" || request.action === "delete") &&
          !this.subscriptions.some(item => item.id === request.id)
        )
          return;
        this.navigation = { ...request };
      }
    }
  }
);
