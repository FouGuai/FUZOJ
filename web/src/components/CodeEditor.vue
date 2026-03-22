<template>
  <div class="editor-shell">
    <div ref="containerRef" class="editor-shell__container"></div>
  </div>
</template>

<script setup lang="ts">
import loader from "@monaco-editor/loader";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    language: string;
  }>(),
  {
    modelValue: "",
    language: "cpp",
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const containerRef = ref<HTMLElement | null>(null);
let editor: import("monaco-editor").editor.IStandaloneCodeEditor | null = null;

onMounted(async () => {
  const monaco = await loader.init();
  if (!containerRef.value) {
    return;
  }

  editor = monaco.editor.create(containerRef.value, {
    value: props.modelValue,
    language: props.language,
    automaticLayout: true,
    minimap: {
      enabled: false,
    },
    fontSize: 14,
    lineHeight: 22,
    scrollBeyondLastLine: false,
    roundedSelection: true,
    smoothScrolling: true,
    tabSize: 2,
    theme: "vs",
  });

  editor.onDidChangeModelContent(() => {
    emit("update:modelValue", editor?.getValue() || "");
  });
});

watch(
  () => props.modelValue,
  (value) => {
    if (editor && value !== editor.getValue()) {
      editor.setValue(value);
    }
  },
);

watch(
  () => props.language,
  async (language) => {
    if (!editor) {
      return;
    }
    const monaco = await loader.init();
    const model = editor.getModel();
    if (model) {
      monaco.editor.setModelLanguage(model, language);
    }
  },
);

onBeforeUnmount(() => {
  editor?.dispose();
});
</script>
