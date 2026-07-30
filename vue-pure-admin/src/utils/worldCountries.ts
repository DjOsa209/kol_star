export type WorldCountryOption = {
  code: string;
  name: string;
  englishName: string;
  label: string;
};

const regionCodes = `
AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ
CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR
GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO
JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR
MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO
RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV
TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW
`
  .trim()
  .split(/\s+/);

function regionDisplayNames(locale: string) {
  try {
    return new Intl.DisplayNames([locale], { type: "region" });
  } catch {
    return null;
  }
}

const chineseNames = regionDisplayNames("zh-CN");
const englishNames = regionDisplayNames("en");

export const worldCountryOptions: WorldCountryOption[] = regionCodes
  .map(code => {
    const name = chineseNames?.of(code) || code;
    const englishName = englishNames?.of(code) || code;
    return {
      code,
      name,
      englishName,
      label:
        name === englishName
          ? `${name} (${code})`
          : `${name} / ${englishName} (${code})`
    };
  })
  .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"));

export function parseProjectTargetMarkets(value: unknown): string[] {
  const values = Array.isArray(value)
    ? value
    : String(value || "").split(/[,，;；、|]/);
  return Array.from(
    new Set(values.map(item => String(item || "").trim()).filter(Boolean))
  );
}

export function serializeProjectTargetMarkets(values: unknown): string {
  return parseProjectTargetMarkets(values).join(",");
}

export function countryOptionsWithLegacyValues(values: unknown) {
  const options = [...worldCountryOptions];
  const known = new Set(options.map(item => item.name));
  for (const name of parseProjectTargetMarkets(values)) {
    if (known.has(name)) continue;
    options.unshift({
      code: "",
      name,
      englishName: name,
      label: `${name}（历史数据）`
    });
    known.add(name);
  }
  return options;
}
