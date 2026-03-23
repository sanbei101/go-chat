<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";
import { LogOut, MessageSquare, Plus, Send, Users } from "lucide-vue-next";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Textarea } from "@/components/ui/textarea";
import { useChatStore } from "@/stores/chat";

const router = useRouter();
const chatStore = useChatStore();
const messageListRef = ref<HTMLElement | null>(null);
const createRoomName = ref("");
const draft = ref("");

const {
  auth,
  roomLoading,
  roomsLoading,
  sending,
  roomError,
  messageError,
  rooms,
  members,
  messages,
  activeRoomId,
  activeRoom,
} = storeToRefs(chatStore);

async function init() {
  const ok = await chatStore.ensureReady();
  if (!ok) {
    router.replace({ name: "auth" });
  }
}

async function handleCreateRoom() {
  await chatStore.createRoom(createRoomName.value);
  createRoomName.value = "";
}

function handleSendMessage() {
  const sent = chatStore.sendMessage(draft.value);
  if (sent) {
    draft.value = "";
  }
}

function handleLogout() {
  chatStore.logout();
  router.replace({ name: "auth" });
}

watch(
  messages,
  async () => {
    await nextTick();
    const element = messageListRef.value;
    if (element) {
      element.scrollTop = element.scrollHeight;
    }
  },
  { deep: true },
);

onMounted(init);

onUnmounted(() => {
  chatStore.disconnectRoom();
});
</script>

