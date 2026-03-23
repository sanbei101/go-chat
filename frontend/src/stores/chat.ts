import { computed, ref, watch } from "vue"
import { defineStore } from "pinia"
import { client } from "@/lib/api"

interface AuthResponse {
  accessToken: string
  username: string
  id: string
}

export interface AuthState {
  token: string
  userId: string
  username: string
}

export interface Room {
  id: string
  name: string
}

export interface Member {
  id: string
  username: string
}

export interface ChatMessage {
  content: string
  room_id: string
  username: string
  user_id: string
}

function ensureArray<T>(value: T[] | null | undefined) {
  return Array.isArray(value) ? value : []
}

async function request<T>(promise: Promise<T>) {
  try {
    return await promise
  } catch (error) {
    throw new Error(error instanceof Error ? error.message : "Request failed")
  }
}

export const useChatStore = defineStore("chat", () => {
  const auth = ref<AuthState | null>(null)
  const roomLoading = ref(false)
  const roomsLoading = ref(false)
  const sending = ref(false)
  const authError = ref("")
  const roomError = ref("")
  const messageError = ref("")
  const rooms = ref<Room[]>([])
  const members = ref<Member[]>([])
  const messages = ref<ChatMessage[]>([])
  const activeRoomId = ref("")
  const connectedRoomId = ref("")
  const initialized = ref(false)

  let socket: WebSocket | null = null
  let membersTimer: number | null = null

  const isLoggedIn = computed(() => Boolean(auth.value?.token))
  const activeRoom = computed(() => rooms.value.find((room) => room.id === activeRoomId.value) || null)

  function clearTimers() {
    if (membersTimer) {
      window.clearInterval(membersTimer)
      membersTimer = null
    }
  }

  function disconnectRoom() {
    clearTimers()
    if (socket) {
      socket.close()
      socket = null
    }
    connectedRoomId.value = ""
  }

  function resetChatState() {
    disconnectRoom()
    rooms.value = []
    members.value = []
    messages.value = []
    activeRoomId.value = ""
    connectedRoomId.value = ""
    initialized.value = false
    roomError.value = ""
    messageError.value = ""
  }

  function getWsURL(roomId: string) {
    const customBase = import.meta.env.VITE_WS_BASE_URL

    if (customBase) {
      return `${customBase}/joinRoom/${roomId}?userID=${auth.value?.userId}&username=${encodeURIComponent(auth.value?.username || "")}`
    }

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
    return `${protocol}//${window.location.host}/joinRoom/${roomId}?userID=${auth.value?.userId}&username=${encodeURIComponent(auth.value?.username || "")}`
  }

  async function loadMembers(roomId: string) {
    if (!auth.value) return
    const response = await request(client.Get<Member[]>(`/ws/getClients/${roomId}`).send())
    members.value = ensureArray(response)
  }

  async function joinRoom(roomId: string) {
    if (!auth.value || connectedRoomId.value === roomId) return

    roomError.value = ""
    disconnectRoom()
    activeRoomId.value = roomId
    connectedRoomId.value = roomId
    members.value = []
    messages.value = []

    try {
      await loadMembers(roomId)
    } catch (error) {
      roomError.value = error instanceof Error ? error.message : "Failed to load members"
    }

    socket = new WebSocket(getWsURL(roomId))

    socket.onmessage = (event) => {
      const message = JSON.parse(event.data) as ChatMessage
      messages.value.push(message)
      if (message.room_id === activeRoomId.value) {
        loadMembers(activeRoomId.value).catch(() => {})
      }
    }

    socket.onopen = () => {
      loadMembers(roomId).catch(() => {})
      membersTimer = window.setInterval(() => {
        if (activeRoomId.value) {
          loadMembers(activeRoomId.value).catch(() => {})
        }
      }, 4000)
    }

    socket.onclose = () => {
      clearTimers()
      if (connectedRoomId.value === roomId) {
        connectedRoomId.value = ""
      }
    }
  }

  async function loadRooms(selectFirst = false) {
    if (!auth.value) return

    roomsLoading.value = true
    try {
      const response = await request(client.Get<Room[]>("/ws/getRooms").send())
      const nextRooms = ensureArray(response)
      rooms.value = nextRooms
      initialized.value = true

      if (!nextRooms.length) {
        activeRoomId.value = ""
        members.value = []
        messages.value = []
        disconnectRoom()
        return
      }

      if (!activeRoomId.value || !nextRooms.some((room) => room.id === activeRoomId.value) || selectFirst) {
        await joinRoom(nextRooms[0].id)
      }
    } finally {
      roomsLoading.value = false
    }
  }

  async function authenticate(
    mode: "login" | "signup",
    payload: { username: string; email: string; password: string },
  ) {
    authError.value = ""

    try {
      if (mode === "signup") {
        await request(
          client.Post("/signup", {
            username: payload.username,
            email: payload.email,
            password: payload.password,
          }).send(),
        )
      }

      const response = await request<AuthResponse>(
        client.Post<AuthResponse>("/login", {
          email: payload.email,
          password: payload.password,
        }).send(),
      )

      auth.value = {
        token: response.accessToken,
        userId: response.id,
        username: response.username,
      }
      localStorage.setItem("token", response.accessToken)

      await loadRooms(true)
      return true
    } catch (error) {
      authError.value = error instanceof Error ? error.message : "Authentication failed"
      return false
    }
  }

  async function createRoom(name: string) {
    if (!name.trim()) return

    roomLoading.value = true
    roomError.value = ""
    try {
      const room = await request<Room>(
        client.Post<Room>("/ws/createRoom", { name: name.trim() }).send(),
      )
      await loadRooms()
      await joinRoom(room.id)
    } catch (error) {
      roomError.value = error instanceof Error ? error.message : "Failed to create room"
    } finally {
      roomLoading.value = false
    }
  }

  function sendMessage(content: string) {
    if (!socket || socket.readyState !== WebSocket.OPEN || !content.trim()) return false
    sending.value = true
    messageError.value = ""
    socket.send(content.trim())
    sending.value = false
    return true
  }

  async function ensureReady() {
    if (!auth.value) return false
    if (initialized.value) return true
    await loadRooms(true)
    return true
  }

  function logout() {
    auth.value = null
    authError.value = ""
    localStorage.removeItem("token")
    resetChatState()
  }

  watch(activeRoomId, (roomId, previousRoomId) => {
    if (roomId && roomId !== previousRoomId) {
      joinRoom(roomId).catch((error) => {
        roomError.value = error instanceof Error ? error.message : "Failed to join room"
      })
    }
  })

  return {
    auth,
    roomLoading,
    roomsLoading,
    sending,
    authError,
    roomError,
    messageError,
    rooms,
    members,
    messages,
    activeRoomId,
    initialized,
    isLoggedIn,
    activeRoom,
    authenticate,
    createRoom,
    sendMessage,
    ensureReady,
    joinRoom,
    disconnectRoom,
    logout,
  }
})
