<template>
  <div class="page page--narrow create-page">
    <section class="hero-panel hero-panel--compact">
      <div>
        <p class="hero-panel__eyebrow">Problem Management</p>
        <h1>创建题目并发布新版本</h1>
        <p class="hero-panel__text">填写基础信息，上传数据包，编辑题面，然后发布版本。</p>
      </div>
    </section>

    <el-steps :active="activeStep" finish-status="success" align-center class="create-steps create-page__steps">
      <el-step title="Base" />
      <el-step title="Upload" />
      <el-step title="Statement" />
      <el-step title="Publish" />
    </el-steps>

    <section class="form-card">
      <div class="form-card__section">
        <h2>1. 基础信息</h2>
        <el-form label-position="top" class="create-form">
          <el-form-item label="题目标题">
            <el-input v-model="form.title" placeholder="Two Sum" />
          </el-form-item>
          <p class="inline-hint">先填写标题，再进行数据包上传和题面编辑。</p>
          <p v-if="problemId" class="inline-hint">Problem ID: {{ problemId }}</p>
        </el-form>
      </div>

      <div class="form-card__section">
        <h2>2. 数据包与判题配置</h2>
        <el-upload :auto-upload="false" :show-file-list="true" :limit="1" :on-change="handleFileChange">
          <el-button>选择数据包</el-button>
        </el-upload>
        <p v-if="selectedFile" class="inline-hint">已选择文件：{{ selectedFile.name }}（{{ selectedFile.size }} bytes）</p>

        <div class="builder-grid">
          <section class="builder-card">
            <div class="builder-card__header">
              <div>
                <h3>判题配置</h3>
                <p>设置时间、内存和判题模式。</p>
              </div>
            </div>
            <el-form label-position="top" class="builder-form">
              <div class="builder-card__grid">
                <el-form-item label="Time Limit (ms)">
                  <el-input-number
                    v-model="judgeConfig.timeLimitMs"
                    class="control-full"
                    :min="1"
                    :step="100"
                    :controls-position="'right'"
                  />
                </el-form-item>
                <el-form-item label="Memory Limit (KB)">
                  <el-input-number
                    v-model="judgeConfig.memoryLimitKb"
                    class="control-full"
                    :min="1024"
                    :step="1024"
                    :controls-position="'right'"
                  />
                </el-form-item>
                <el-form-item label="Checker Type">
                  <el-select v-model="judgeConfig.checkerType" class="control-full">
                    <el-option label="Default" value="default" />
                    <el-option label="Special Judge" value="spj" />
                  </el-select>
                </el-form-item>
                <el-form-item label="Output Compare">
                  <el-select v-model="judgeConfig.compareMode" class="control-full">
                    <el-option label="Exact Match" value="exact" />
                    <el-option label="Ignore Whitespace" value="trim" />
                  </el-select>
                </el-form-item>
              </div>
            </el-form>
          </section>

          <section class="builder-card">
            <div class="builder-card__header">
              <div>
                <h3>测试点</h3>
                <p>用表单维护测试点，不需要手写 JSON。</p>
              </div>
              <el-button type="primary" plain @click="addTestCase">新增测试点</el-button>
            </div>

            <div class="testcase-stack">
              <article v-for="(item, index) in testCases" :key="item.key" class="testcase-editor">
                <div class="testcase-editor__header">
                  <strong>测试点 {{ index + 1 }}</strong>
                  <el-button text @click="removeTestCase(index)">删除</el-button>
                </div>
                <el-form label-position="top" class="builder-form">
                  <div class="builder-card__grid">
                    <el-form-item label="Test ID">
                      <el-input v-model="item.id" placeholder="sample-1" />
                    </el-form-item>
                    <el-form-item label="Score">
                      <el-input-number v-model="item.score" class="control-full" :min="0" :step="10" :controls-position="'right'" />
                    </el-form-item>
                    <el-form-item label="Subtask">
                      <el-input v-model="item.subtaskId" placeholder="optional" />
                    </el-form-item>
                    <el-form-item label="Input File">
                      <el-input v-model="item.input" placeholder="tests/sample1.in" />
                    </el-form-item>
                    <el-form-item label="Answer File">
                      <el-input v-model="item.output" placeholder="tests/sample1.out" />
                    </el-form-item>
                  </div>
                </el-form>
              </article>
            </div>
          </section>
        </div>

        <div class="builder-preview">
          <div class="builder-preview__item">
            <span>Manifest</span>
            <strong>{{ normalizedTestCases.length }} tests</strong>
          </div>
          <div class="builder-preview__item">
            <span>Limits</span>
            <strong>{{ judgeConfig.timeLimitMs }} ms / {{ judgeConfig.memoryLimitKb }} KB</strong>
          </div>
        </div>
        <div class="page__footer-actions">
          <el-button :disabled="!selectedFile || !form.title.trim()" :loading="uploading" @click="handleUpload">
            上传并完成
          </el-button>
          <el-button
            v-if="problemId && uploadState.uploadId"
            :disabled="uploading"
            @click="handleAbortUpload"
          >
            中止上传
          </el-button>
        </div>
        <p v-if="uploadState.version" class="inline-hint">
          Upload ready. Version {{ uploadState.version }}
        </p>
      </div>

      <div class="form-card__section">
        <h2>3. 题面内容</h2>
        <el-input
          v-model="form.statementMd"
          type="textarea"
          :rows="16"
          placeholder="# Title&#10;&#10;Describe the problem here."
        />
        <div class="page__footer-actions">
          <el-button
            :disabled="!form.title.trim() || !uploadState.version"
            :loading="savingStatement"
            @click="handleSaveStatement"
          >
            保存题面
          </el-button>
        </div>
      </div>

      <div class="form-card__section">
        <h2>4. 发布版本</h2>
        <el-button
          type="primary"
          :disabled="!form.title.trim() || !uploadState.version"
          :loading="publishing"
          @click="handlePublish"
        >
          发布版本
        </el-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import type { UploadFile } from "element-plus";
