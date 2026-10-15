package main

import (
	"net/url"
	"slices"
	"strings"
)

// User-selected research sources. Priority is an editorial preference, never a
// guarantee of relevance, IP licensing availability or access to paid datasets.
type ipResearchSource struct {
	Name     string
	Domains  []string
	Priority string
	Category string
	Use      string
	URL      string
}

var ipResearchSources = []ipResearchSource{
	{"Licensing Expo", []string{"licensingexpo.com", "brandlicensingexpo.com"}, "P0", "授权目录", "用于发现品牌、授权方和合作主体；参展或目录收录不代表已确认可授权", "https://www.brandlicensingexpo.com/exhibitor-list/brand-directory/"},
	{"License Global", []string{"licenseglobal.com"}, "P0", "授权行业榜单", "用于核对授权企业与行业榜单；保留报告年份，排名不代表当前授权或报价", "https://www.licenseglobal.com/licensing-resources/rankings-lists"},
	{"Licensing International", []string{"licensinginternational.org"}, "P0", "行业研究", "用于行业分类、规模与趋势背景；行业数据不能作为具体 IP 授权证明，未获取的会员报告不作引用", "https://licensinginternational.org/get-survey/"},
	{"Brand Licensing Europe", []string{"brandlicensing.eu"}, "P1", "授权目录", "用于欧洲品牌、授权机构和合作主体发现；具体地域、品类与合作档期待核实", "https://www.brandlicensing.eu/"},
	{"WIPO Global Brand Database", []string{"wipo.int"}, "P1", "商标核查线索", "用于商标主体、注册地域及权利核查线索；不能据此推断已获 IP 合作授权", "https://www.wipo.int/zh/web/global-brand-database"},
	{"Kantar BrandZ", []string{"kantar.com"}, "P2", "品牌价值", "用于品牌影响力与估值背景；保留榜单年份和市场口径，不等同于授权价格", "https://www.kantar.com/Campaigns/BrandZ/Global"},
	{"YouGov BrandIndex", []string{"yougov.com"}, "P2", "品牌认知", "用于消费者认知、推荐度及市场差异；仅使用实际可见的年份、地区与指标", "https://yougov.com/reports/rankings"},
	{"SportsPro", []string{"sportspro.com"}, "P2", "体育商业价值", "用于运动员商业价值与跨市场影响力背景；不能推断代言或形象授权已开放", "https://50mm.sportspro.com/"},
	{"Parrot Analytics", []string{"parrotanalytics.com"}, "P2", "影视需求", "用于公开影视需求和地区影响力；未取得的付费或实时数据不作引用", "https://www.parrotanalytics.com/"},
	{"Billboard", []string{"billboard.com"}, "P2", "音乐榜单", "用于公开音乐榜单和艺人趋势，保留榜期与地区；不代表商业授权可用", "https://www.billboard.com/charts/"},
	{"Luminate", []string{"luminatedata.com"}, "P2", "音乐消费", "用于公开音乐消费研究与趋势；未取得的数据产品不作引用", "https://luminatedata.com/"},
	{"Esports Charts", []string{"escharts.com"}, "P2", "电竞观众", "用于公开赛事、战队和观众指标；保留统计时间与范围，不推断游戏 IP 授权状态", "https://escharts.com/"},
}

func preferredIPResearchDomains() []string {
	domains := []string{}
	for _, source := range ipResearchSources {
		domains = append(domains, source.Domains...)
	}
	return domains
}

func labelIPResearchEvidence(item recommendationEvidence) recommendationEvidence {
	parsed, err := url.Parse(item.URL)
	if err != nil {
		return item
	}
	host := strings.ToLower(parsed.Hostname())
	if (host == "wipo.int" || strings.HasSuffix(host, ".wipo.int")) && host != "branddb.wipo.int" && !strings.Contains(parsed.Path, "/global-brand-database") {
		item.SourceName = "WIPO"
		item.SourcePriority = ""
		item.SourceCategory = "公开资料"
		item.EvidenceUse = "WIPO 网站公开页面；具体内容用途需逐页核实，不能视为商标数据库查询结果或授权证明"
		return item
	}
	for _, source := range ipResearchSources {
		for _, domain := range source.Domains {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				item.SourceName, item.SourcePriority, item.SourceCategory, item.EvidenceUse = source.Name, source.Priority, source.Category, source.Use
				return item
			}
		}
	}
	item.EvidenceUse = "其他公开网页：一般背景与合作线索需核实，不能证明授权、报价或影响力指标"
	return item
}

func prioritizeIPResearchEvidence(items []recommendationEvidence) []recommendationEvidence {
	for i := range items {
		items[i] = labelIPResearchEvidence(items[i])
	}
	rank := func(priority string) int {
		switch priority {
		case "P0":
			return 0
		case "P1":
			return 1
		case "P2":
			return 2
		default:
			return 3
		}
	}
	slices.SortStableFunc(items, func(a, b recommendationEvidence) int { return rank(a.SourcePriority) - rank(b.SourcePriority) })
	if len(items) > 6 {
		items = items[:6]
	}
	return items
}

const ipResearchSourcePrompt = `来源优先级按实际返回的 sourcePriority 使用：P0 授权目录/行业机构，P1 欧洲授权目录与商标核查线索，P2 品牌、体育、影视、音乐与电竞影响力资料，未列明来源继续作为一般公开线索。优先级仅用于组织可见资料，不能代替相关性核实。按照每条 evidenceUse 区分用途：行业规模不能说明某 IP 可授权，品牌价值不能作为授权报价，商标注册不能代表已取得 IP 合作授权，影响力榜单不能保证本次合作效果。没有可见具体名称、指标、年份、地域或授权主体时明确缺口，不用机构声誉填补数据。只引用本轮实际返回的标题与摘要；来源目录中列有网站不代表本轮访问成功，更不代表读取了会员、付费报告或数据库完整记录。保留已提供的发布时间，区分发布时间、榜单年份和检索时间，不将旧榜单描述为实时或最新。`
