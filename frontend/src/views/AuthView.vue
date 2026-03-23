<script setup lang="ts">
import { reactive, ref } from "vue"
import { storeToRefs } from "pinia"
import { MessageSquare } from "lucide-vue-next"
import { useRouter } from "vue-router"
import Button from "@/components/ui/button/Button.vue"
import Badge from "@/components/ui/badge/Badge.vue"
import Card from "@/components/ui/card/Card.vue"
import Input from "@/components/ui/input/Input.vue"
import { useChatStore } from "@/stores/chat"

const router = useRouter()
const chatStore = useChatStore()
const { authError } = storeToRefs(chatStore)

const authMode = ref<"login" | "signup">("login")
const authLoading = ref(false)
const authForm = reactive({
  username: "",
  email: "",
  password: "",
})

async function handleSubmit() {
  authLoading.value = true
  const ok = await chatStore.authenticate(authMode.value, authForm)
  authLoading.value = false
  if (ok) {
    router.push({ name: "chat" })
  }
}
</script>

<template>
  <main class="min-h-screen bg-linear-to-b from-white to-stone-50 px-4 py-6 text-foreground md:px-6">
    <div class="mx-auto flex min-h-[calc(100vh-3rem)] max-w-7xl flex-col gap-4">
      <section class="flex items-center rounded-2xl border bg-background/80 px-4 py-3 shadow-sm backdrop-blur md:px-5">
        <div class="flex items-center gap-3">
          <div class="flex size-10 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <MessageSquare class="size-5" />
          </div>
          <div>
            <h1 class="text-lg font-semibold">Go Chat</h1>
            <p class="text-sm text-muted-foreground">简洁的房间式聊天</p>
          </div>
        </div>
      </section>

      <section class="grid flex-1 place-items-center">
        <Card class="w-full max-w-md border-0 bg-background/90 p-6 shadow-lg shadow-black/5">
          <div class="mb-6 flex items-center justify-between">
            <div>
              <h2 class="text-xl font-semibold">{{ authMode === "login" ? "登录" : "注册" }}</h2>
              <p class="text-sm text-muted-foreground">连接到 `backend/api.md` 定义的聊天室 API</p>
            </div>
            <Badge variant="outline">{{ authMode }}</Badge>
          </div>

          <div class="mb-4 flex rounded-lg bg-secondary p-1">
            <button
              type="button"
              class="flex-1 rounded-md px-3 py-2 text-sm transition"
              :class="authMode === 'login' ? 'bg-background shadow-sm' : 'text-muted-foreground'"
              @click="authMode = 'login'"
            >
              登录
            </button>
            <button
              type="button"
              class="flex-1 rounded-md px-3 py-2 text-sm transition"
              :class="authMode === 'signup' ? 'bg-background shadow-sm' : 'text-muted-foreground'"
              @click="authMode = 'signup'"
            >
              注册
            </button>
          </div>

          <form class="space-y-3" @submit.prevent="handleSubmit">
            <Input v-if="authMode === 'signup'" v-model="authForm.username" placeholder="用户名" />
            <Input v-model="authForm.email" placeholder="邮箱" type="email" />
            <Input v-model="authForm.password" placeholder="密码" type="password" />
            <p v-if="authError" class="text-sm text-destructive">{{ authError }}</p>
            <Button type="submit" class="w-full" :disabled="authLoading">
              {{ authLoading ? "处理中..." : authMode === "login" ? "进入聊天室" : "注册并登录" }}
            </Button>
          </form>
        </Card>
      </section>
    </div>
  </main>
</template>
