import { defineStore } from "pinia";

const STORAGE_KEY = "fuzoj-web-drafts";

type DraftState = Record<string, string>;

function loadDrafts(): DraftState {
  if (typeof window === "undefined") {
    return {};
  }
  try {
    return JSON.parse(window.localStorage.getItem(STORAGE_KEY) || "{}") as DraftState;
  } catch {
    return {};
  }
}

export const useProblemDraftStore = defineStore("problem-drafts", {
  state: () => ({
    drafts: loadDrafts(),
  }),
  actions: {
    getDraft(problemId: number, languageId: string) {
      return this.drafts[`${problemId}:${languageId}`] || "";
    },
    setDraft(problemId: number, languageId: string, value: string) {
      this.drafts[`${problemId}:${languageId}`] = value;
      if (typeof window !== "undefined") {
        window.localStorage.setItem(STORAGE_KEY, JSON.stringify(this.drafts));
      }
    },
  },
});
