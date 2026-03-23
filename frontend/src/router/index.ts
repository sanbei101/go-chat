import { createRouter, createWebHistory } from "vue-router"
import AuthView from "@/views/AuthView.vue"
import ChatView from "@/views/ChatView.vue"
import { hasStoredAuth } from "@/composables/use-chat"

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "auth",
      component: AuthView,
      meta: { guestOnly: true },
    },
    {
      path: "/chat",
      name: "chat",
      component: ChatView,
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to) => {
  const authed = hasStoredAuth()

  if (to.meta.requiresAuth && !authed) {
    return { name: "auth" }
  }

  if (to.meta.guestOnly && authed) {
    return { name: "chat" }
  }

  return true
})

export default router
