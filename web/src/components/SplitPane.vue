<template>
  <div class="split-pane" :class="{ 'split-pane--column': stacked }">
    <div class="split-pane__primary" :style="primaryStyle">
      <slot name="primary" />
    </div>
    <div class="split-pane__divider" @mousedown="beginDrag" />
    <div class="split-pane__secondary">
      <slot name="secondary" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";

const props = withDefaults(
  defineProps<{
    initialPrimaryWidth?: number;
    stacked?: boolean;
  }>(),
  {
    initialPrimaryWidth: 46,
    stacked: false,
  },
);

const width = ref(props.initialPrimaryWidth);

const primaryStyle = computed(() =>
  props.stacked ? undefined : { width: `${Math.max(28, Math.min(width.value, 72))}%` },
);

function moveHandler(event: MouseEvent) {
  width.value = (event.clientX / window.innerWidth) * 100;
}

function endDrag() {
  window.removeEventListener("mousemove", moveHandler);
  window.removeEventListener("mouseup", endDrag);
}

function beginDrag() {
  if (props.stacked) {
    return;
  }
  window.addEventListener("mousemove", moveHandler);
  window.addEventListener("mouseup", endDrag);
}

onBeforeUnmount(() => {
  endDrag();
});
</script>
