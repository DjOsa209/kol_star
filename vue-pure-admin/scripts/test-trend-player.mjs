import assert from "node:assert/strict";
import { createServer } from "vite";
import vue from "@vitejs/plugin-vue";
import { createSSRApp } from "vue";
import { renderToString } from "vue/server-renderer";
import { createI18n } from "vue-i18n";

// Isolate the player from authenticated HTTP/image loading. This scenario has
// no CDN URL, so it must still offer playback using the public TikTok item ID.
const server = await createServer({
  configFile: false,
  optimizeDeps: { noDiscovery: true, entries: [] },
  server: { middlewareMode: true },
  plugins: [
    {
      name: "player-test-boundaries",
      enforce: "pre",
      resolveId(id) {
        if (
          id === "@/api/trends" ||
          id === "./TrendImage.vue" ||
          id.startsWith("~icons/")
        )
          return "\0test:" + id;
        if (id === "@/utils/trendLinks")
          return new URL("../src/utils/trendLinks.ts", import.meta.url)
            .pathname;
      },
      load(id) {
        if (id === "\0test:@/api/trends")
          return 'export const safeTrendVideoURL = () => "";';
        if (id.startsWith("\0test:"))
          return "export default { render() { return null; } };";
      }
    },
    vue()
  ]
});
try {
  const { default: Player } = await server.ssrLoadModule(
    "/src/views/business/trends/TrendVideoPlayer.vue"
  );
  const app = createSSRApp(Player, { itemID: "6718335390845095173" });
  app.use(createI18n({ legacy: false, locale: "zh", messages: { zh: {} } }));
  const html = await renderToString(app);
  assert.match(
    html,
    /<iframe[^>]+src="https:\/\/www\.tiktok\.com\/player\/v1\/6718335390845095173/
  );
  assert.doesNotMatch(html, /视频暂不可播放/);
  console.log(
    "PASS: a collected item ID without a CDN URL renders the official player"
  );
} finally {
  await server.close();
}
