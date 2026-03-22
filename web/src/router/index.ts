import { createRouter, createWebHistory } from "vue-router";
import AppShell from "@/layouts/AppShell.vue";
import LoginPage from "@/pages/LoginPage.vue";
import ProblemCreatePage from "@/pages/ProblemCreatePage.vue";
import ProblemListPage from "@/pages/ProblemListPage.vue";
import ProblemWorkspacePage from "@/pages/ProblemWorkspacePage.vue";
import { useAuthStore } from "@/stores/auth";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/login",
      name: "login",
      component: LoginPage,
      meta: { public: true },
    },
    {
      path: "/",
      component: AppShell,
      children: [
        {
          path: "",
          redirect: "/problems",
        },
        {
          path: "problems",
          name: "problems",
          component: ProblemListPage,
          meta: { public: true },
        },
        {
          path: "problems/:id",
          name: "problem-workspace",
          component: ProblemWorkspacePage,
          meta: { public: true },
        },
        {
          path: "problemset/new",
          name: "problem-create",
          component: ProblemCreatePage,
          meta: { requiresAuth: true },
        },
      ],
    },
  ],
});

router.beforeEach((to) => {
  const authStore = useAuthStore();
  authStore.initialize();

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return {
      name: "login",
      query: {
        redirect: to.fullPath,
      },
    };
  }

  if (to.name === "login" && authStore.isAuthenticated) {
    return { name: "problems" };
  }

  return true;
});

export default router;