import { computed, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useRouter } from "vue-router";
import {
  abortUpload,
  completeUpload,
  createProblem,
  prepareUpload,
  publishVersion,
  signParts,
  updateStatement,
} from "@/api/problems";
import { useAuthStore } from "@/stores/auth";
import { fileSha256Hex, makeClientId, makeIdempotencyKey, sha256Hex, splitFile, uploadChunk } from "@/utils/upload";

const router = useRouter();
const authStore = useAuthStore();

const activeStep = ref(0);
const creating = ref(false);
const uploading = ref(false);
const savingStatement = ref(false);
const publishing = ref(false);
const problemId = ref<number | null>(null);
const selectedFile = ref<File | null>(null);

const form = reactive({
  title: "",
  statementMd: "# Problem Statement\n\nDescribe the task.",
});

const uploadState = reactive({
  uploadId: 0,
  version: 0,
});

const judgeConfig = reactive({
  timeLimitMs: 1000,
  memoryLimitKb: 262144,
  checkerType: "default",
  compareMode: "exact",
});

type EditableTestCase = {
  key: string;
  id: string;
  score: number;
  subtaskId: string;
  input: string;
  output: string;
};

const testCases = ref<EditableTestCase[]>([
  {
    key: makeClientId(),
    id: "sample-1",
    score: 100,
    subtaskId: "",
    input: "tests/sample1.in",
    output: "tests/sample1.out",
  },
]);

const normalizedTestCases = computed(() =>
  testCases.value
    .map((item) => ({
      id: item.id.trim(),
      score: item.score,
      subtask_id: item.subtaskId.trim(),
      input: item.input.trim(),
      output: item.output.trim(),
    }))
    .filter((item) => item.id && item.input && item.output),
);

const manifestJson = computed(() =>
  JSON.stringify(
    {
      version: 1,
      tests: normalizedTestCases.value,
    },
    null,
    2,
  ),
);

const configJson = computed(() =>
  JSON.stringify(
    {
      time_limit_ms: judgeConfig.timeLimitMs,
      memory_limit_kb: judgeConfig.memoryLimitKb,
      checker_type: judgeConfig.checkerType,
      compare_mode: judgeConfig.compareMode,
    },
    null,
    2,
  ),
);

function requireUserId() {
  authStore.initialize();
  if (!authStore.user) {
    throw new Error("User session not found");
  }
  return authStore.user.id;
}

function handleFileChange(file: UploadFile) {
  selectedFile.value = file.raw || null;
}

