import { createAlova } from "alova"
import adapterFetch from "alova/fetch"

const baseURL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8800"

function getToken() {
  return localStorage.getItem("go-chat-token") || ""
}

export const client = createAlova({
  baseURL,
  requestAdapter: adapterFetch(),
  cacheFor: null,
  cacheLogger: false,
  beforeRequest(method) {
    const token = getToken()
    if (token) {
      method.config.headers = {
        ...(method.config.headers || {}),
        Authorization: `Bearer ${token}`,
      }
    }
  }
})
