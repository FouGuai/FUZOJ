<template>
  <div class="page page--wide">
    <section class="hero-panel">
      <div>
        <p class="hero-panel__eyebrow">Problemset</p>
        <h1>挑题、写代码、看结果，一页完成。</h1>
        <p class="hero-panel__text">
          公开题目可直接浏览；登录后可以保存状态、提交判题并进入出题台。
        </p>
      </div>
      <div class="hero-panel__metrics">
        <div class="metric-card">
          <span>Loaded</span>
          <strong>{{ problems.length }}</strong>
        </div>
        <div class="metric-card">
          <span>Has More</span>
          <strong>{{ hasMore ? "Yes" : "No" }}</strong>
        </div>
      </div>
    </section>

    <section v-if="problems.length" class="problem-grid">
      <RouterLink
        v-for="item in problems"
        :key="item.problem_id"
        :to="`/problems/${item.problem_id}`"
        class="problem-card"
      >
        <div class="problem-card__header">
          <span class="problem-card__id">#{{ item.problem_id }}</span>
          <span class="problem-card__version">v{{ item.version }}</span>
        </div>
        <h3>{{ item.title }}</h3>
        <p>最近更新：{{ formatDate(item.updated_at) }}</p>
      </RouterLink>
    </section>
    <section v-else class="form-card">
      <el-empty description="当前题库没有可见题目。只有已发布题目会出现在列表中。" />
      <div class="page__footer-actions">
        <el-input v-model="quickProblemId" placeholder="输入题号，例如 1001" style="max-width: 280px" />
        <el-button type="primary" @click="openByProblemId">按题号打开</el-button>
      </div>
    </section>

    <div class="page__footer-actions">
      <el-button :loading="loading" @click="loadProblems()">
        {{ problems.length ? "刷新" : "加载题库" }}
      </el-button>
      <el-button v-if="hasMore" type="primary" :loading="loadingMore" @click="loadMore">
        加载更多
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { RouterLink, useRouter } from "vue-router";
import { listProblems } from "@/api/problems";
import type { ProblemListItem } from "@/api/types";
import { formatDate } from "@/utils/format";

const problems = ref<ProblemListItem[]>([]);
const hasMore = ref(false);
const nextCursor = ref("");
const loading = ref(false);
const loadingMore = ref(false);
const quickProblemId = ref("");
const router = useRouter();

async function loadProblems() {
  loading.value = true;
  try {
    const data = await listProblems({ limit: 12 });
    problems.value = data.items;
    hasMore.value = data.has_more;
    nextCursor.value = data.next_cursor || "";
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Load problems failed");
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  if (!nextCursor.value) {
    return;
  }

  loadingMore.value = true;
  try {
    const data = await listProblems({ limit: 12, cursor: nextCursor.value });
    problems.value = problems.value.concat(data.items);
    hasMore.value = data.has_more;
    nextCursor.value = data.next_cursor || "";
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Load more failed");
  } finally {
    loadingMore.value = false;
  }
}

async function openByProblemId() {
  const id = Number(quickProblemId.value.trim());
  if (!id || Number.isNaN(id) || id <= 0) {
    ElMessage.warning("请输入有效题号");
    return;
  }
  await router.push(`/problems/${id}`);
}

onMounted(() => {
  void loadProblems();
});
</script>