<template>
  <main
    class="h-screen overflow-hidden bg-[radial-gradient(circle_at_top,rgba(0,0,0,0.04),transparent_35%),linear-gradient(180deg,#fafaf9_0%,#f4f4f0_100%)] px-4 py-6 text-foreground md:px-6"
  >
    <div class="mx-auto flex h-[calc(100vh-3rem)] max-w-7xl flex-col gap-4">
      <section
        class="flex items-center justify-between rounded-2xl border bg-background/80 px-4 py-3 shadow-sm backdrop-blur md:px-5"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex size-10 items-center justify-center rounded-xl bg-primary text-primary-foreground"
          >
            <MessageSquare class="size-5" />
          </div>
          <div>
            <h1 class="text-lg font-semibold">Go Chat</h1>
            <p class="text-sm text-muted-foreground">简洁的房间式聊天</p>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <Badge variant="secondary">{{ auth?.username }}</Badge>
          <Button variant="ghost" size="icon" @click="handleLogout">
            <LogOut class="size-4" />
          </Button>
        </div>
      </section>

      <section
        class="grid min-h-0 flex-1 gap-4 lg:grid-cols-[280px_minmax(0,1fr)]"
      >
        <Card
          class="flex min-h-0 flex-col overflow-hidden bg-background/88 backdrop-blur"
        >
          <div class="space-y-4 p-4">
            <div>
              <p class="text-sm font-medium">房间</p>
              <p class="text-sm text-muted-foreground">创建或切换聊天室</p>
            </div>

            <div class="flex gap-2">
              <Input
                v-model="createRoomName"
                placeholder="new-room"
                @keyup.enter="handleCreateRoom"
              />
              <Button
                size="icon"
                :disabled="roomLoading"
                @click="handleCreateRoom"
              >
                <Plus class="size-4" />
              </Button>
            </div>

            <p v-if="roomError" class="text-sm text-destructive">
              {{ roomError }}
            </p>
          </div>

          <Separator />

          <div class="flex-1 space-y-2 overflow-y-auto p-3">
            <Button
              v-for="room in rooms"
              :key="room.id"
              variant="ghost"
              class="h-auto w-full justify-start rounded-xl border px-3 py-3 text-left transition"
              :class="
                room.id === activeRoomId
                  ? 'border-primary bg-primary/5 hover:bg-primary/5'
                  : 'bg-background hover:bg-secondary/70'
              "
              @click="chatStore.activeRoomId = room.id"
            >
              <div class="flex w-full items-center justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate font-medium">{{ room.name }}</p>
                  <p class="text-xs text-muted-foreground">
                    room #{{ room.id }}
                  </p>
                </div>

                <Badge variant="outline">
                  在线 {{ room.id === activeRoomId ? members.length : "-" }}
                </Badge>
              </div>
            </Button>

            <div
              v-if="!rooms.length && !roomsLoading"
              class="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground"
            >
              还没有聊天室
            </div>
          </div>
        </Card>

        <Card
          class="flex min-h-0 flex-col overflow-hidden bg-background/88 backdrop-blur"
        >
          <div class="flex items-center justify-between gap-3 p-4">
            <div>
              <h2 class="text-lg font-semibold">
                {{ activeRoom?.name || "选择一个房间" }}
              </h2>
              <p class="text-sm text-muted-foreground">
                {{
                  activeRoom
                    ? `房间 ID ${activeRoom.id}`
                    : "加入后即可开始消息收发"
                }}
              </p>
            </div>
            <div
              class="flex items-center gap-2 rounded-full bg-secondary px-3 py-1.5 text-sm text-secondary-foreground"
            >
              <Users class="size-4" />
              <span>{{ members.length }}</span>
            </div>
          </div>

          <Separator />

          <div
            class="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_220px]"
          >
            <div class="flex min-h-0 flex-col">
              <div
                ref="messageListRef"
                class="flex-1 space-y-3 overflow-y-auto p-4"
              >
                <template v-if="messages.length">
                  <div
                    v-for="(message, index) in messages"
                    :key="`${message.user_id}-${index}-${message.content}`"
                    class="flex"
                    :class="
                      message.user_id === auth?.userId
                        ? 'justify-end'
                        : 'justify-start'
                    "
                  >
                    <div
                      class="max-w-[85%] rounded-2xl px-4 py-3"
                      :class="
                        message.user_id === auth?.userId
                          ? 'bg-primary text-primary-foreground'
                          : 'border bg-secondary/40'
                      "
                    >
                      <p class="mb-1 text-xs opacity-70">
                        {{ message.username }}
                      </p>
                      <p class="whitespace-pre-wrap wrap-break-word text-sm">
                        {{ message.content }}
                      </p>
                    </div>
                  </div>
                </template>

                <div
                  v-else
                  class="flex h-full min-h-70 items-center justify-center rounded-2xl border border-dashed text-sm text-muted-foreground"
                >
                  {{ activeRoom ? "发送第一条消息" : "先选择或创建房间" }}
                </div>
              </div>

              <Separator />

              <div class="p-4">
                <form class="space-y-3" @submit.prevent="handleSendMessage">
                  <Textarea
                    v-model="draft"
                    :disabled="!activeRoomId"
                    class="min-h-28 resize-none"
                    placeholder="输入消息，按发送提交"
                  />
                  <div class="flex items-center justify-between gap-3">
                    <p v-if="messageError" class="text-sm text-destructive">
                      {{ messageError }}
                    </p>
                    <p v-else class="text-sm text-muted-foreground">
                      {{
                        activeRoom ? `发送到 ${activeRoom.name}` : "未连接房间"
                      }}
                    </p>
                    <Button
                      type="submit"
                      :disabled="!activeRoomId || !draft.trim() || sending"
                    >
                      <Send class="size-4" />
                      发送
                    </Button>
                  </div>
                </form>
              </div>
            </div>

            <div
              class="min-h-0 overflow-y-auto border-t p-4 lg:border-t-0 lg:border-l"
            >
              <div class="mb-4 flex items-center justify-between">
                <p class="font-medium">在线成员</p>
                <Badge variant="secondary">{{ members.length }}</Badge>
              </div>

              <div class="space-y-2">
                <div
                  v-for="member in members"
                  :key="member.id"
                  class="flex items-center justify-between rounded-xl bg-secondary/50 px-3 py-2"
                >
                  <span class="truncate text-sm">{{ member.username }}</span>
                  <Badge
                    :variant="
                      member.id === auth?.userId ? 'default' : 'outline'
                    "
                  >
                    {{ member.id === auth?.userId ? "你" : "在线" }}
                  </Badge>
                </div>

                <div
                  v-if="!members.length"
                  class="rounded-xl border border-dashed px-3 py-6 text-center text-sm text-muted-foreground"
                >
                  暂无在线成员
                </div>
              </div>
            </div>
          </div>
        </Card>
      </section>
    </div>
  </main>
</template>
