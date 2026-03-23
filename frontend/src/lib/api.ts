import { createAlova } from "alova";
import adapterFetch from "alova/fetch";
const baseURL = import.meta.env.VITE_API_BASE_URL as string || "http://localhost:8800";

function readToken() {
  const token = localStorage.getItem("token");
  if (!token || token === "undefined" || token === "null") {
    return "";
  }
  return token;
}

export const client = createAlova({
  baseURL,
  requestAdapter: adapterFetch(),
  beforeRequest: method => {
    const path = `${method.url ?? ""}`;
    const isAnonymousEndpoint = path.endsWith("/login") || path.endsWith("/signup");
    const token = isAnonymousEndpoint ? "" : readToken();

    method.config.headers = {
      ...(method.config.headers || {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    };
  },
  responded: {
    onSuccess: async response => {
      const contentType = response.headers.get("content-type") || "";
      if (contentType.includes("application/json")) {
        return response.json();
      }
      return response.text();
    },
  },
});
