<script setup>
import { useRoute } from 'vue-router'
import NavBar from './components/NavBar.vue'
import SiteFooter from './components/SiteFooter.vue'

const route = useRoute()
const showChrome = () => !route.meta.hideChrome
</script>

<template>
  <NavBar v-if="showChrome()" />
  <router-view />
  <SiteFooter v-if="showChrome()" />
</template>