function addTestCase() {
  testCases.value.push({
    key: makeClientId(),
    id: `test-${testCases.value.length + 1}`,
    score: 0,
    subtaskId: "",
    input: "",
    output: "",
  });
}

function removeTestCase(index: number) {
  if (testCases.value.length === 1) {
    testCases.value[0] = {
      key: makeClientId(),
      id: "",
      score: 0,
      subtaskId: "",
      input: "",
      output: "",
    };
    return;
  }
  testCases.value.splice(index, 1);
}

async function ensureProblemCreated() {
  if (problemId.value) {
    return problemId.value;
  }

  if (!form.title.trim()) {
    throw new Error("Problem title is required");
  }

  creating.value = true;
  try {
    const created = await createProblem({
      title: form.title.trim(),
      owner_id: requireUserId(),
    });
    problemId.value = created.id;
    activeStep.value = Math.max(activeStep.value, 1);
    ElMessage.success(`Problem created: ${created.id}`);
    return created.id;
  } finally {
    creating.value = false;
  }
}

async function handleUpload() {
  if (!selectedFile.value) {
    ElMessage.warning("Data pack is required");
    return;
  }
  if (!normalizedTestCases.value.length) {
    ElMessage.warning("At least one valid testcase is required");
    return;
  }

  uploading.value = true;
  try {
    const ensuredProblemId = await ensureProblemCreated();
    const file = selectedFile.value;
    const dataPackHash = await fileSha256Hex(file);
    const manifestHash = await sha256Hex(new TextEncoder().encode(manifestJson.value));
    const prepared = await prepareUpload(
      ensuredProblemId,
      {
        expected_size_bytes: file.size,
        expected_sha256: dataPackHash,
        content_type: file.type || "application/octet-stream",
        created_by: requireUserId(),
        client_type: "web",
        upload_strategy: "multipart_presigned",
      },
      makeIdempotencyKey(),
    );

    uploadState.uploadId = prepared.upload_id;
    uploadState.version = prepared.version;

    const chunks = splitFile(file, prepared.part_size_bytes);
    const signed = await signParts(
      ensuredProblemId,
      prepared.upload_id,
      chunks.map((item: { partNumber: number }) => item.partNumber),
    );

    const parts: Array<{ part_number: number; etag: string }> = [];
    for (const chunk of chunks) {
      const etag = await uploadChunk(signed.urls[String(chunk.partNumber)], chunk.blob);
      parts.push({
        part_number: chunk.partNumber,
        etag,
      });
    }

    await completeUpload(ensuredProblemId, prepared.upload_id, {
      parts,
      manifest_json: manifestJson.value,
      config_json: configJson.value,
      manifest_hash: manifestHash,
      data_pack_hash: dataPackHash,
    });

    activeStep.value = Math.max(activeStep.value, 2);
    ElMessage.success(`Data pack uploaded for version ${prepared.version}`);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Upload failed");
  } finally {
    uploading.value = false;
  }
}

async function handleAbortUpload() {
  if (!problemId.value || !uploadState.uploadId) {
    return;
  }

  try {
    await abortUpload(problemId.value, uploadState.uploadId);
    uploadState.uploadId = 0;
    uploadState.version = 0;
    ElMessage.success("Upload aborted");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Abort upload failed");
  }
}

async function handleSaveStatement() {
  if (!uploadState.version) {
    ElMessage.warning("Upload data pack first to get version");
    return;
  }

  savingStatement.value = true;
  try {
    const ensuredProblemId = await ensureProblemCreated();
    await updateStatement(ensuredProblemId, uploadState.version, form.statementMd);
    activeStep.value = Math.max(activeStep.value, 3);
    ElMessage.success("Statement saved");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Save statement failed");
  } finally {
    savingStatement.value = false;
  }
}

async function handlePublish() {
  if (!uploadState.version) {
    ElMessage.warning("Version is required");
    return;
  }

  publishing.value = true;
  try {
    const ensuredProblemId = await ensureProblemCreated();
    await publishVersion(ensuredProblemId, uploadState.version);
    ElMessage.success("Problem published");
    await router.push(`/problems/${ensuredProblemId}`);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Publish failed");
  } finally {
    publishing.value = false;
  }
}
</script>
