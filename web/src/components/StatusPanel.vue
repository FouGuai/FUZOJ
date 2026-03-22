<template>
  <section class="status-panel">
    <header class="status-panel__header">
      <div>
        <p class="status-panel__eyebrow">Judge Result</p>
        <h3>{{ status?.verdict || "Waiting" }}</h3>
      </div>
      <el-tag :type="statusTagType">{{ status?.status || "Idle" }}</el-tag>
    </header>

    <template v-if="status">
      <div class="status-panel__stats">
        <div class="metric-card">
          <span>Score</span>
          <strong>{{ status.score }}</strong>
        </div>
        <div class="metric-card">
          <span>Time</span>
          <strong>{{ displayTotalTimeMs }} ms</strong>
        </div>
        <div class="metric-card">
          <span>Memory</span>
          <strong>{{ formatBytesFromKB(displayMaxMemoryKB) }}</strong>
        </div>
      </div>

      <el-progress
        :percentage="progressPercentage"
        :stroke-width="10"
        :show-text="false"
        class="status-panel__progress"
      />

      <p class="status-panel__progress-text">
        {{ status.progress.done_tests }} / {{ status.progress.total_tests }} tests
      </p>

      <div v-if="status.compile" class="status-panel__block">
        <div class="status-panel__block-title">Compile</div>
        <pre>{{ status.compile.Log || status.compile.Error || "No compile log." }}</pre>
      </div>

      <div v-if="status.tests?.length" class="status-panel__block">
        <div class="status-panel__block-title">Testcases</div>
        <div class="testcase-table">
          <div class="testcase-table__row testcase-table__row--head">
            <span>ID</span>
            <span>Verdict</span>
            <span>Time</span>
            <span>Memory</span>
            <span>Score</span>
          </div>
          <div v-for="item in status.tests" :key="item.TestID" class="testcase-table__row">
            <span>{{ item.TestID }}</span>
            <span>{{ item.Verdict }}</span>
            <span>{{ item.TimeMs }} ms</span>
            <span>{{ formatBytesFromKB(item.MemoryKB) }}</span>
            <span>{{ item.Score }}</span>
          </div>
        </div>
      </div>

      <div v-if="status.error_message" class="status-panel__block">
        <div class="status-panel__block-title">Error</div>
        <pre>{{ status.error_message }}</pre>
      </div>
    </template>

    <div v-else class="status-panel__empty">
      提交后会在这里实时显示编译和测试点结果。
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { JudgeStatusData } from "@/api/types";
import { formatBytesFromKB } from "@/utils/format";

const props = defineProps<{
  status: JudgeStatusData | null;
}>();

const progressPercentage = computed(() => {
  if (!props.status || !props.status.progress.total_tests) {
    return 0;
  }
  return Math.round((props.status.progress.done_tests / props.status.progress.total_tests) * 100);
});

const displayTotalTimeMs = computed(() => {
  const status = props.status;
  if (!status) {
    return 0;
  }
  if (status.summary?.TotalTimeMs && status.summary.TotalTimeMs > 0) {
    return status.summary.TotalTimeMs;
  }
  if (status.tests?.length) {
    return status.tests.reduce((sum, item) => sum + (item.TimeMs || 0), 0);
  }
  return status.compile?.TimeMs || 0;
});

const displayMaxMemoryKB = computed(() => {
  const status = props.status;
  if (!status) {
    return 0;
  }
  if (status.summary?.MaxMemoryKB && status.summary.MaxMemoryKB > 0) {
    return status.summary.MaxMemoryKB;
  }
  if (status.tests?.length) {
    return status.tests.reduce((max, item) => Math.max(max, item.MemoryKB || 0), 0);
  }
  return status.compile?.MemoryKB || 0;
});

const statusTagType = computed(() => {
  const verdict = props.status?.verdict?.toLowerCase() || "";
  if (verdict.includes("accept")) {
    return "success";
  }
  if (verdict.includes("wrong") || verdict.includes("fail")) {
    return "danger";
  }
  return "warning";
});
</script>
