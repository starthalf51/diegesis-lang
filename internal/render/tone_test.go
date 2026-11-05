package render

import "testing"

func TestClassifyTone(t *testing.T) {
	sets := map[string]string{
		"haera":  "그는 문을 열었다. 바람이 차게 불었다. 누가 왔느냐? 어서 가자.\n방 안은 어두웠다…",
		"hapsho": "그는 문을 열었습니다. 바람이 붑니다. 누가 왔습니까? 어서 들어오십시오. 이리 앉으세요. 그것은 편지입니다!",
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
		`『이것이 무엇이냐?』 아이가 물었습니다. 'ㄱ이다.' 하고 누군가 속삭였습니다.`
	got := ClassifyTone(prose)
	if got["hapsho"] != 1 {
		t.Errorf("want all narration hapsho, got %v", got)
	}
}

func TestLastEojeol(t *testing.T) {
	for in, want := range map[string]string{" 그는 떠났다) ": "떠났다", "말했습니다,": "말했습니다", "Hello": ""} {
		if got := lastEojeol(in); got != want {
			t.Errorf("lastEojeol(%q) = %q, want %q", in, got, want)
		}
	}
}
