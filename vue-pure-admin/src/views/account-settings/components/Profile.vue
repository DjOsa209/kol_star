<script setup lang="ts">
import { formUpload } from "@/api/mock";
import { message } from "@/utils/message";
import { fieldLabel } from "@/utils/fieldI18n";
import { onMounted, reactive, ref } from "vue";
import { type UserInfo, getMine } from "@/api/user";
import type { FormInstance, FormRules } from "element-plus";
import ReCropperPreview from "@/components/ReCropperPreview";
import { createFormData, deviceDetection } from "@pureadmin/utils";
import uploadLine from "~icons/ri/upload-line";

defineOptions({
  name: "Profile"
});

const imgSrc = ref("");
const cropperBlob = ref();
const cropRef = ref();
const uploadRef = ref();
const isShow = ref(false);
const userInfoFormRef = ref<FormInstance>();

const userInfos = reactive({
  avatar: "",
  nickname: "",
  email: "",
  phone: "",
  description: ""
});

const rules = reactive<FormRules<UserInfo>>({
  nickname: [{ required: true, message: "昵称必填", trigger: "blur" }]
});

function queryEmail(queryString, callback) {
  const emailList = [
    { value: "@qq.com" },
    { value: "@126.com" },
    { value: "@163.com" }
  ];
  let results = [];
  let queryList = [];
  emailList.map(item =>
    queryList.push({ value: queryString.split("@")[0] + item.value })
  );
  results = queryString
    ? queryList.filter(
        item =>
          item.value.toLowerCase().indexOf(queryString.toLowerCase()) === 0
      )
    : queryList;
  callback(results);
}

const onChange = uploadFile => {
  const reader = new FileReader();
  reader.onload = e => {
    imgSrc.value = e.target.result as string;
    isShow.value = true;
  };
  reader.readAsDataURL(uploadFile.raw);
};

const handleClose = () => {
  cropRef.value.hidePopover();
  uploadRef.value.clearFiles();
  isShow.value = false;
};

const onCropper = ({ blob }) => (cropperBlob.value = blob);

const getImageFile = (blob: Blob) => {
  const type = blob.type || "image/png";
  const ext =
    type === "image/jpeg"
      ? "jpg"
      : type === "image/webp"
        ? "webp"
        : type === "image/gif"
          ? "gif"
          : "png";
  return new File([blob], `avatar.${ext}`, { type });
};

const handleSubmitImage = () => {
  if (!cropperBlob.value) {
    message(fieldLabel("头像裁剪失败，请重新选择图片"), { type: "error" });
    return;
  }
  const formData = createFormData({
    files: getImageFile(cropperBlob.value)
  });
  formUpload(formData)
    .then(({ code, data }) => {
      if (code === 0) {
        userInfos.avatar = data?.url || userInfos.avatar;
        message(fieldLabel("更新头像成功"), { type: "success" });
        handleClose();
      } else {
        message(fieldLabel("更新头像失败"));
      }
    })
    .catch(error => {
      message(`${fieldLabel("提交异常")} ${error}`, { type: "error" });
    });
};

// 更新信息
const onSubmit = async (formEl: FormInstance) => {
  await formEl.validate((valid, fields) => {
    if (valid) {
      console.log(userInfos);
      message(fieldLabel("更新信息成功"), { type: "success" });
    } else {
      console.log("error submit!", fields);
    }
  });
};

onMounted(async () => {
  const { code, data } = await getMine();
  if (code === 0) {
    Object.assign(userInfos, data);
  }
});
</script>

<template>
  <div :class="['min-w-45', deviceDetection() ? 'max-w-full' : 'max-w-[70%]']">
    <h3 class="my-8!">{{ fieldLabel("个人信息") }}</h3>
    <el-form
      ref="userInfoFormRef"
      label-position="top"
      :rules="rules"
      :model="userInfos"
    >
      <el-form-item :label="fieldLabel('头像')">
        <el-avatar :size="80" :src="userInfos.avatar" />
        <el-upload
          ref="uploadRef"
          accept="image/*"
          action="#"
          :limit="1"
          :auto-upload="false"
          :show-file-list="false"
          :on-change="onChange"
        >
          <el-button plain class="ml-4!">
            <IconifyIconOffline :icon="uploadLine" />
            <span class="ml-2">{{ fieldLabel("更新头像") }}</span>
          </el-button>
        </el-upload>
      </el-form-item>
      <el-form-item :label="fieldLabel('昵称')" prop="nickname">
        <el-input v-model="userInfos.nickname" :placeholder="fieldLabel('请输入昵称')" />
      </el-form-item>
      <el-form-item :label="fieldLabel('邮箱')" prop="email">
        <el-autocomplete
          v-model="userInfos.email"
          :fetch-suggestions="queryEmail"
          :trigger-on-focus="false"
          :placeholder="fieldLabel('请输入邮箱')"
          clearable
          class="w-full"
        />
      </el-form-item>
      <el-form-item :label="fieldLabel('联系电话')">
        <el-input
          v-model="userInfos.phone"
          :placeholder="fieldLabel('请输入联系电话')"
          clearable
        />
      </el-form-item>
      <el-form-item :label="fieldLabel('简介')">
        <el-input
          v-model="userInfos.description"
          :placeholder="fieldLabel('请输入简介')"
          type="textarea"
          :autosize="{ minRows: 6, maxRows: 8 }"
          maxlength="56"
          show-word-limit
        />
      </el-form-item>
      <el-button type="primary" @click="onSubmit(userInfoFormRef)">
        {{ fieldLabel("更新信息") }}
      </el-button>
    </el-form>
    <el-dialog
      v-model="isShow"
      width="40%"
      :title="fieldLabel('编辑头像')"
      destroy-on-close
      :closeOnClickModal="false"
      :before-close="handleClose"
      :fullscreen="deviceDetection()"
    >
      <ReCropperPreview ref="cropRef" :imgSrc="imgSrc" @cropper="onCropper" />
      <template #footer>
        <div class="dialog-footer">
          <el-button bg text @click="handleClose">{{ fieldLabel("取消") }}</el-button>
          <el-button bg text type="primary" @click="handleSubmitImage">
            {{ fieldLabel("确定") }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
