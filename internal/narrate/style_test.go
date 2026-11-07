package narrate

import (
	"context"
	"strings"
	"testing"

	"diegesis/internal/render"
)

func hapshoBackend() render.Backend { return render.Backend{ID: "h", Lang: "ko", Tone: "hapsho"} }

func TestRestyleKeepsOriginalWhenToneFails(t *testing.T) {
	f := &seqFake{replies: []string{"문이 열렸다. 가온이 들어갔다. 불이 꺼졌다."}}
	orig := "문이 열렸다. 가온이 들어갔다."
	got, ok, err := Restyle(context.Background(), f, orig, hapshoBackend())
	if err != nil || ok || got != orig || len(f.users) != 2 {
		t.Fatalf("%q %v %v calls=%d", got, ok, err, len(f.users))
	}
}

func TestRestylePassesOnTargetTone(t *testing.T) {
	want := "문이 열렸습니다. 가온이 들어갔습니다."
	f := &seqFake{replies: []string{want}}
	got, ok, err := Restyle(context.Background(), f, "문이 열렸다. 가온이 들어갔다.", hapshoBackend())
	if err != nil || !ok || got != want || len(f.users) != 1 {
		t.Fatalf("%q %v %v", got, ok, err)
	}
}

func TestRestyleEnglishNoCheck(t *testing.T) {
	f := &seqFake{replies: []string{"The door opened."}}
	got, ok, err := Restyle(context.Background(), f, "x", render.Backend{Lang: "en"})
	if err != nil || !ok || got != "The door opened." {
		t.Fatalf("%q %v %v", got, ok, err)
	}
}

func TestRestylePromptPreservesContent(t *testing.T) {
	b := hapshoBackend()
	b.Sample.Text = "견본 문장입니다."
	sys, user := RestylePrompt("원문.", b)
	all := sys + user
	for _, w := range []string{"문장 수", "대사", "사실", "견본 문장입니다.", "-습니다", "원문."} {
		if !strings.Contains(all, w) {
			t.Errorf("없음: %s", w)
		}
	}
}

func TestStitchPromptForbidsNewFacts(t *testing.T) {
	sys, user := StitchPrompt([]string{"하나.", "둘."}, "ko")
	if !strings.Contains(sys, "새 사실을 더하지 않는다") || !strings.Contains(sys, "새 대사를 더하지 않는다") {
		t.Fatal("새 사실·새 대사 금지 문구 없음")
	}
	if !strings.Contains(user, "하나.") || !strings.Contains(user, "둘.") {
		t.Fatal("비트 없음")
	}
}

func TestStitchReturnsTrimmedText(t *testing.T) {
	f := &seqFake{replies: []string{"  이어진 글.\n"}}
	got, err := Stitch(context.Background(), f, []string{"a", "b"}, "ko")
	if err != nil || got != "이어진 글." {
		t.Fatalf("%q %v", got, err)
	}
}
