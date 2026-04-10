package modes

// Dimension defines one axis of value evaluation.
//
// The value system evaluates user input across multiple independent dimensions.
// Each dimension has a weight that controls its influence on the aggregate judgment.
// Dimensions are evaluated simultaneously in a single LLM call for efficiency.
type Dimension struct {
	Name          string  `json:"name"`           // Chinese display name
	NameEN        string  `json:"name_en"`        // English display name
	Description   string  `json:"description"`    // Chinese description for LLM prompt
	DescriptionEN string  `json:"description_en"` // English description for LLM prompt
	Weight        float64 `json:"weight"`         // 0.0–1.0, influence on aggregate
}

// DefaultDimensions returns the standard 5-dimension evaluation framework.
//
// Design rationale:
//   - Logic and Safety have the highest weights because flawed reasoning
//     and dangerous operations cause the most harm.
//   - Creativity and Sincerity reward genuine, thoughtful input.
//   - Practicality grounds the evaluation in real-world applicability.
//
// These dimensions are intentionally broad enough to apply to any domain
// (code, conversation, planning) while specific enough to produce
// meaningful differentiation.
func DefaultDimensions() []Dimension {
	return []Dimension{
		{
			Name:          "逻辑性",
			NameEN:        "Logic",
			Description:   "推理是否严谨，论证是否有据，是否存在逻辑谬误或绝对化表述",
			DescriptionEN: "Is the reasoning rigorous? Is there evidence? Are there logical fallacies or absolutist claims?",
			Weight:        0.9,
		},
		{
			Name:          "创造性",
			NameEN:        "Creativity",
			Description:   "是否有新颖的想法、独特的视角、或创造性的解决方案",
			DescriptionEN: "Does it show novel ideas, unique perspectives, or creative solutions?",
			Weight:        0.7,
		},
		{
			Name:          "真诚性",
			NameEN:        "Sincerity",
			Description:   "是否真诚认真，还是敷衍了事、随意应付",
			DescriptionEN: "Is it sincere and thoughtful, or dismissive and careless?",
			Weight:        0.8,
		},
		{
			Name:          "安全性",
			NameEN:        "Safety",
			Description:   "是否存在安全风险、危险操作、或可能造成不可逆损害的行为",
			DescriptionEN: "Are there security risks, dangerous operations, or potentially irreversible damage?",
			Weight:        1.0,
		},
		{
			Name:          "实用性",
			NameEN:        "Practicality",
			Description:   "方案是否可行、是否考虑了实际约束、是否能真正解决问题",
			DescriptionEN: "Is the approach feasible? Does it consider real constraints? Will it actually solve the problem?",
			Weight:        0.75,
		},
	}
}
