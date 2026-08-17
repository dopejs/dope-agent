import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Served from the custom Pages domain (agent.dopejs.com) at the root.
export default defineConfig({
  base: "/",
  plugins: [react()],
});
