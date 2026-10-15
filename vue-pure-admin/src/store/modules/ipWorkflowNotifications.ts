import { defineStore } from "pinia";
import {
  listIPWorkflowNotifications,
  markIPWorkflowNotificationsRead,
  type IPWorkflowNotification
} from "@/api/ip";

export const useIPWorkflowNotificationsStore = defineStore(
  "ip-workflow-notifications",
  {
    state: () => ({
      owner: "",
      items: [] as IPWorkflowNotification[],
      unread: 0,
      loading: false,
      shown: [] as number[]
    }),
    actions: {
      bindOwner(owner: string) {
        if (this.owner === owner) return;
        this.$reset();
        this.owner = owner;
      },
      async refresh() {
        if (!this.owner || this.loading) return;
        const owner = this.owner;
        this.loading = true;
        try {
          const { data } = await listIPWorkflowNotifications();
          if (this.owner !== owner) return;
          this.items = data.list || [];
          this.unread = Number(data.unread) || 0;
        } finally {
          if (this.owner === owner) this.loading = false;
        }
      },
      async markRead(ids: number[]) {
        if (!ids.length) return;
        const owner = this.owner;
        await markIPWorkflowNotificationsRead(ids);
        if (this.owner !== owner) return;
        const selected = new Set(ids);
        const time = new Date().toISOString();
        for (const item of this.items) {
          if (selected.has(item.id) && !item.readAt) {
            item.readAt = time;
            this.unread = Math.max(0, this.unread - 1);
          }
        }
      }
    }
  }
);
