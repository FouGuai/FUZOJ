import { onBeforeUnmount, ref } from "vue";
import { getSubmissionStatus } from "@/api/submissions";
import type { JudgeStatusData } from "@/api/types";

const finalStatuses = new Set(["Finished", "Failed"]);

export function useSubmissionStream() {
  const status = ref<JudgeStatusData | null>(null);
  const isStreaming = ref(false);
  const activeSubmissionId = ref("");

  let eventSource: EventSource | null = null;
  let pollTimer: number | null = null;

  function reset() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    if (pollTimer) {
      window.clearTimeout(pollTimer);
      pollTimer = null;
    }
    isStreaming.value = false;
  }

  async function poll(submissionId: string) {
    status.value = await getSubmissionStatus(submissionId, "log");
    if (!finalStatuses.has(status.value.status)) {
      pollTimer = window.setTimeout(() => {
        void poll(submissionId);
      }, 2000);
    } else {
      try {
        status.value = await getSubmissionStatus(submissionId, "log");
      } catch {
        // Keep last known status when log hydration fails.
      }
      isStreaming.value = false;
    }
  }

  function connect(submissionId: string) {
    reset();
    activeSubmissionId.value = submissionId;
    isStreaming.value = true;

    const envBase = import.meta.env.VITE_SSE_BASE_URL || import.meta.env.VITE_API_BASE_URL || "";
    const normalizedBase =
      !envBase || envBase === "/" || envBase.startsWith("/")
        ? window.location.origin
        : envBase;
    const url = new URL(`/api/v1/status/submissions/${submissionId}/events?include=details`, normalizedBase);
    eventSource = new EventSource(url.toString(), { withCredentials: false });

    const handleMessage = (event: MessageEvent<string>) => {
      try {
        const payload = JSON.parse(event.data) as {
          data?: JudgeStatusData;
          Data?: JudgeStatusData;
        };
        const nextStatus = payload.data ?? payload.Data ?? null;
        if (nextStatus) {
          status.value = nextStatus;
          if (finalStatuses.has(nextStatus.status)) {
            reset();
            void getSubmissionStatus(submissionId, "log")
              .then((detail) => {
                status.value = detail;
              })
              .catch(() => {
                // Keep SSE final payload when detail query fails.
              });
          }
        }
      } catch {
        void poll(submissionId);
      }
    };

    eventSource.addEventListener("snapshot", handleMessage);
    eventSource.addEventListener("update", handleMessage);
    eventSource.addEventListener("final", handleMessage);
    eventSource.onerror = () => {
      reset();
      void poll(submissionId);
    };
  }

  onBeforeUnmount(() => {
    reset();
  });

  return {
    status,
    isStreaming,
    activeSubmissionId,
    connect,
    reset,
  };
}
