<script setup lang="ts">
import { useI18n } from "vue-i18n";
import Motion from "./utils/motion";
import { useRouter } from "vue-router";
import { message } from "@/utils/message";
import { loginRules } from "./utils/rule";
import { debounce } from "@pureadmin/utils";
import { useNav } from "@/layout/hooks/useNav";
import { useEventListener } from "@vueuse/core";
import type { FormInstance } from "element-plus";
import { $t, transformI18n } from "@/plugins/i18n";
import { useLayout } from "@/layout/hooks/useLayout";
import { useUserStoreHook } from "@/store/modules/user";
import { initRouter, getTopMenu } from "@/router/utils";
import brandMark from "@/assets/infinix-resource-mark.png";
import { ReImageVerify } from "@/components/ReImageVerify";
import { onMounted, ref, reactive, watch } from "vue";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { useTranslationLang } from "@/layout/hooks/useTranslationLang";
import { useDataThemeChange } from "@/layout/hooks/useDataThemeChange";
import { getAuthConfig } from "@/api/user";

import dayIcon from "@/assets/svg/day.svg?component";
import darkIcon from "@/assets/svg/dark.svg?component";
import globalization from "@/assets/svg/globalization.svg?component";
import Lock from "~icons/ri/lock-fill";
import Check from "~icons/ep/check";
import User from "~icons/ri/user-3-fill";
import Info from "~icons/ri/information-line";
import Keyhole from "~icons/ri/shield-keyhole-line";

defineOptions({
  name: "Login"
});

const imgCode = ref("");
const loginDay = ref(7);
const router = useRouter();
const loading = ref(false);
const checked = ref(false);
const disabled = ref(false);
const ruleFormRef = ref<FormInstance>();
const ssoLoginUrl = ref("/api/auth/sso/login");
const ssoEnabled = ref(false);
const authConfigLoaded = ref(false);

const { t } = useI18n();
const { initStorage } = useLayout();
initStorage();
const { dataTheme, themeMode, dataThemeChange } = useDataThemeChange();
dataThemeChange(themeMode.value);
const { getDropdownItemStyle, getDropdownItemClass } = useNav();
const { locale, translationCh, translationEn } = useTranslationLang();

const ruleForm = reactive({
  username: "",
  password: "",
  verifyCode: ""
});

const onLogin = async (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  await formEl.validate(valid => {
    if (valid) {
      loading.value = true;
      useUserStoreHook()
        .loginByUsername({
          username: ruleForm.username,
          password: ruleForm.password
        })
        .then(async () => {
          // 获取后端路由
          await initRouter();
          const topMenu = getTopMenu(true);
          if (!topMenu?.path) {
            message("当前账号没有可访问菜单，请联系管理员分配权限", {
              type: "warning"
            });
            return;
          }
          disabled.value = true;
          router.push(topMenu.path).then(() => {
            message(t("login.pureLoginSuccess"), { type: "success" });
          });
        })
        .catch(_err => {
          message(t("login.pureLoginFail"), { type: "error" });
        })
        .finally(() => {
          disabled.value = false;
          loading.value = false;
        });
    }
  });
};

function onSSOLogin() {
  if (!ssoEnabled.value) return;
  window.location.assign(ssoLoginUrl.value || "/api/auth/sso/login");
}

onMounted(async () => {
  try {
    const response = await getAuthConfig();
    if (response.code === 0) {
      ssoEnabled.value = Boolean(response.data?.ssoEnabled);
      ssoLoginUrl.value = response.data?.ssoLoginUrl || "/api/auth/sso/login";
    }
  } catch {
    ssoEnabled.value = false;
    ssoLoginUrl.value = "/api/auth/sso/login";
  } finally {
    authConfigLoaded.value = true;
  }
});

const immediateDebounce: any = debounce(
  formRef => onLogin(formRef),
  1000,
  true
);

