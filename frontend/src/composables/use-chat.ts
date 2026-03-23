import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue"
import { client } from "@/lib/api"

interface AuthResponse {
  accessToken: string
  username: string
  id: string
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

export interface AuthState {
  token: string
  userId: string
  username: string
}

const storageKey = "go-chat-auth"

const authMode = ref<"login" | "signup">("login")
const authLoading = ref(false)
const roomLoading = ref(false)
const sending = ref(false)
const roomsLoading = ref(false)
const authError = ref("")
const roomError = ref("")
const messageError = ref("")

const authForm = ref({
  username: "",
  email: "",
  password: "",
})

const createRoomName = ref("")
const draft = ref("")
const rooms = ref<Room[]>([])
const members = ref<Member[]>([])
const messages = ref<ChatMessage[]>([])
const activeRoomId = ref("")
const connectedRoomId = ref("")
const auth = ref<AuthState | null>(null)

let socket: WebSocket | null = null
let membersTimer: number | null = null
let sessionBootstrapped = false

function ensureArray<T>(value: T[] | null | undefined) {
  return Array.isArray(value) ? value : []
}

export function hasStoredAuth() {
  return Boolean(localStorage.getItem(storageKey))
}

function readStoredAuth() {
  const raw = localStorage.getItem(storageKey)
  if (!raw) return null

  try {
    return JSON.parse(raw) as AuthState
  } catch {
    localStorage.removeItem(storageKey)
    return null
  }
}

function persistAuth(nextAuth: AuthState | null) {
  auth.value = nextAuth

  if (nextAuth) {
    localStorage.setItem(storageKey, JSON.stringify(nextAuth))
    localStorage.setItem("go-chat-token", nextAuth.token)
    return
  }

  localStorage.removeItem(storageKey)
  localStorage.removeItem("go-chat-token")
}

function resetChatState() {
  disconnectRoom()
  rooms.value = []
  members.value = []
  messages.value = []
  activeRoomId.value = ""
  connectedRoomId.value = ""
  createRoomName.value = ""
  draft.value = ""
}

function disconnectRoom() {
  if (membersTimer) {
    window.clearInterval(membersTimer)
    membersTimer = null
  }

  if (socket) {
    socket.close()
    socket = null
  }

  connectedRoomId.value = ""
}

function getWsURL(roomId: string) {
  const customBase = import.meta.env.VITE_WS_BASE_URL

  if (customBase) {
    return `${customBase}/joinRoom/${roomId}?userID=${auth.value?.userId}&username=${encodeURIComponent(auth.value?.username || "")}`
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
  const host = window.location.host
  return `${protocol}//${host}/joinRoom/${roomId}?userID=${auth.value?.userId}&username=${encodeURIComponent(auth.value?.username || "")}`
}

async function request<T>(promise: Promise<T>) {
  try {
    return await promise
  } catch (error) {
    throw new Error(error instanceof Error ? error.message : "Request failed")
  }
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
    if (membersTimer) {
      window.clearInterval(membersTimer)
      membersTimer = null
    }
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

async function submitAuth() {
  authLoading.value = true
  authError.value = ""

  try {
    if (authMode.value === "signup") {
      await request(
        client.Post("/signup", {
          username: authForm.value.username,
          email: authForm.value.email,
          password: authForm.value.password,
        }).send(),
      )
    }

    const response = await request<AuthResponse>(
      client.Post<AuthResponse>("/login", {
        email: authForm.value.email,
        password: authForm.value.password,
      }).send(),
    )

    persistAuth({
      token: response.accessToken,
      userId: response.id,
      username: response.username,
    })

    authForm.value.password = ""
    sessionBootstrapped = true
    await loadRooms(true)
    return true
  } catch (error) {
    authError.value = error instanceof Error ? error.message : "Authentication failed"
    return false
  } finally {
    authLoading.value = false
  }
}

async function bootstrapSession() {
  if (sessionBootstrapped && auth.value) {
    return true
  }

  const savedAuth = readStoredAuth()
  if (!savedAuth) {
    return false
  }

  persistAuth(savedAuth)

  try {
    await request(client.Get("/ws/auth").send())
    await loadRooms(true)
    sessionBootstrapped = true
    return true
  } catch {
    logout()
    return false
  }
}

async function createRoom() {
  if (!createRoomName.value.trim()) return

  roomLoading.value = true
  roomError.value = ""

  try {
    const room = await request<Room>(
      client.Post<Room>("/ws/createRoom", { name: createRoomName.value.trim() }).send(),
    )

    createRoomName.value = ""
    await loadRooms()
    await joinRoom(room.id)
  } catch (error) {
    roomError.value = error instanceof Error ? error.message : "Failed to create room"
  } finally {
    roomLoading.value = false
  }
}

function sendMessage() {
  if (!socket || socket.readyState !== WebSocket.OPEN || !draft.value.trim()) return
  sending.value = true
  messageError.value = ""
  socket.send(draft.value.trim())
  draft.value = ""
  sending.value = false
}

function logout() {
  sessionBootstrapped = false
  persistAuth(null)
  resetChatState()
}

watch(activeRoomId, (roomId, previousRoomId) => {
  if (roomId && roomId !== previousRoomId) {
    joinRoom(roomId).catch((error) => {
      roomError.value = error instanceof Error ? error.message : "Failed to join room"
    })
  }
})

export function useChatPage(messageListRef?: { value: HTMLElement | null }) {
  if (messageListRef) {
    watch(
      messages,
      async () => {
        await nextTick()
        const element = messageListRef.value
        if (element) {
          element.scrollTop = element.scrollHeight
        }
      },
      { deep: true },
    )
  }

  onBeforeUnmount(() => {
    disconnectRoom()
  })

  return {
    auth,
    roomLoading,
    roomsLoading,
    sending,
    roomError,
    messageError,
    createRoomName,
    draft,
    rooms,
    members,
    messages,
    activeRoomId,
    activeRoom: computed(() => rooms.value.find((room) => room.id === activeRoomId.value) || null),
    bootstrapSession,
    createRoom,
    sendMessage,
    logout,
  }
}

export function useAuthPage() {
  return {
    authMode,
    authLoading,
    authError,
    authForm,
    submitAuth,
  }
}
