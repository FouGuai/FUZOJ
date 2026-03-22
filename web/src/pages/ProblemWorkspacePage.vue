<template>
  <div class="workspace-page">
    <section class="workspace-header">
      <div>
        <p class="workspace-header__eyebrow">Problem Workspace</p>
        <h1>{{ titleText }}</h1>
        <p class="workspace-header__meta">
          Problem #{{ problemId }} · Version {{ statement?.version || "-" }} · Updated {{ formatDate(statement?.updated_at) }}
        </p>
      </div>

      <div class="workspace-header__actions">
        <el-select v-model="selectedLanguageId" style="width: 140px">
          <el-option
            v-for="language in editorLanguages"
            :key="language.id"
            :label="language.label"
            :value="language.id"
          />
        </el-select>
        <el-button @click="toggleResultPanel">
          {{ resultPanelCollapsed ? "展开结果" : "收起结果" }}
        </el-button>
        <el-button @click="resetTemplate">重置模板</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCode">提交</el-button>
      </div>
    </section>

    <SplitPane :stacked="isMobile">
      <template #primary>
        <section class="workspace-panel workspace-panel--statement">
          <div class="workspace-panel__title">题面</div>
          <div v-if="statement" class="statement-body" v-html="statementHtml"></div>
          <el-empty v-else description="题面加载中" />
        </section>
      </template>

      <template #secondary>
        <div class="workspace-right" :class="{ 'workspace-right--result-collapsed': resultPanelCollapsed }">
          <section class="workspace-panel workspace-panel--editor">
            <div class="workspace-panel__title">Code</div>
            <CodeEditor v-model="code" :language="currentLanguage.monaco" />
          </section>
          <section class="workspace-panel workspace-panel--result">
            <div class="workspace-panel__titlebar">
              <div class="workspace-panel__title">Result</div>
              <el-button text @click="toggleResultPanel">
                {{ resultPanelCollapsed ? "展开" : "收起" }}
              </el-button>
            </div>
            <div v-if="resultPanelCollapsed" class="workspace-result-collapsed-tip">
              结果面板已收起
            </div>
            <div v-else class="workspace-result-body">
              <StatusPanel :status="submissionStatus" />
            </div>
          </section>
        </div>
      </template>
    </SplitPane>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useRoute, useRouter } from "vue-router";
import { createSubmission } from "@/api/submissions";
import { getLatest, getStatement } from "@/api/problems";
import CodeEditor from "@/components/CodeEditor.vue";
import SplitPane from "@/components/SplitPane.vue";
import StatusPanel from "@/components/StatusPanel.vue";
import { editorLanguages } from "@/constants/languages";
import { useSubmissionStream } from "@/composables/useSubmissionStream";
import { useAuthStore } from "@/stores/auth";
import { useProblemDraftStore } from "@/stores/problemDrafts";
import type { LatestMetaPayload, StatementPayload } from "@/api/types";
import { formatDate, toStatementHtml } from "@/utils/format";
import { makeIdempotencyKey } from "@/utils/upload";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const draftStore = useProblemDraftStore();
const stream = useSubmissionStream();
const submissionStatus = stream.status;

const statement = ref<StatementPayload | null>(null);
const latestMeta = ref<LatestMetaPayload | null>(null);
const titleText = ref("Loading...");
const submitting = ref(false);
const selectedLanguageId = ref(editorLanguages[0].id);
const code = ref(editorLanguages[0].template);
const isMobile = ref(window.innerWidth < 820);
const resultPanelCollapsed = ref(false);

const problemId = computed(() => Number(route.params.id));
const currentLanguage = computed(
  () => editorLanguages.find((item: { id: string }) => item.id === selectedLanguageId.value) || editorLanguages[0],
);
const statementHtml = computed(() => {
  if (!statement.value) {
    return "";
  }
  return toStatementHtml(statement.value.statement_md);
});

const resizeHandler = () => {
  isMobile.value = window.innerWidth < 820;
};

async function loadProblem() {
  const id = problemId.value;
  if (!id || Number.isNaN(id) || id <= 0) {
    statement.value = null;
    latestMeta.value = null;
    titleText.value = "Invalid Problem ID";
    ElMessage.warning("Invalid problem id");
    return;
  }

  try {
    const [statementData, latestData] = await Promise.all([getStatement(id), getLatest(id)]);
    statement.value = statementData;
    latestMeta.value = latestData;
    const firstHeading = statementData.statement_md
      .split("\n")
      .map((line: string) => line.trim())
      .find((line: string) => line.startsWith("#"));
    titleText.value = firstHeading ? firstHeading.replace(/^#+\s*/, "") : `Problem #${id}`;

    const cached = draftStore.getDraft(id, selectedLanguageId.value);
    code.value = cached || currentLanguage.value.template;
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Load problem failed");
  }
}

function resetTemplate() {
  code.value = currentLanguage.value.template;
}

function toggleResultPanel() {
  resultPanelCollapsed.value = !resultPanelCollapsed.value;
  void nextTick(() => {
    window.dispatchEvent(new Event("resize"));
  });
}

async function submitCode() {
  authStore.initialize();
  const userId = Number(authStore.user?.id || 0);
  if (!authStore.isAuthenticated || !authStore.user || !userId || Number.isNaN(userId) || userId <= 0) {
    await router.push({
      name: "login",
      query: {
        redirect: route.fullPath,
      },
    });
    return;
  }
  if (!problemId.value || Number.isNaN(problemId.value) || problemId.value <= 0) {
    ElMessage.warning("Invalid problem id");
    return;
  }
  if (!selectedLanguageId.value.trim()) {
    ElMessage.warning("Language is required");
    return;
  }
  if (!code.value.trim()) {
    ElMessage.warning("Source code is required");
    return;
  }

  submitting.value = true;
  try {
    resultPanelCollapsed.value = false;
    void nextTick(() => {
      window.dispatchEvent(new Event("resize"));
    });
    const submission = await createSubmission({
      problem_id: problemId.value,
      user_id: userId,
      language_id: selectedLanguageId.value,
      source_code: code.value,
      contest_id: "",
      scene: "practice",
      extra_compile_flags: [],
    }, makeIdempotencyKey());

    stream.connect(submission.submission_id);
    ElMessage.success(`Submitted: ${submission.submission_id}`);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Submit failed");
  } finally {
    submitting.value = false;
  }
}

watch(
  () => selectedLanguageId.value,
  (languageId) => {
    const cached = draftStore.getDraft(problemId.value, languageId);
    code.value = cached || currentLanguage.value.template;
  },
);

watch(
  () => problemId.value,
  async () => {
    stream.reset();
    statement.value = null;
    latestMeta.value = null;
    titleText.value = "Loading...";
    await loadProblem();
  },
  { immediate: true },
);

watch(
  () => code.value,
  (value) => {
    draftStore.setDraft(problemId.value, selectedLanguageId.value, value);
  },
);

onMounted(() => {
  window.addEventListener("resize", resizeHandler);
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", resizeHandler);
});
</script>
