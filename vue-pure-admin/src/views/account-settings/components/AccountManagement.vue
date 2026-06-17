<script setup lang="ts">
import { reactive, ref } from "vue";
import { updateMinePassword } from "@/api/user";
import { message } from "@/utils/message";
import { deviceDetection } from "@pureadmin/utils";
import type { FormInstance, FormRules } from "element-plus";

defineOptions({
  name: "AccountManagement"
});

const list = ref([
  {
    key: "password",
    title: "账户密码",
    illustrate: "当前密码强度：强",
    button: "修改"
  },
  {
    key: "securityQuestion",
    title: "密保问题",
    illustrate: "未设置密保问题，密保问题可有效保护账户安全",
    button: "修改"
  }
]);

const passwordDialogVisible = ref(false);
const passwordLoading = ref(false);
const passwordFormRef = ref<FormInstance>();
const passwordForm = reactive({
  oldPassword: "",
  newPassword: "",
  confirmPassword: ""
});
const REGEXP_PWD =
  /^(?![0-9]+$)(?![a-z]+$)(?![A-Z]+$)(?!([^(0-9a-zA-Z)]|[()])+$)(?!^.*[\u4E00-\u9FA5].*$)([^(0-9a-zA-Z)]|[()]|[a-z]|[A-Z]|[0-9]){8,18}$/;
const passwordRules: FormRules = {
  oldPassword: [{ required: true, message: "请输入当前密码", trigger: "blur" }],
  newPassword: [
    { required: true, message: "请输入新密码", trigger: "blur" },
    {
      validator: (_rule, value, callback) => {
        if (!REGEXP_PWD.test(value)) {
          callback(
            new Error("密码格式应为8-18位数字、字母、符号的任意两种组合")
          );
          return;
        }
        if (value === passwordForm.oldPassword) {
          callback(new Error("新密码不能与当前密码相同"));
          return;
        }
        callback();
      },
      trigger: "blur"
    }
  ],
  confirmPassword: [
    { required: true, message: "请再次输入新密码", trigger: "blur" },
    {
      validator: (_rule, value, callback) => {
        if (value !== passwordForm.newPassword) {
          callback(new Error("两次输入的新密码不一致"));
          return;
        }
        callback();
      },
      trigger: "blur"
    }
  ]
};

function onClick(item) {
  if (item.key === "password") {
    passwordDialogVisible.value = true;
    return;
  }
  message("请根据具体业务自行实现", { type: "success" });
}

function resetPasswordForm() {
  passwordForm.oldPassword = "";
  passwordForm.newPassword = "";
  passwordForm.confirmPassword = "";
  passwordFormRef.value?.clearValidate();
}

async function submitPassword() {
  if (!passwordFormRef.value) return;
  await passwordFormRef.value.validate(async valid => {
    if (!valid) return;
    passwordLoading.value = true;
    try {
      const { code, message: resultMessage } =
        await updateMinePassword(passwordForm);
      if (code !== 0) {
        message(resultMessage || "修改密码失败", { type: "error" });
        return;
      }
      message("密码修改成功", { type: "success" });
      passwordDialogVisible.value = false;
      resetPasswordForm();
    } finally {
      passwordLoading.value = false;
    }
  });
}
</script>

<template>
  <div :class="['min-w-45', deviceDetection() ? 'max-w-full' : 'max-w-[70%]']">
    <h3 class="my-8!">账户管理</h3>
    <div v-for="(item, index) in list" :key="index">
      <div class="flex items-center">
        <div class="flex-1">
          <p>{{ item.title }}</p>
          <el-text class="mx-1" type="info">{{ item.illustrate }}</el-text>
        </div>
        <el-button type="primary" text @click="onClick(item)">
          {{ item.button }}
        </el-button>
      </div>
      <el-divider />
    </div>
    <el-dialog
      v-model="passwordDialogVisible"
      title="修改账户密码"
      width="420px"
      :close-on-click-modal="false"
      @closed="resetPasswordForm"
    >
      <el-form
        ref="passwordFormRef"
        :model="passwordForm"
        :rules="passwordRules"
        label-width="88px"
      >
        <el-form-item label="当前密码" prop="oldPassword">
          <el-input
            v-model="passwordForm.oldPassword"
            clearable
            show-password
            type="password"
            autocomplete="current-password"
            placeholder="请输入当前密码"
          />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input
            v-model="passwordForm.newPassword"
            clearable
            show-password
            type="password"
            autocomplete="new-password"
            placeholder="请输入新密码"
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="passwordForm.confirmPassword"
            clearable
            show-password
            type="password"
            autocomplete="new-password"
            placeholder="请再次输入新密码"
          />
        </el-form-item>
        <el-text type="info" size="small">
          密码需为 8-18 位，并包含数字、字母、符号中的任意两类。
        </el-text>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="passwordLoading"
          @click="submitPassword"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss" scoped>
.el-divider--horizontal {
  border-top: 0.1px var(--el-border-color) var(--el-border-style);
}
</style>
