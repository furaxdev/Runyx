package main

import "testing"

func TestSelfTestReplyOKAcceptsExpectedAnswers(t *testing.T) {
	cases := []string{"4", "Four", "The answer is 4.", "four!", " 4\n"}
	for _, c := range cases {
		if !selfTestReplyOK(c) {
			t.Errorf("selfTestReplyOK(%q) = false, want true", c)
		}
	}
}

func TestSelfTestReplyOKRejectsGibberishOrWrongAnswers(t *testing.T) {
	cases := []string{
		"", "5", "期内 spans lậputoryView sû泛utanutzade药业ueur结局例子洵 possibilità",
		"liga蝌úa_pemb除去 whipped少数展望神爷转入",
	}
	for _, c := range cases {
		if selfTestReplyOK(c) {
			t.Errorf("selfTestReplyOK(%q) = true, want false", c)
		}
	}
}
