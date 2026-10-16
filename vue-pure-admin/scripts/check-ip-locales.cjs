const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const assert = require("node:assert/strict");
const ts = require("typescript");
const XLSX = require("xlsx");
const { parse } = require("@vue/compiler-sfc");
const { baseParse } = require("@vue/compiler-dom");
const root = path.resolve(__dirname, "..");
const catalog = JSON.parse(
  fs.readFileSync(path.join(root, "src/locales/ip.en.json"), "utf8")
);
const backendCatalog = JSON.parse(
  fs.readFileSync(
    path.join(root, "../backend/cmd/server/data/ip_en.json"),
    "utf8"
  )
);
const legacyCatalog = JSON.parse(
  fs.readFileSync(path.join(root, "src/locales/ip.en.legacy.json"), "utf8")
);
assert.deepEqual(
  catalog,
  backendCatalog,
  "Frontend and server terminology must match"
);
const source = fs.readFileSync(path.join(root, "src/utils/ipI18n.ts"), "utf8");
const output = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.CommonJS,
    esModuleInterop: true
  }
}).outputText;
const selected = { value: "en" };
const moduleValue = { exports: {} };
vm.runInNewContext(output, {
  exports: moduleValue.exports,
  require(name) {
    if (name === "vue-i18n") return { useI18n: () => ({ locale: selected }) };
    if (name === "@/plugins/i18n")
      return { i18n: { global: { locale: selected } } };
    if (name === "@/locales/ip.en.json") return catalog;
    if (name === "@/locales/ip.en.legacy.json") return legacyCatalog;
    throw new Error(name);
  }
});
const { ipText, useIPLocale, canonicalIPOption, canonicalIPImportHeader } =
  moduleValue.exports;
assert.equal(ipText("游戏"), "Gaming");
assert.equal(canonicalIPOption("Gaming"), "游戏");
assert.equal(ipText("已提交 {0} / 5 个职能", [3]), "3 / 5 teams submitted");
assert.equal(ipText("测试项目 · GT Opus"), "测试项目 · GT Opus");
assert.equal(ipText("{0} 前", ["2026-10-21 23:59"]), "By 2026-10-21 23:59");
const hook = useIPLocale();
assert.equal(hook.ipText("游戏"), "Gaming");
selected.value = "zh";
assert.equal(hook.ipText("游戏"), "游戏");
selected.value = "en";
// Exercise the component formatter and its translation wrapper, including percentages.
const resourceScript = parse(
  fs.readFileSync(
    path.join(root, "src/views/business/ip/resources/index.vue"),
    "utf8"
  )
).descriptor.scriptSetup.content;
const resourceAST = ts.createSourceFile(
  "resources.ts",
  resourceScript,
  ts.ScriptTarget.Latest,
  true
);
const marketFormatter = [
  "arrayValue",
  "penetrationValues",
  "marketCoverageText"
]
  .map(name =>
    resourceAST.statements
      .find(node => ts.isFunctionDeclaration(node) && node.name?.text === name)
      .getText(resourceAST)
  )
  .join("\n");
const marketRuntime = { ipText, JSON, Object, Number };
vm.createContext(marketRuntime);
vm.runInContext(
  ts.transpileModule(marketFormatter, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 }
  }).outputText,
  marketRuntime
);
const marketFixture = {
  markets: ["全球", "中国", "东南亚", "南亚"],
  marketPenetrations: { 全球: 80, 中国: 80, 东南亚: 80, 南亚: 80 }
};
const originalMarkets = JSON.stringify(marketFixture);
const displayedMarkets = item => ipText(marketRuntime.marketCoverageText(item));
assert.equal(
  displayedMarkets(marketFixture),
  "Global 80% / China 80% / Southeast Asia 80% / South Asia 80%",
  "Market labels with percentages must translate in English"
);
assert.equal(
  displayedMarkets({
    markets: '["中国","东南亚","自定义市场"]',
    marketPenetrations: '{"中国":0,"东南亚":12.5}'
  }),
  "China 0% / Southeast Asia 12.5% / 自定义市场"
);
selected.value = "zh";
assert.equal(
  displayedMarkets(marketFixture),
  "全球 80% / 中国 80% / 东南亚 80% / 南亚 80%"
);
selected.value = "en";
assert.equal(
  displayedMarkets(marketFixture),
  "Global 80% / China 80% / Southeast Asia 80% / South Asia 80%"
);
assert.equal(
  JSON.stringify(marketFixture),
  originalMarkets,
  "Display formatting must preserve canonical market names and penetration keys"
);
const book = XLSX.readFile(
  path.join(root, "public/IP-import-template.en.xlsx")
);
const headers = XLSX.utils.sheet_to_json(book.Sheets[book.SheetNames[0]], {
  header: 1
})[0];
for (const header of headers)
  assert.match(
    canonicalIPImportHeader(header),
    /[\u3400-\u9fff]/,
    `Unsupported English template header: ${header}`
  );
assert.equal(canonicalIPImportHeader("IP name*"), "IP名称*");
assert.equal(canonicalIPImportHeader("Covered market 1*"), "覆盖市场1*");
assert.equal(
  canonicalIPImportHeader("Market penetration 3 (%)"),
  "渗透率3（%）"
);
assert.equal(canonicalIPOption(ipText("可合作")), "可合作");
for (const label of ["Offline Events", "In-person events"])
  assert.equal(canonicalIPOption(label), "线下活动");
for (const [label, expected] of Object.entries(legacyCatalog)) {
  if (expected === "线下活动") continue;
  assert.equal(canonicalIPImportHeader(label), expected);
}
for (const source of [
  "基础权益价格下限",
  "基础权益价格上限",
  "IP形象与风格",
  "主要年龄",
  "消费力评价",
  "历史销售转化力",
  "IP展示与合作表现",
  "历史合作名称"
])
  assert.equal(canonicalIPImportHeader(ipText(source)), source);
