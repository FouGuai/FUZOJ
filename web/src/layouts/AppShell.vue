<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="app-shell__main">
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
import { RouterView } from "vue-router";
import AppNavbar from "@/components/AppNavbar.vue";
</script>
