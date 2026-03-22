<template>
  <div class="auth-page">
    <section class="auth-page__hero">
      <p class="auth-page__eyebrow">Welcome Back</p>
      <h1>登录后进入题库与提交中心</h1>
      <p class="auth-page__description">
        浏览题目、在线提交代码、查看判题结果，并在需要时进入题目管理页面。
      </p>
    </section>

    <section class="auth-card">
      <div class="auth-card__tabs">
        <button :class="{ active: mode === 'login' }" @click="mode = 'login'">登录</button>
        <button :class="{ active: mode === 'register' }" @click="mode = 'register'">注册</button>
      </div>

      <el-form label-position="top" @submit.prevent>
        <el-form-item label="Username">
          <el-input v-model="form.username" placeholder="demo" />
        </el-form-item>
        <el-form-item label="Password">
          <el-input v-model="form.password" type="password" show-password placeholder="secret" />
        </el-form-item>
        <el-button type="primary" class="auth-card__submit" :loading="submitting" @click="submit">
          {{ mode === "login" ? "登录并进入题库" : "创建账号" }}
        </el-button>
      </el-form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

const mode = ref<"login" | "register">("login");
const submitting = ref(false);
const form = reactive({
  username: "",
  password: "",
});

async function submit() {
  if (!form.username || !form.password) {
    ElMessage.warning("Please fill username and password");
    return;
  }

  submitting.value = true;
  try {
    if (mode.value === "login") {
      await authStore.login(form.username, form.password);
    } else {
      await authStore.register(form.username, form.password);
    }

    ElMessage.success(mode.value === "login" ? "Login success" : "Register success");
    const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/problems";
    await router.push(redirect);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "Authentication failed");
  } finally {
    submitting.value = false;
  }
}
</script>