useEventListener(document, "keydown", ({ code }) => {
  if (
    ["Enter", "NumpadEnter"].includes(code) &&
    !disabled.value &&
    !loading.value
  )
    immediateDebounce(ruleFormRef.value);
});

watch(imgCode, value => {
  useUserStoreHook().SET_VERIFYCODE(value);
});
watch(checked, bool => {
  useUserStoreHook().SET_ISREMEMBERED(bool);
});
watch(loginDay, value => {
  useUserStoreHook().SET_LOGINDAY(value);
});
</script>

<template>
  <div class="login-page select-none">
    <div class="login-tools">
      <!-- 主题 -->
      <el-switch
        v-model="dataTheme"
        inline-prompt
        :active-icon="dayIcon"
        :inactive-icon="darkIcon"
        @change="dataThemeChange"
      />
      <!-- 国际化 -->
      <el-dropdown trigger="click">
        <globalization
          class="hover:text-primary hover:bg-transparent! size-5 ml-1.5 cursor-pointer outline-hidden duration-300"
        />
        <template #dropdown>
          <el-dropdown-menu class="translation">
            <el-dropdown-item
              :style="getDropdownItemStyle(locale, 'zh')"
              :class="['dark:text-white!', getDropdownItemClass(locale, 'zh')]"
              @click="translationCh"
            >
              <IconifyIconOffline
                v-show="locale === 'zh'"
                class="check-zh"
                :icon="Check"
              />
              简体中文
            </el-dropdown-item>
            <el-dropdown-item
              :style="getDropdownItemStyle(locale, 'en')"
              :class="['dark:text-white!', getDropdownItemClass(locale, 'en')]"
              @click="translationEn"
            >
              <span v-show="locale === 'en'" class="check-en">
                <IconifyIconOffline :icon="Check" />
              </span>
              English
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>

    <section class="login-brand" aria-label="产品介绍">
      <div class="brand-lockup">
        <img
          class="brand-logo"
          :src="brandMark"
          alt="Infinix 全球资源运营系统"
        />
        <div>
          <strong>INFINIX</strong>
          <span>GLOBAL RESOURCE OPERATIONS</span>
        </div>
      </div>
      <div class="brand-copy">
        <span class="brand-eyebrow">
          INFINIX RESOURCE NETWORK · ENTERPRISE
        </span>
        <h1>Infinix<br />全球资源运营系统</h1>
        <p>
          在一个工作台里发现创作者、推进合作、追踪内容表现，并让每一次海外投放都有清晰依据。
        </p>
      </div>
      <div class="brand-modules" aria-label="平台能力">
        <article>
          <strong>DISCOVER</strong>
          <span>跨市场资源协同</span>
        </article>
        <article>
          <strong>OPERATE</strong>
          <span>实时内容与项目进度</span>
        </article>
        <article>
          <strong>INSIGHT</strong>
          <span>智能推荐与复盘</span>
        </article>
      </div>
      <p class="brand-note">INFINIX GLOBAL RESOURCES · SECURE WORKSPACE</p>
    </section>

    <main class="login-auth">
      <div class="login-box">
        <div class="login-form">
          <div class="auth-header">
            <span>SECURE SIGN-IN</span>
            <h2>欢迎回来</h2>
            <p>
              登录 <strong>Infinix 全球资源运营系统</strong>
              继续管理全球创作者合作。
            </p>
          </div>
          <Motion>
            <div class="auth-status">
              <i />
              <span>企业级安全认证</span>
              <em>{{ ssoEnabled ? "UAC CONNECTED" : "LOCAL ACCESS" }}</em>
            </div>
          </Motion>

          <Motion :delay="80">
            <div class="sso-login-block">
              <el-button
                class="w-full"
                size="large"
                type="primary"
                :loading="!authConfigLoaded"
                :disabled="authConfigLoaded && !ssoEnabled"
                @click="onSSOLogin"
              >
                <IconifyIconOnline icon="ri:shield-user-line" class="mr-2" />
                企业 SSO 登录
              </el-button>
              <p>
                {{
                  ssoEnabled
                    ? "使用企业统一身份进入 Infinix 全球资源运营系统"
                    : "当前环境暂未连接企业统一身份服务"
                }}
              </p>
              <el-divider>或</el-divider>
              <span class="account-login-label">管理员账号登录</span>
            </div>
          </Motion>

          <el-form
            ref="ruleFormRef"
            :model="ruleForm"
            :rules="loginRules"
            size="large"
          >
            <Motion :delay="100">
              <el-form-item
                :rules="[
                  {
                    required: true,
                    message: transformI18n($t('login.pureUsernameReg')),
                    trigger: 'blur'
                  }
                ]"
                prop="username"
              >
                <el-input
                  v-model="ruleForm.username"
                  clearable
                  :placeholder="t('login.pureUsername')"
                  :prefix-icon="useRenderIcon(User)"
                />
              </el-form-item>
            </Motion>

            <Motion :delay="150">
              <el-form-item prop="password">
                <el-input
                  v-model="ruleForm.password"
                  clearable
                  show-password
                  :placeholder="t('login.purePassword')"
                  :prefix-icon="useRenderIcon(Lock)"
                />
              </el-form-item>
            </Motion>

            <Motion :delay="200">
              <el-form-item prop="verifyCode">
                <el-input
                  v-model="ruleForm.verifyCode"
                  clearable
                  :placeholder="t('login.pureVerifyCode')"
                  :prefix-icon="useRenderIcon(Keyhole)"
                >
                  <template v-slot:append>
                    <ReImageVerify v-model:code="imgCode" />
                  </template>
                </el-input>
              </el-form-item>
            </Motion>

            <Motion :delay="250">
              <el-form-item>
                <div class="w-full h-5 flex-bc">
                  <el-checkbox v-model="checked">
                    <span class="flex">
                      <select
                        v-model="loginDay"
                        :style="{
                          width: loginDay < 10 ? '10px' : '16px',
                          outline: 'none',
                          background: 'none',
                          appearance: 'none',
                          border: 'none'
                        }"
                      >
                        <option value="1">1</option>
                        <option value="7">7</option>
                        <option value="30">30</option>
                      </select>
                      {{ t("login.pureRemember") }}
                      <IconifyIconOffline
                        v-tippy="{
                          content: t('login.pureRememberInfo'),
                          placement: 'top'
                        }"
                        :icon="Info"
                        class="ml-1"
                      />
                    </span>
                  </el-checkbox>
                </div>
                <el-button
                  class="w-full mt-4!"
                  size="default"
                  type="primary"
                  :loading="loading"
                  :disabled="disabled"
                  @click="onLogin(ruleFormRef)"
                >
                  {{ t("login.pureLogin") }}
                </el-button>
              </el-form-item>
            </Motion>
          </el-form>
        </div>
      </div>
      <div class="login-copyright">
        Copyright © 2026&nbsp;Infinix 全球资源运营系统
      </div>
    </main>
  </div>
</template>

<style scoped>
@import url("@/style/login.css");
</style>

<style lang="scss" scoped>
:deep(.el-input-group__append, .el-input-group__prepend) {
  padding: 0;
}

.sso-login-block {
  margin: 18px 0 14px;
  text-align: center;

  p {
    margin: 8px 0 0;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  :deep(.el-divider) {
    margin: 18px 0 10px;
  }

  .account-login-label {
    display: inline-block;
    font-size: 12px;
    font-weight: 700;
    color: #686762;
  }
}

.translation {
  :deep(.el-dropdown-menu__item) {
    padding: 5px 40px;
  }

  .check-zh {
    position: absolute;
    left: 20px;
  }

  .check-en {
    position: absolute;
    left: 20px;
  }
}
</style>