for (const [key, value] of Object.entries(catalog)) {
  assert.ok(value.trim(), `Empty translation: ${key}`);
  assert.ok(!/[\u3400-\u9fff]/.test(value), `Untranslated copy: ${key}`);
  assert.deepEqual(
    (key.match(/\{\d+\}|%[+\-#.0-9]*[sdfv]/g) || []).sort(),
    (value.match(/\{\d+\}|%[+\-#.0-9]*[sdfv]/g) || []).sort(),
    `Placeholder mismatch: ${key}`
  );
}
const files = [
  "resources/index.vue",
  "requests/index.vue",
  "requests/components/DeadlineStatus.vue",
  "requests/components/RequestText.vue",
  "recommendations/index.vue",
  "recommendations/ExternalResearchWorkbench.vue"
];
for (const file of files) {
  const source = fs.readFileSync(
    path.join(root, "src/views/business/ip", file),
    "utf8"
  );
  const descriptor = parse(source).descriptor;
  const ast = baseParse(descriptor.template.content);
  function visit(node) {
    if (node.type === 2)
      assert.ok(
        !/[\u3400-\u9fff]/.test(node.content),
        `Unlocalized template text: ${file}: ${node.content}`
      );
    for (const p of node.props || [])
      if (
        p.type === 6 &&
        p.value &&
        [
          "label",
          "placeholder",
          "aria-label",
          "title",
          "empty-text",
          "description"
        ].includes(p.name)
      )
        assert.ok(
          !/[\u3400-\u9fff]/.test(p.value.content),
          `Unlocalized attribute: ${file}`
        );
    for (const child of node.children || []) visit(child);
  }
  visit(ast);
}
console.log(
  `PASS: ${Object.keys(catalog).length} English strings, runtime language switching, canonical option values, all English workbook headers and IP template coverage`
);

for (const file of files) {
  const descriptor = parse(
    fs.readFileSync(path.join(root, "src/views/business/ip", file), "utf8")
  ).descriptor;
  function checkExpressions(source) {
    const sf = ts.createSourceFile(
      "copy.ts",
      source,
      ts.ScriptTarget.Latest,
      true
    );
    function walk(node) {
      if (
        ts.isCallExpression(node) &&
        node.expression.getText(sf) === "ipText" &&
        node.arguments[0] &&
        ts.isStringLiteral(node.arguments[0])
      ) {
        const key = node.arguments[0].text;
        if (/[\u3400-\u9fff]/.test(key))
          assert.ok(catalog[key], `Missing catalog entry in ${file}: ${key}`);
      }
      ts.forEachChild(node, walk);
    }
    walk(sf);
  }
  checkExpressions(descriptor.scriptSetup.content);
  function walk(node) {
    if (node.type === 5) checkExpressions(node.content.content);
    for (const prop of node.props || [])
      if (prop.exp) checkExpressions(prop.exp.content);
    for (const child of node.children || []) walk(child);
  }
  walk(baseParse(descriptor.template.content));
}
const recommendationSource = parse(
  fs.readFileSync(
    path.join(root, "src/views/business/ip/recommendations/index.vue"),
    "utf8"
  )
).descriptor.scriptSetup.content;
const recommendationAST = ts.createSourceFile(
  "recommendations.ts",
  recommendationSource,
  ts.ScriptTarget.Latest,
  true
);
const sendFunction = recommendationAST.statements
  .find(
    node => ts.isFunctionDeclaration(node) && node.name?.text === "sendMessage"
  )
  .getText(recommendationAST);
const calls = [];
const composer = { value: "Why is this recommended?" };
const compare = { value: [] };
const routing = {
  canSendMessage: { value: true },
  composerText: composer,
  webSearchMode: { value: false },
  selectedSubscription: { value: { ipWorkspace: { stage: "research" } } },
  attachment: { value: null },
  visibleResults: { value: [{ resourceId: 1 }] },
  comparison: compare,
  converse: (action, text) => calls.push({ action, text })
};
vm.createContext(routing);
vm.runInContext(sendFunction, routing);
routing.sendMessage();
assert.equal(calls.at(-1).action, "discuss");
composer.value = "Compare these selected candidates";
compare.value = [1, 2];
routing.sendMessage();
assert.equal(calls.at(-1).action, "compare");
composer.value = "为什么优先它？";
routing.sendMessage();
assert.equal(calls.at(-1).action, "discuss");
console.log(
  "PASS: translated-call coverage and English/Chinese follow-up routing"
);

const normalizationCode = ["parseTextArray", "normalizeIPTypes"]
  .map(name =>
    recommendationAST.statements
      .find(node => ts.isFunctionDeclaration(node) && node.name?.text === name)
      .getText(recommendationAST)
  )
  .join("\n");
const normalization = {
  canonicalIPOption,
  ipTypes: [
    "游戏",
    "动画",
    "影视",
    "综艺",
    "体育",
    "潮牌",
    "潮玩",
    "品牌",
    "文化艺术",
    "线下活动",
    "其他"
  ]
};
vm.createContext(normalization);
vm.runInContext(
  ts.transpileModule(normalizationCode, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 }
  }).outputText,
  normalization
);
assert.deepEqual([...normalization.normalizeIPTypes(["Esports"])], ["游戏"]);
assert.deepEqual([...normalization.normalizeIPTypes(["Cartoon"])], ["动画"]);
assert.ok(
  !/[\u3400-\u9fff]/.test(
    ipText("2027 年法定节假日及调休日历尚未配置，截止时间待核定")
  )
);
console.log("PASS: English IP type disambiguation and uncached-year warnings");
