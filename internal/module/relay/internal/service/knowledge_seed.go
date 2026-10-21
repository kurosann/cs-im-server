package service

func seedKnowledge(tenantID string) ([]*KnowledgeGroup, []*KnowledgeBase, []*KnowledgeDoc) {
	groups := []*KnowledgeGroup{
		{GroupKey: "g1", TenantID: tenantID, Name: "电商产品线"},
		{GroupKey: "g2", TenantID: tenantID, Name: "会员产品线"},
		{GroupKey: "g3", TenantID: tenantID, Name: "海外产品线"},
	}
	bases := []*KnowledgeBase{
		{BaseKey: "kb1", TenantID: tenantID, GroupKey: "g1", Name: "商品与库存库", Product: "电商主站", Description: "商品参数、活动规则、库存和发货。"},
		{BaseKey: "kb2", TenantID: tenantID, GroupKey: "g1", Name: "售后政策库", Product: "电商主站", Description: "退款、退货、换货和赔付边界。"},
		{BaseKey: "kb3", TenantID: tenantID, GroupKey: "g2", Name: "会员权益库", Product: "星辰会员", Description: "等级规则、折扣、券包和续费。"},
		{BaseKey: "kb4", TenantID: tenantID, GroupKey: "g3", Name: "海外售后库", Product: "Global Store", Description: "English / Español refund and logistics policies."},
	}
	docs := []*KnowledgeDoc{
		doc(tenantID, "d1", "kb1", "春节发货时间", "FAQ", "中文", "已发布", "春节期间只发货预付订单，2 月 8 日恢复全部发货。", 186, []string{"Web Chat", "Messenger"}, "2026-01-20", "2026-02-01", "手动问答对"),
		doc(tenantID, "d2", "kb1", "商品优惠叠加规则", "规则", "中文", "已发布", "限时折扣与优惠券可叠加，会员折不可叠加。", 92, []string{"Web Chat"}, "2026-01-01", "2026-01-18", "文档上传"),
		doc(tenantID, "d3", "kb2", "退款多久到账？", "FAQ", "中文", "已发布", "原路退回，1-2 个工作日到账；大额退款 3-5 个工作日。", 268, []string{"WhatsApp", "Telegram", "Web Chat"}, "2026-01-01", "2026-03-02", "历史会话沉淀"),
		doc(tenantID, "d4", "kb2", "退货流程", "模板", "中文", "已发布", "请提供订单号和商品照片，审核通过后生成退货码。", 141, []string{"WhatsApp", "Web Chat"}, "2026-01-01", "2026-02-14", "常用语导入"),
		doc(tenantID, "d5", "kb3", "会员权益表", "规则", "中文", "待审核", "普通会员 95 折，黑卡会员 88 折。", 34, []string{"Web Chat", "Messenger"}, "2026-04-01", "2026-03-20", "手动问答对"),
		doc(tenantID, "d6", "kb3", "会员券包使用", "FAQ", "中文", "草稿", "券包每月 1 日刷新，过期券不可补发。", 0, []string{"Web Chat"}, "2026-04-01", "2026-03-21", "文档上传"),
		doc(tenantID, "d7", "kb4", "Refund policy", "政策", "English", "已发布", "Refunds return to the original payment method within 1-2 business days.", 121, []string{"WhatsApp", "Telegram"}, "2026-01-01", "2026-03-12", "业务系统同步"),
		doc(tenantID, "d8", "kb4", "Política de reembolso", "政策", "Español", "已发布", "El reembolso llega en 1-2 días hábiles.", 78, []string{"WhatsApp"}, "2026-01-01", "2026-03-12", "业务系统同步"),
	}
	return groups, bases, docs
}

func doc(tenantID, id, baseID, title, docType, language, status, answer string, hits int, channels []string, effectiveAt, updatedOn, source string) *KnowledgeDoc {
	return &KnowledgeDoc{
		DocKey: id, TenantID: tenantID, BaseKey: baseID, Title: title, DocType: docType,
		Language: language, Status: status, Answer: answer, Hits: hits, Channels: channels,
		EffectiveAt: effectiveAt, UpdatedOn: updatedOn, Source: source,
	}
}
