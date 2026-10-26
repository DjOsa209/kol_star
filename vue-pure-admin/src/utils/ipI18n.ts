import { useI18n } from "vue-i18n";
import { i18n } from "@/plugins/i18n";
import english from "@/locales/ip.en.json";
import legacyEnglish from "@/locales/ip.en.legacy.json";

const catalog: Record<string, string> = english;
// Accept earlier English templates without keeping their wording in the interface.
const legacyTerms = new Map(
  Object.entries(legacyEnglish).map(([label, source]) => [
    label.toLowerCase(),
    source
  ])
);
const regexSpecial = new Set([
  ".",
  "*",
  "+",
  "?",
  "^",
  "$",
  "{",
  "}",
  "(",
  ")",
  "|",
  "[",
  "]",
  String.fromCharCode(92)
]);
const escapeRegex = (value: string) =>
  [...value]
    .map(char =>
      regexSpecial.has(char) ? String.fromCharCode(92) + char : char
    )
    .join("");
const catalogPatterns = Object.entries(catalog)
  .flatMap(([source, target]) => {
    if (!/%[+\-#.0-9]*[sdvf]/.test(source)) return [[source, target]];
    let index = 0;
    const template = source.replace(
      /%[+\-#.0-9]*[sdvf]/g,
      () => `{${index++}}`
    );
    index = 0;
    const translated = target.replace(
      /%[+\-#.0-9]*[sdvf]/g,
      () => `{${index++}}`
    );
    return [
      [source, target],
      [template, translated]
    ];
  })
  .filter(([source]) => /\{\d+\}/.test(source))
  .map(([source, target]) => ({
    source,
    target,
    pattern: new RegExp(
      "^" +
        source
          .split(/(\{\d+\})/)
          .map(part => (/^\{\d+\}$/.test(part) ? "(.*?)" : escapeRegex(part)))
          .join("") +
        "$",
      "s"
    ),
    reverse: new RegExp(
      "^" +
        target
          .split(/(\{\d+\})/)
          .map(part => (/^\{\d+\}$/.test(part) ? "(.*?)" : escapeRegex(part)))
          .join("") +
        "$",
      "si"
    )
  }));

export function currentIPLanguage(): "en" | "zh" {
  const locale = i18n.global.locale;
  return String(typeof locale === "string" ? locale : locale.value).startsWith(
    "en"
  )
    ? "en"
    : "zh";
}

/** Only application copy and canonical option labels enter this function. */
export function ipText(
  value: unknown,
  values: unknown[] = [],
  language = currentIPLanguage()
): string {
  const source = value == null ? "" : String(value);
  let text = language === "en" ? (catalog[source] ?? source) : source;
  if (language === "en" && text === source) {
    const entry = catalogPatterns.find(entry => entry.pattern.test(source));
    if (entry) {
      const captures = source.match(entry.pattern);
      text = entry.target.replace(
        /\{(\d+)\}/g,
        (token, index) => captures?.[Number(index) + 1] ?? token
      );
    }
  }
  if (language === "en" && text === source && /[、；]| \/ /.test(source)) {
    text = source
      .split(/(、|；| \/ )/)
      .map(part => catalog[part] ?? part)
      .join("");
  }
  return text.replace(/\{(\d+)\}/g, (token, index) =>
    index in values ? String(values[index]) : token
  );
}

export function useIPLocale() {
  const { locale } = useI18n({ useScope: "global" });
  return {
    ipText: (value: unknown, values: unknown[] = []) =>
      ipText(
        value,
        values,
        String(locale.value).startsWith("en") ? "en" : "zh"
      ),
    locale
  };
}

const canonicalOptions = new Set([
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
  "其他",
  "体育IP",
  "动画/影视",
  "音乐/活动",
  "电竞",
  "可合作",
  "洽谈中",
  "待评估",
  "暂不可合作",
  "中国",
  "东南亚",
  "南亚",
  "欧洲",
  "中东",
  "非洲",
  "拉美",
  "北美",
  "全球",
  "产品定制",
  "功能联动",
  "联名周边礼盒",
  "线上营销",
  "线下物料及互动"
]);
export function canonicalIPOption(value: string): string {
  const normalized = value.trim();
  const legacy = legacyTerms.get(normalized.toLowerCase());
  if (legacy && canonicalOptions.has(legacy)) return legacy;
  return (
    [...canonicalOptions].find(
      source => catalog[source]?.toLowerCase() === normalized.toLowerCase()
    ) ?? normalized
  );
}

/** English template headers work even when the interface has switched back to Chinese. */
export function canonicalIPImportHeader(value: string): string {
  const preferred = [
    "导入编号",
    "IP名称*",
    "IP名称",
    "IP类型*",
    "IP类型",
    "覆盖市场*",
    "覆盖市场",
    ...Array.from({ length: 9 }, (_, i) => i + 1).flatMap(i => [
      `覆盖市场${i}${i === 1 ? "*" : ""}`,
      `覆盖市场${i}`,
      `渗透率${i}（%）`
    ]),
    "版权方",
    "联系人",
    "目标受众",
    "IP简介",
    "合作状态",
    "可合作形式",
    "素材链接",
    "币种",
    "基础权益价格下限",
    "基础权益价格上限",
    "授权范围与限制",
    "IP形象与风格",
    "IP生命周期",
    "受众性别占比",
    "主要年龄",
    "消费力评价",
    "注册用户 / 赛事参与",
    "历史销售转化力",
    "IP展示与合作表现",
    "合作建议与提醒",
    "历史合作名称",
    "历史合作概述"
  ];
  const supported = preferred.find(
    source => catalog[source]?.toLowerCase() === value.trim().toLowerCase()
  );
  if (supported) return supported;
  const legacy = legacyTerms.get(value.trim().toLowerCase());
  if (legacy) return legacy;

  const direct = Object.entries(catalog).find(
    ([, english]) => english.toLowerCase() === value.trim().toLowerCase()
  );
  if (direct) return direct[0];
  const entry = catalogPatterns.find(entry => entry.reverse.test(value));
  if (entry) {
    const captures = value.match(entry.reverse);
    return entry.source.replace(
      /\{(\d+)\}/g,
      (token, index) => captures?.[Number(index) + 1] ?? token
    );
  }
  return value;
}
