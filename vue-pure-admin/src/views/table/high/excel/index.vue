<script setup lang="ts">
import { useColumns } from "./columns";
import { fieldLabel } from "@/utils/fieldI18n";

const { columns, dataList, exportExcel } = useColumns();
</script>

<template>
  <div>
    <el-button type="primary" class="mb-5!" @click="exportExcel">
      {{ fieldLabel("导出") }}
    </el-button>
    <pure-table row-key="id" border :data="dataList" :columns="columns" />
  </div>
</template>
