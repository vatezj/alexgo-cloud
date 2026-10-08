<template>
  <n-space vertical :size="12">
    <n-page-header title="仪表盘" subtitle="Vben Admin (Naive UI) 风格最小实现" />
    <n-card>
      <div>后端健康检查：<a href="/health" target="_blank">/health</a></div>
      <div>Prometheus 指标：<a href="/metrics" target="_blank">/metrics</a></div>
      <div>Swagger：<a href="/swagger/doc.json" target="_blank">/swagger/doc.json</a></div>
    </n-card>
  </n-space>
</template>

<script setup lang="ts"></script>

