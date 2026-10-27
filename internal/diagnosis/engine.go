package diagnosis

import "fmt"

// Evaluation is the complete output of one rule. Limitations explain why a
// rule could not make a supported finding and are consumed by the
// insufficient-evidence rule.
type Evaluation struct {
	Finding     *Finding
	Limitations []string
}

// Rule is deliberately pure: all evidence and policy arrive as values.
type Rule interface {
	Code() Code
	Evaluate(Suite, Thresholds) Evaluation
}

type summaryRule interface {
	Rule
	EvaluateSummary(Suite, Thresholds, []Evaluation) Evaluation
}

// Registry stores rules in their public evaluation order.
type Registry struct {
	rules []Rule
}

// NewRegistry validates stable rule identity while preserving input order.
func NewRegistry(rules ...Rule) (Registry, error) {
	seen := make(map[Code]struct{}, len(rules))
	registered := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			return Registry{}, fmt.Errorf("register diagnostic rule: rule must not be nil")
		}
		code := rule.Code()
		if code == "" {
			return Registry{}, fmt.Errorf("register diagnostic rule: code must not be empty")
		}
		if _, exists := seen[code]; exists {
			return Registry{}, fmt.Errorf("register diagnostic rule: duplicate code %q", code)
		}
		seen[code] = struct{}{}
		registered = append(registered, rule)
	}
	return Registry{rules: registered}, nil
}

// Rules returns a copy so callers cannot mutate evaluation order.
func (registry Registry) Rules() []Rule {
	return append([]Rule(nil), registry.rules...)
}

// Engine evaluates rules without performing I/O or reading ambient state.
type Engine struct {
	thresholds Thresholds
	registry   Registry
}

func NewEngine(thresholds Thresholds, registry Registry) (Engine, error) {
	if thresholds.RuleVersion == "" {
		return Engine{}, fmt.Errorf("create diagnostic engine: rule version must not be empty")
	}
	return Engine{thresholds: thresholds, registry: registry}, nil
}

// Evaluate returns findings in registry order. A rule cannot change its stable
// identity by returning a different finding code.
func (engine Engine) Evaluate(suite Suite) (Report, error) {
	report := Report{RuleVersion: engine.thresholds.RuleVersion, Findings: []Finding{}}
	evaluations := make([]Evaluation, 0, len(engine.registry.rules))
	for _, rule := range engine.registry.rules {
		var evaluation Evaluation
		if summary, ok := rule.(summaryRule); ok {
			evaluation = summary.EvaluateSummary(suite, engine.thresholds, append([]Evaluation(nil), evaluations...))
		} else {
			evaluation = rule.Evaluate(suite, engine.thresholds)
		}
		evaluations = append(evaluations, evaluation)
		if evaluation.Finding == nil {
			continue
		}
		finding := *evaluation.Finding
		if finding.Code != rule.Code() {
			return Report{}, fmt.Errorf("evaluate diagnostic rule %q: returned code %q", rule.Code(), finding.Code)
		}
		if finding.RuleVersion == "" {
			finding.RuleVersion = engine.thresholds.RuleVersion
		}
		report.Findings = append(report.Findings, finding)
	}
	return report, nil
}

// DefaultRegistry returns all v1 rules in their stable public order.
func DefaultRegistry() Registry {
	registry, err := NewRegistry(
		FirstByteRule{},
		ConnectionRule{},
		TailRule{},
		ConcurrencyRule{},
		ErrorRule{},
		InsufficientEvidenceRule{},
	)
	if err != nil {
		panic(err) // The compile-time default set has fixed nonempty unique codes.
	}
	return registry
}

// DefaultEngine returns the complete deterministic v1 diagnostic engine.
func DefaultEngine() Engine {
	engine, err := NewEngine(DefaultThresholds(), DefaultRegistry())
	if err != nil {
		panic(err) // The built-in threshold profile always has a rule version.
	}
	return engine
}
