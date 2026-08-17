import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Served from https://<owner>.github.io/dope-agent/
export default defineConfig({
  base: "/dope-agent/",
  plugins: [react()],
});
