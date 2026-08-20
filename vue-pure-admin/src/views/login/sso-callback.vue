<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useUserStoreHook } from "@/store/modules/user";
import { getTopMenu, initRouter } from "@/router/utils";
import { removeToken } from "@/utils/auth";

defineOptions({ name: "SSOCallback" });

const router = useRouter();
const errorMessage = ref("");

function parseCallbackParams(raw: string) {
  const params = new URLSearchParams(raw);
  for (const [encodedQuery, value] of Array.from(params.entries())) {
    if (value !== "" || !encodedQuery.includes("=")) continue;
    params.delete(encodedQuery);
    new URLSearchParams(encodedQuery).forEach((nestedValue, key) => {
      if (!params.has(key)) params.set(key, nestedValue);
    });
  }
  return params;
}

onMounted(async () => {
  const params = parseCallbackParams(window.location.search);
  const hash = window.location.hash.replace(/^#/, "");
  const hashQuery = hash.includes("?") ? hash.slice(hash.indexOf("?") + 1) : "";
  parseCallbackParams(hashQuery).forEach((value, key) => {
    if (!params.has(key)) params.set(key, value);
  });
  try {
    await useUserStoreHook().loginBySSO({
      state: params.get("state") || "",
      token: params.get("token") || "",
      rtoken: params.get("rtoken") || "",
      employeeNo: params.get("employeeNo") || "",
      params: Object.fromEntries(params.entries())
    });
    await initRouter();
    const topMenu = getTopMenu(true);
    if (!topMenu?.path) {
      throw new Error("当前账号没有可访问菜单，请联系管理员分配权限");
    }
    window.location.replace(
      `${window.location.origin}${window.location.pathname}#${topMenu.path}`
    );
  } catch (error) {
    removeToken();
    const responseMessage = (error as any)?.response?.data?.message;
    errorMessage.value =
      responseMessage ||
      (typeof error === "string"
        ? error
        : error instanceof Error
          ? error.message
          : "统一身份认证失败，请重新登录");
  }
});
</script>

<template>
  <main class="sso-callback-page">
    <section class="callback-card">
      <IconifyIconOnline
        :icon="errorMessage ? 'ri:error-warning-line' : 'ri:loader-4-line'"
        :class="['callback-icon', { spinning: !errorMessage }]"
      />
      <h1>{{ errorMessage ? "企业身份认证失败" : "正在完成企业身份认证" }}</h1>
      <p>{{ errorMessage || "正在读取企业账号并进入 XMP，请稍候…" }}</p>
      <el-button
        v-if="errorMessage"
        type="primary"
        @click="router.replace('/login')"
      >
        返回登录
      </el-button>
    </section>
  </main>
</template>

<style scoped>
.sso-callback-page {
  display: grid;
  min-height: 100vh;
  padding: 24px;
  place-items: center;
  background: linear-gradient(135deg, #eff6ff, #f8fafc 55%, #ecfdf5);
}

.callback-card {
  width: min(440px, 100%);
  padding: 40px 32px;
  text-align: center;
  background: #fff;
  border: 1px solid rgb(148 163 184 / 22%);
  border-radius: 12px;
  box-shadow: 0 18px 50px rgb(15 23 42 / 10%);
}

.callback-icon {
  width: 34px;
  height: 34px;
  color: #2563eb;
}

.callback-card h1 {
  margin: 18px 0 8px;
  color: #0f172a;
  font-size: 22px;
}

.callback-card p {
  margin: 0 0 22px;
  color: #64748b;
  line-height: 1.6;
}

.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
