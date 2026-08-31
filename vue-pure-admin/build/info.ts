import type { Plugin } from "vite";
import gradient from "gradient-string";
import { getPackageSize } from "./utils";
import dayjs, { type Dayjs } from "dayjs";
import duration from "dayjs/plugin/duration";
import boxen, { type Options as BoxenOptions } from "boxen";
import { resolve } from "node:path";
dayjs.extend(duration);

const welcomeMessage = gradient(["cyan", "magenta"]).multiline(
  `您好! 欢迎使用 pure-admin 开源项目\n我们为您精心准备了下面两个贴心的保姆级文档\nhttps://pure-admin.cn\nhttps://pure-admin-utils.netlify.app`
);

const boxenOptions: BoxenOptions = {
  padding: 0.5,
  borderColor: "cyan",
  borderStyle: "round"
};

export function viteBuildInfo(): Plugin {
  let config: { command: string };
  let startTime: Dayjs;
  let endTime: Dayjs;
  let outDir: string;
  return {
    name: "vite:buildInfo",
    configResolved(resolvedConfig) {
      config = resolvedConfig;
      outDir = resolve(
        resolvedConfig.root,
        resolvedConfig.build?.outDir ?? "dist"
      );
    },
    buildStart() {
      console.log(boxen(welcomeMessage, boxenOptions));
      if (config.command === "build") {
        startTime = dayjs(new Date());
      }
    },
    async writeBundle() {
      if (config.command === "build") {
        endTime = dayjs(new Date());
        let size: string | number;
        try {
          size = await getPackageSize({ folder: outDir });
        } catch (error) {
          if ((error as NodeJS.ErrnoException)?.code === "ENOENT") {
            console.warn(`跳过构建产物统计：未找到输出目录 ${outDir}`);
            return;
          }
          throw error;
        }
        console.log(
          boxen(
            gradient(["cyan", "magenta"]).multiline(
              `🎉 恭喜打包完成（总用时${dayjs
                .duration(endTime.diff(startTime))
                .format("mm分ss秒")}，打包后的大小为${size}）`
            ),
            boxenOptions
          )
        );
      }
    }
  };
}
