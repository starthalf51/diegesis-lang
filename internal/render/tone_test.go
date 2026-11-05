package render

import "testing"

func TestClassifyTone(t *testing.T) {
	sets := map[string]string{
		"haera":  "그는 문을 열었다. 바람이 차게 불었다. 누가 왔느냐? 어서 가자.\n방 안은 어두웠다…",
		"hapsho": "그는 문을 열었습니다. 바람이 붑니다. 누가 왔습니까? 어서 들어오십시오. 그것은 편지입니다!",
		"gojeon": "길동이 문을 열더라. 이는 하늘의 뜻이니라. 슬프도다! 내 이제 가노라. 그 일이 참으로 기이하도다.\n세월이 흘러 길동이 자라매 재주가 비상하니",
	}
	for want, prose := range sets {
		got := ClassifyTone(prose)
		if got[want] < 0.8 {
			t.Errorf("%s: share %.2f < 0.8 (%v)", want, got[want], got)
		}
	}
	if got := ClassifyTone(""); len(got) != 0 {
		t.Errorf("empty prose: %v", got)
	}
}

// 대사 속의 어미는 세지 않는다: 하십시오체 서술 + 해라체 대사.
func TestClassifyToneIgnoresDialogue(t *testing.T) {
	prose := `그가 “나는 간다. 너도 와라. 늦었다.” 하고 말했습니다.` + "\n" +
		`"빨리 가자. 시간이 없다." 그녀가 대답했습니다. 「죽여라. 살려 두지 마라.」 장군이 외쳤습니다.` + "\n" +
		`『이것이 무엇이냐?』 아이가 물었습니다. ‘ㄱ이다.’ 하고 누군가 속삭였습니다.`
	got := ClassifyTone(prose)
	if got["hapsho"] != 1 {
		t.Errorf("want all narration hapsho, got %v", got)
	}
}

func TestLastEojeol(t *testing.T) {
	for in, want := range map[string]string{" 그는 떠났다) ": "떠났다", "말했습니다.": "말했습니다", "가로되,": "", "Hello": ""} {
		if got := lastEojeol(in); got != want {
			t.Errorf("lastEojeol(%q) = %q, want %q", in, got, want)
		}
	}
}

// 대사를 따로 줄에 둔 서술: 대사 앞 이음 절("…가로되,", "…돌면서,")은 문장이 아니다.
func TestClassifyToneSkipsLeadInClauses(t *testing.T) {
	for want, prose := range map[string]string{
		"hapsho": `맥베스가 마녀들을 돌아보면서,
“너희는 누구냐?”
하고 물었습니다.
뱅쿠오가 가로되:
“사라졌다.”
그는 놀랐습니다.`,
		"gojeon": `베드로가 가로되,
“나는 그를 모르노라.”
하더라.
닭이 울매、
베드로가 그 말을 생각하고 울더라.`,
	} {
		if got := ClassifyTone(prose); got[want] != 1 {
			t.Errorf("%s: want 1, got %v", want, got)
		}
	}
}

// ASCII 작은따옴표는 아포스트로피와 구별할 수 없으니 대사 따옴표로 보지 않는다.
func TestClassifyToneKeepsApostrophes(t *testing.T) {
	got := ClassifyTone("Peter's 집에 갔습니다. 그는 떠났다. John's 말이 맞았습니다.")
	if got["haera"] < 0.33 || got["hapsho"] < 0.66 {
		t.Errorf("apostrophes swallowed narration: %v", got)
	}
}

// 세요는 해요체다.
func TestSeyoIsNotHapsho(t *testing.T) {
	if got := toneOf("앉으세요"); got == "hapsho" {
		t.Errorf("toneOf(앉으세요) = %s", got)
	}
}
