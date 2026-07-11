package catalog

import (
	"github.com/local/easysearch/backend/internal/model"
)

// BuiltinDefinitions returns the small set of demo definitions shipped
// with the binary. The Phase 4 demo lets users add these as-is and see
// the system work end to end before Phase 5's full YAML engine lands.
//
// All built-ins use protocol "mock" so they don't reach the network.
func BuiltinDefinitions() []model.IndexerDefinition {
	now := "1.0.0"
	return []model.IndexerDefinition{
		{
			ID:          "demo-alpha",
			Name:        "示例 A (内置)",
			Description: "内置 demo 索引器，结果稳定可预测。",
			Version:     now,
			Language:    "zh-CN",
			Type:        "public",
			Protocol:    "mock",
		},
		{
			ID:          "demo-beta",
			Name:        "示例 B (内置)",
			Description: "第二个内置 demo，用于演示多源去重。",
			Version:     now,
			Language:    "zh-CN",
			Type:        "public",
			Protocol:    "mock",
		},
		{
			ID:          "demo-gamma",
			Name:        "示例 C (内置，故意失败)",
			Description: "永远失败的 demo，用于演示错误隔离。",
			Version:     now,
			Language:    "zh-CN",
			Type:        "public",
			Protocol:    "mock",
		},
	}
}
