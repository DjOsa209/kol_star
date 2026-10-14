import assert from "node:assert/strict";
import test from "node:test";
// @ts-expect-error -- Native Node tests resolve the TypeScript source directly.
import { readRecommendationStream } from "./recommendationStream.ts";

const presentation = {
  headline: "优先评估设计资源",
  explanation: "结合产品方向与授权资料。",
  basis: [],
  adjustments: [],
  questions: [],
  suggestedPrompts: [],
  source: "ai"
};
const completed = {
  code: 0,
  message: "ok",
  data: { subscription: { id: 7 }, results: [], presentation }
};
const frame = (event: string, data: unknown) =>
  `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
const encoded = (text: string) => new TextEncoder().encode(text);
const fragmented = (text: string) => {
  const bytes = encoded(text);
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const byte of bytes) controller.enqueue(new Uint8Array([byte]));
      controller.close();
    }
  });
};

test("delivers Chinese previews before the terminal event arrives", async () => {
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const stream = new ReadableStream<Uint8Array>({
    start(value) {
      controller = value;
    }
  });
  let previewReady!: () => void;
  const received = new Promise<void>(resolve => {
    previewReady = resolve;
  });
  let finished = false;
  const answer = readRecommendationStream(stream, {
    onPresentation(value) {
      assert.equal(value.headline, presentation.headline);
      assert.equal(finished, false);
      previewReady();
    }
  }).then(value => {
    finished = true;
    return value;
  });
  controller.enqueue(encoded(frame("presentation", presentation)));
  await received;
  assert.equal(finished, false);
  controller.enqueue(encoded(frame("done", completed)));
  assert.deepEqual(await answer, completed);
});

test("handles byte-split UTF-8, CRLF, heartbeat and multiline data", async () => {
  const statuses: string[] = [];
  const result = await readRecommendationStream(
    fragmented(
      ': keep-alive\r\n\r\nevent: status\r\ndata: {\r\ndata: "message":"正在筛选设计资源"}\r\n\r\n' +
        frame("presentation", presentation).replaceAll("\n", "\r\n") +
        frame("done", completed).trimEnd()
    ),
    { onStatus: value => statuses.push(value) }
  );
  assert.deepEqual(statuses, ["正在筛选设计资源"]);
  assert.deepEqual(result, completed);
});

test("a dropped connection cannot commit an unfinished answer", async () => {
  await assert.rejects(
    readRecommendationStream(fragmented(frame("presentation", presentation))),
    /提前结束/
  );
});

test("reports persistence failure after preview instead of success", async () => {
  let previews = 0;
  await assert.rejects(
    readRecommendationStream(
      fragmented(
        frame("presentation", presentation) +
          frame("error", { message: "回答保存失败" })
      ),
      {
        onPresentation: () => {
          previews++;
        }
      }
    ),
    /回答保存失败/
  );
  assert.equal(previews, 1);
});

test("stop cancels the response reader and rejects the pending completion", async () => {
  const abort = new AbortController();
  let canceled = false;
  const stream = new ReadableStream<Uint8Array>({
    cancel() {
      canceled = true;
    }
  });
  const answer = readRecommendationStream(stream, { signal: abort.signal });
  abort.abort();
  await assert.rejects(answer, { name: "AbortError" });
  assert.equal(canceled, true);
});

test("reports the saved session before completion so retries reuse it", async () => {
  let savedId = 0;
  await assert.rejects(
    readRecommendationStream(
      fragmented(
        frame("session", { id: 7 }) + frame("error", { message: "模型断流" })
      ),
      {
        onSession: id => {
          savedId = id;
        }
      }
    ),
    /模型断流/
  );
  assert.equal(savedId, 7);
});
