package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Backend 는 교체할 수 있는 문체 백엔드 하나(styles/<id>.json): 언어, 목표 어투, 퍼블릭 도메인 샘플.
type Backend struct {
	ID     string      `json:"id"`
	Lang   string      `json:"lang"`
	Tone   string      `json:"tone"` // haera | hapsho | gojeon | "" (어투 판정 없음)
	Sample StyleSample `json:"sample"`
}

// 한국어 목표 어투 지시. ClassifyTone 이 세는 어미와 같은 것을 요구한다.
var toneRule = map[string]string{
	"haera": "서술문은 해라체로 쓴다. 따옴표 밖의 모든 서술 문장을 '-다'(했다, 이었다, 있다, 보았다)로 끝낸다. " +
		"'-습니다', '-요', '-더라', '-니라' 로 끝내지 않는다.",
	"hapsho": "서술문은 하십시오체로 쓴다. 따옴표 밖의 모든 서술 문장을 '-습니다/-ㅂ니다'(했습니다, 갔습니다, 입니다)로 끝낸다. " +
		"'-다', '-요', '-더라' 로 끝내지 않는다.",
	"gojeon": "서술문은 고전소설체로 쓴다. 따옴표 밖의 모든 서술 문장을 '-더라', '-니라', '-도다', '-로다' 같은 옛 소설 어미" +
		"(하더라, 이더라, 하였더라, 하니라, 이로다)로 끝낸다. '-다', '-습니다' 로 끝내지 않는다. " +
		"글자와 띄어쓰기는 현대 한글 맞춤법을 따른다(아래아 같은 옛 글자는 쓰지 않는다).",
}

const koDialogue = "대사(quote)는 원어 그대로 주어진다. 한국어로 옮겨 쓰되 뜻을 바꾸지 않고, 큰따옴표(“ ”) 안에 넣는다. " +
	"어투 규칙은 따옴표 밖의 서술문에만 적용된다."

// LoadBackends 는 dir/*.json 을 이름순으로 읽는다. 모르는 키, 언어가 다른 샘플, 퍼블릭 도메인이 아닌 샘플(2026),
// 모르는 어투는 오류다.
func LoadBackends(dir string) ([]Backend, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []Backend
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var b Backend
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if _, ok := langName[b.Lang]; !ok || b.Sample.Lang != b.Lang {
			return nil, fmt.Errorf("%s: lang %q, sample lang %q", p, b.Lang, b.Sample.Lang)
		}
		if _, ok := toneRule[b.Tone]; b.Tone != "" && (!ok || b.Lang != "ko") {
			return nil, fmt.Errorf("%s: unknown tone %q for %s", p, b.Tone, b.Lang)
		}
		if err := CheckPublicDomain(b.Sample, 2026); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, b)
	}
	return out, nil
}

// Prompt 는 M0 변형 c(샘플 + 렌더 지시어)에 목표 어투 지시를 더한다.
func (b Backend) Prompt(s Scene) (system, user string, err error) {
	system, user, err = BuildPrompt(s, VariantStyleDirectives, b.Lang, &b.Sample)
	if err != nil {
		return "", "", err
	}
	if b.Lang == "ko" {
		system += "\n" + koDialogue + "\n"
	}
	if r := toneRule[b.Tone]; r != "" {
		system += "\n목표 어투(반드시 지킨다): " + r + "\n"
	}
	return system, user, nil
}
