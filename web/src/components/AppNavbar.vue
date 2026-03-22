<template>
  <header class="navbar">
    <div class="navbar__brand" @click="goHome">
      <div class="navbar__logo">F</div>
      <div>
        <div class="navbar__title">FuzOJ</div>
        <div class="navbar__subtitle">Online Judge Platform</div>
      </div>
    </div>

    <nav class="navbar__links">
      <RouterLink to="/problems" class="navbar__link">题库</RouterLink>
      <RouterLink to="/problemset/new" class="navbar__link">添加题目</RouterLink>
    </nav>

    <div class="navbar__actions">
      <template v-if="authStore.isAuthenticated">
        <div class="navbar__user">
          <span class="navbar__username">{{ authStore.user?.username }}</span>
          <span class="navbar__role">{{ authStore.user?.role }}</span>
        </div>
        <el-button text @click="handleLogout">退出</el-button>
      </template>
      <template v-else>
        <el-button type="primary" @click="goLogin">登录</el-button>
      </template>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import { RouterLink, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";

const router = useRouter();
const authStore = useAuthStore();

function goHome() {
  router.push("/problems");
}

function goLogin() {
  router.push("/login");
}

async function handleLogout() {
  try {
    await authStore.logout();
    router.push("/login");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Logout failed");
  }
}
</script>
