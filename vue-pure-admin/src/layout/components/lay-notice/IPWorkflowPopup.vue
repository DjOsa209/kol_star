<script setup lang="ts">
import { h, onMounted, onUnmounted, watch } from "vue";
import { ElButton, ElNotification } from "element-plus";
import { useRouter } from "vue-router";
import { useUserStoreHook } from "@/store/modules/user";
import { useIPWorkflowNotificationsStore } from "@/store/modules/ipWorkflowNotifications";

const router = useRouter();
const user = useUserStoreHook();
const workflow = useIPWorkflowNotificationsStore();
let timer: ReturnType<typeof setInterval>;
let popup: ReturnType<typeof ElNotification> | undefined;
let disposed = false;
watch(
  () => user.username,
  owner => workflow.bindOwner(owner),
  { immediate: true }
);

async function refresh() {
  if (
    document.hidden ||
    !["IPRequests", "IPFeedback", "IPMarketing"].some(name =>
      router.hasRoute(name)
    )
  )
    return;
  try {
    await workflow.refresh();
    if (disposed || popup) return;
    const completed = workflow.items.filter(
      item =>
        item.stage === "marketing_reviewed" &&
        !item.readAt &&
        !workflow.shown.includes(item.id)
    );
    if (!completed.length) return;
    const ids = completed.map(item => item.id);
    workflow.shown.push(...ids);
    const owner = workflow.owner;
    let acknowledged = false;
    const acknowledge = () => {
      if (acknowledged || workflow.owner !== owner) return;
      acknowledged = true;
      void workflow.markRead(ids).catch(() => {
        /* Preserve unread state on a failed acknowledgement. */
      });
    };
    popup = ElNotification({
      title: "IP 需求意见已齐全",
      type: "success",
      duration: 0,
      message: h("div", [
        h(
          "p",
          { style: "margin:0 0 10px;overflow-wrap:anywhere" },
          completed.length === 1
            ? `「${completed[0].projectName}」的 IP 与营销意见已补充完整，请查看。`
            : `${completed.length} 个需求的 IP 与营销意见已补充完整，请到需求管理查看。`
        ),
        h(
          ElButton,
          {
            size: "small",
            type: "primary",
            onClick: () => {
              acknowledge();
              router.push(
                completed.length === 1
                  ? `/business/ip/requests?id=${completed[0].requestId}`
                  : "/business/ip/requests"
              );
              popup?.close();
            }
          },
          () => "查看需求"
        )
      ]),
      onClose: () => {
        if (!disposed) acknowledge();
        popup = undefined;
      }
    });
  } catch {
    // The bell retains previously loaded notices; a later poll retries quietly.
  }
}
onMounted(() => {
  void refresh();
  timer = setInterval(refresh, 60000);
  window.addEventListener("focus", refresh);
  document.addEventListener("visibilitychange", refresh);
  window.addEventListener("ip-workflow-updated", refresh);
});
onUnmounted(() => {
  disposed = true;
  clearInterval(timer);
  window.removeEventListener("focus", refresh);
  document.removeEventListener("visibilitychange", refresh);
  window.removeEventListener("ip-workflow-updated", refresh);
  popup?.close();
  workflow.$reset();
});
</script>
<template><span v-show="false" aria-hidden="true" /></template>
