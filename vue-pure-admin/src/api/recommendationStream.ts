import type {
  IPRecommendationPresentation,
  IPRecommendationConversationResult
} from "./recommendations";

export interface RecommendationStreamOptions {
  signal?: AbortSignal;
  onStatus?: (message: string) => void;
  onSession?: (id: number) => void;
  onPresentation?: (presentation: IPRecommendationPresentation) => void;
}

export async function readRecommendationStream(
  stream: ReadableStream<Uint8Array>,
  options: RecommendationStreamOptions = {}
): Promise<IPRecommendationConversationResult> {
  const reader = stream.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let buffer = "";
  let event = "message";
  let data: string[] = [];
  let dataLength = 0;
  let result: IPRecommendationConversationResult | undefined;
  const abort = () => void reader.cancel();
  options.signal?.addEventListener("abort", abort, { once: true });
  const checkAbort = () => {
    if (options.signal?.aborted)
      throw new DOMException("已停止生成", "AbortError");
  };
  const dispatch = () => {
    if (!data.length) {
      event = "message";
      return;
    }
    const body = JSON.parse(data.join("\n"));
    if (event === "error")
      throw new Error(body.message || "推荐生成失败，请重试");
    if (event === "status" && typeof body.message === "string")
      options.onStatus?.(body.message);
    if (event === "session" && Number.isSafeInteger(body.id) && body.id > 0)
      options.onSession?.(body.id);
    if (event === "presentation") {
      if (
        typeof body.headline !== "string" ||
        typeof body.explanation !== "string" ||
        !Array.isArray(body.basis) ||
        !["ai", "rule"].includes(body.source)
      )
        throw new Error("推荐内容格式不合法，请重试");
      options.onPresentation?.(body);
    }
    if (event === "done") {
      if (
        body.code !== 0 ||
        !body.data?.subscription?.id ||
        !Array.isArray(body.data.results) ||
        typeof body.data.presentation?.headline !== "string"
      )
        throw new Error(body.message || "推荐结果不完整，请重试");
      result = body;
    }
    event = "message";
    data = [];
    dataLength = 0;
  };
  const line = (value: string) => {
    if (!value) dispatch();
    else if (value.startsWith("event:")) event = value.slice(6).trim();
    else if (value.startsWith("data:")) {
      dataLength += value.length;
      if (dataLength > 1 << 20) throw new Error("推荐响应过长，请重试");
      data.push(value.slice(5).replace(/^ /, ""));
    }
  };
  try {
    checkAbort();
    while (!result) {
      const chunk = await reader.read();
      checkAbort();
      buffer += decoder.decode(chunk.value, { stream: !chunk.done });
      if (buffer.length > 1 << 20) throw new Error("推荐响应过长，请重试");
      let end: number;
      while ((end = buffer.indexOf("\n")) >= 0) {
        line(buffer.slice(0, end).replace(/\r$/, ""));
        buffer = buffer.slice(end + 1);
        if (result) break;
      }
      if (chunk.done) {
        if (buffer) line(buffer.replace(/\r$/, ""));
        dispatch();
        break;
      }
    }
    if (!result) throw new Error("流式连接提前结束，回答尚未完成，请重试");
    return result;
  } finally {
    options.signal?.removeEventListener("abort", abort);
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}
