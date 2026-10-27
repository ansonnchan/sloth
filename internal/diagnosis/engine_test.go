package diagnosis

import (
	"reflect"
	"testing"
)

type testRule struct {
	code    Code
	finding *Finding
}

func (rule testRule) Code() Code { return rule.code }

func (rule testRule) Evaluate(Suite, Thresholds) Evaluation {
	return Evaluation{Finding: rule.finding}
}

func TestEnginePreservesRegistryOrder(t *testing.T) {
	rules := []Rule{
		testRule{code: CodeTailVariability, finding: &Finding{Code: CodeTailVariability}},
		testRule{code: CodeHighFirstByteWait, finding: &Finding{Code: CodeHighFirstByteWait}},
		testRule{code: CodeConnectionOverhead, finding: nil},
	}
	registry, err := NewRegistry(rules...)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(DefaultThresholds(), registry)
	if err != nil {
		t.Fatal(err)
	}

	report, err := engine.Evaluate(Suite{})
	if err != nil {
		t.Fatal(err)
	}
	got := []Code{report.Findings[0].Code, report.Findings[1].Code}
	want := []Code{CodeTailVariability, CodeHighFirstByteWait}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("finding order = %v, want %v", got, want)
	}
	if report.Findings[0].RuleVersion != CurrentRuleVersion {
		t.Fatalf("rule version = %q", report.Findings[0].RuleVersion)
	}
}

func TestRegistryRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name  string
		rules []Rule
	}{
		{name: "nil", rules: []Rule{nil}},
		{name: "empty code", rules: []Rule{testRule{}}},
		{name: "duplicate", rules: []Rule{testRule{code: CodeErrorRate}, testRule{code: CodeErrorRate}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRegistry(test.rules...); err == nil {
				t.Fatal("expected registry error")
			}
		})
	}
}

func TestEngineRejectsMismatchedFindingCode(t *testing.T) {
	registry, err := NewRegistry(testRule{
		code: CodeErrorRate, finding: &Finding{Code: CodeTailVariability},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(DefaultThresholds(), registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Evaluate(Suite{}); err == nil {
		t.Fatal("expected mismatched code error")
	}
}
