import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// В режиме разработки (npm run dev) запросы /api уходят на Go-сервер.
// В Docker то же самое делает nginx (см. nginx.conf), поэтому кука сессии работает одинаково.
export default defineConfig({
  plugins: [react()],
  server: {
    host: true, // открывать dev-сервер и с телефона в той же сети
    proxy: {
      "/api": { target: "http://localhost:8080", ws: true },
    },
  },
});
