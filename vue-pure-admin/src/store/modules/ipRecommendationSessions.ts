import { defineStore } from "pinia";
import type { RecommendationSubscription } from "@/api/recommendations";

type SessionNavigation =
  | { action: "select"; id: number }
  | { action: "new" }
  | { action: "reload" };

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
      navigation: null as SessionNavigation | null
    }),
    actions: {
      navigate(request: SessionNavigation) {
        if (this.busy) return;
        if (
          request.action === "select" &&
          !this.subscriptions.some(item => item.id === request.id)
        )
          return;
        this.navigation = { ...request };
      }
    }
  }
);
