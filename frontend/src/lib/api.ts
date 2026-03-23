import { createAlova } from "alova";
import adapterFetch from "alova/fetch";
const baseURL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8800";
export const client = createAlova({
  baseURL,
  requestAdapter: adapterFetch(),
  beforeRequest: method => {
    const token = localStorage.getItem("token");

    method.config.headers = {
      ...(method.config.headers || {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    };
  },
});
