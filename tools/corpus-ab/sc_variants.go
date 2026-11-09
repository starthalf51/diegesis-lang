package main

// 스코어카드 스파이크: 변형본(정답이 기계적으로 정해진) 정의·생성. 호출 없음.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

type edit struct{ Old, New string }

type vdef struct {
	Name, Base, Kind string
	Edits            []edit
	Absent           []string // 이 사실이 산문에서 빠진다(fact_present=false)
	Wrong            []string // 사실은 있되 주체·동사가 틀린다(as_stated=false)
	Leak             []string // 숨길 사실 W* 가 드러난다
	Invented         []string // 끼워 넣은 문장(있는 그대로)
}

type iTruth struct{ Present, AsStated, Unsure bool }

type vTruth struct {
	Name, Base, Kind string
	Expect           map[string]iTruth
	Withheld         map[string]bool
	Invented         []string
	Mutated          []string
	Target           int // S4 목표 글자(원본 산문)
}

// haejin 은 withheld 가 없어서 스파이크 안에서만 minjae_leaves 를 숨긴다: 해당 문장을 지운 것이 기준 산문이다.
var haejinW = []edit{{"\n\n“급한 회사 일이 생겨서 오늘 오후 배로 육지에 나가야 해. 섬에는 돌아오지 않을 거야.”\n\n서윤은 그 말을 듣고도 한동안 말이 없었다. 이윽고", "\n\n서윤은 한동안 말이 없었다. 이윽고"}}

var chibiD1 = edit{"그때 주유가 고개를 들고 게슴츠레한 눈으로 장간을 바라보며 말합니다.\n\n“친구, 하루 이틀 안에 조조의 머리를 보여 주겠소.”\n\n", ""}
var chibiD3 = []edit{chibiD1,
	{" 그러고는 짐짓 위로하는 목소리로 말합니다.\n\n“조조 편의 사자 노릇을 하느라 먼 길을 다니며 고생이 많으셨구려.”", ""},
	{"장간은 웃는 낯으로 고개를 저으며 대답합니다.\n\n“오랫동안 뵙지 못했기에 옛정을 생각해서 찾아왔소. 나더러 조조 편의 사자라니 무슨 말씀이오?”", "장간은 웃는 낯으로 대답합니다.\n\n“오랫동안 뵙지 못했기에 옛정을 생각해서 찾아왔소.”"},
	{"주유는 빙그레 웃으면서 말을 돌려, 장간이 조조를 위해 자기를 설득하러 왔다는 속뜻을 드러냅니다.\n\n“내가 사광처럼 음악을 잘 아는 사람은 아니지만, 곡 속에 담긴 뜻은 알아들을 수 있소.”", "주유는 빙그레 웃으면서 말을 돌립니다."}}

var macD1 = edit{" 또한 캐더의 영주가 노르웨이 편에 서서 던컨 왕에게 맞섰다는 사실을 밝혔다. 그는 차분한 어조로 말을 이었다.\n\n“사형에 해당하는 반역죄가 자백되고 입증되어 그를 쓰러뜨렸소.”", ""}
var macD3 = []edit{macD1,
	{"그리고 그는 세 여인의 얼굴을 하나씩 살폈다. 그가 마지막 말을 내뱉을 때, 그의 입가에는 가벼운 비웃음이 스쳤다.\n\n“번성한 신사.”\n\n", ""},
	{" 셋째 마녀가 이번에는 뱅쿼를 향해 손가락을 들었다.\n\n“그대는 왕을 낳으리라, 그대 자신은 왕이 아니어도.”", ""},
	{"뱅쿼가 맥베스 쪽으로 몸을 돌렸다. 맥베스는 그를 바라보며 말했다.\n\n“그대의 자식들이 왕이 되리라.”\n\n", ""}}

const invChibi = "그때 장막 밖에서 큰 불길이 일어 군량을 쌓아 둔 창고 한 채가 순식간에 타올랐으나, 장수들은 아무도 돌아보지 않습니다."
const invHaejin = "그때 문이 벌컥 열리고 이웃집 노인이 뛰어 들어와 서윤의 손에 낡은 돈 봉투를 쥐여 주고는 말없이 돌아갔다."
const invMac = "그때 로스가 품에서 던컨 왕의 인장이 찍힌 서신을 꺼내 맥베스의 손에 쥐여 주었다."

// 변형 목록 = 실행 우선순위 순서.
var scVariants = []vdef{
	{Name: "orig-chibi", Base: "chibi/45.88.ko-hapsho", Kind: "orig"},
	{Name: "d1-chibi", Base: "chibi/45.88.ko-hapsho", Kind: "D1", Edits: []edit{chibiD1}, Absent: []string{"E5"}},
	{Name: "d3-chibi", Base: "chibi/45.88.ko-hapsho", Kind: "D3", Edits: chibiD3, Absent: []string{"E1", "E2", "E5"}},
	{Name: "h-chibi", Base: "chibi/45.88.ko-hapsho", Kind: "H", Edits: []edit{{"장간은 그 편지를 자기 소매 속에 넣습니다.", "장간은 그 편지를 자기 소매 속에 넣습니다.\n\n" + invChibi}}, Invented: []string{invChibi}},
	{Name: "orig-macbeth", Base: "macbeth/1.3.ko-haera", Kind: "orig"},
	{Name: "d1-macbeth", Base: "macbeth/1.3.ko-haera", Kind: "D1", Edits: []edit{macD1}, Absent: []string{"E12"}},
	{Name: "d3-macbeth", Base: "macbeth/1.3.ko-haera", Kind: "D3", Edits: macD3, Absent: []string{"E12", "E7", "E4", "E8"}},
	{Name: "k-macbeth", Base: "macbeth/1.3.ko-haera", Kind: "K", Edits: []edit{{"로스가 맥베스 앞에 멈추어 서서 허리를 굽혔다.", "앵거스가 맥베스 앞에 멈추어 서서 허리를 굽혔다."}}, Wrong: []string{"E10"}},
	{Name: "orig-bible", Base: "bible/mark_denial.ko-gojeon", Kind: "orig"},
	{Name: "d1-bible", Base: "bible/mark_denial.ko-gojeon", Kind: "D1", Edits: []edit{{"그때 닭이 두 번째로 우는 소리를 듣더라.\n\n", ""}}, Absent: []string{"E7"}},
	{Name: "k-bible", Base: "bible/mark_denial.ko-gojeon", Kind: "K", Edits: []edit{{"문간으로 나가니, 닭이 우는 소리를 듣더라.", "문간으로 나가니, 여종이 닭이 우는 소리를 듣더라."}}, Wrong: []string{"E4"}},
	{Name: "orig-haejin", Base: "haejin/s3.ko-haera", Kind: "orig"},
	{Name: "h-haejin", Base: "haejin/s3.ko-haera", Kind: "H", Edits: []edit{{"서윤은 민재를 돌아보았다.", invHaejin + "\n\n서윤은 민재를 돌아보았다."}}, Invented: []string{invHaejin}},
	{Name: "l-haejin", Base: "haejin/s3.ko-haera", Kind: "L", Edits: []edit{{"민재는 잠깐 서윤을 바라보다가 시선을 내렸다.", "민재는 잠깐 서윤을 바라보다가 시선을 내렸다.\n\n“나 오늘 오후 배 타고 육지로 나가. 회사 일이 급해서, 이제 섬에는 돌아오지 않아.”"}}, Leak: []string{"W1"}},
	{Name: "orig-odyssey", Base: "odyssey/19.1.ko-haera", Kind: "orig"},
	// 예산이 남으면 쓰는 추가분
	{Name: "h-macbeth", Base: "macbeth/1.3.ko-haera", Kind: "H", Edits: []edit{{"앵거스가 말머리를 돌렸다.", invMac + "\n\n앵거스가 말머리를 돌렸다."}}, Invented: []string{invMac}},
}

func baseDefOf(pair string) baseDef {
	for _, d := range scBases {
		if d.Pair == pair {
			return d
		}
	}
	panic("no base " + pair)
}

func applyEdits(p string, es []edit) (string, error) {
	for _, e := range es {
		if n := strings.Count(p, e.Old); n != 1 {
			return "", fmt.Errorf("edit matches %d times: %.40q", n, e.Old)
		}
		p = strings.Replace(p, e.Old, e.New, 1)
	}
	return p, nil
}

// scBase: 장면(스파이크용 withheld 반영)·백엔드·기준 산문.
func scBase(ctx context.Context, pair string) (render.Scene, render.Backend, string, error) {
	d := baseDefOf(pair)
	s, b, prose, err := scScene(ctx, d)
	if err != nil {
		return s, b, "", err
	}
	prose = strings.ReplaceAll(prose, "\r\n", "\n")
	if d.Work == "haejin" {
		var bt []render.Beat
		for _, x := range s.Beats {
			y := x
			y.Acts = nil
			for _, a := range x.Acts {
				if a.Fact != "minjae_leaves" {
					y.Acts = append(y.Acts, a)
				}
			}
			bt = append(bt, y)
		}
		s.Beats, s.Withheld = bt, []string{"minjae_leaves"}
		prose, err = applyEdits(prose, haejinW)
	}
	return s, b, prose, err
}

func baseTruth(pair string, v vSet) map[string]iTruth {
	m := map[string]iTruth{}
	var tr truth
	raw, _ := os.ReadFile("examples/corpus-ab/verifier/truth.json")
	_ = json.Unmarshal(raw, &tr)
	for _, it := range v.Expect {
		t := iTruth{Present: true, AsStated: true}
		if l, ok := tr[pair][it.ID]; ok {
			t = iTruth{Present: l.Present, AsStated: l.Present, Unsure: l.Unsure}
		}
		m[it.ID] = t
	}
	return m
}

func scGen(ctx context.Context, out string) error {
	for _, d := range []string{"variants", "truth", "runs", "calls", "old"} {
		_ = os.MkdirAll(filepath.Join(out, d), 0o755)
	}
	for _, vd := range scVariants {
		s, b, prose, err := scBase(ctx, vd.Base)
		if err != nil {
			return err
		}
		v := buildSet(s, b.Lang, vd.Base, prose)
		t := vTruth{Name: vd.Name, Base: vd.Base, Kind: vd.Kind, Expect: baseTruth(vd.Base, v), Withheld: map[string]bool{}, Invented: vd.Invented, Target: nws(prose)}
		for _, w := range v.Withheld {
			t.Withheld[w.ID] = false
		}
		p, err := applyEdits(prose, vd.Edits)
		if err != nil {
			return fmt.Errorf("%s: %w", vd.Name, err)
		}
		for _, id := range vd.Absent {
			t.Expect[id] = iTruth{}
			t.Mutated = append(t.Mutated, id)
		}
		for _, id := range vd.Wrong {
			x := t.Expect[id]
			x.AsStated = false
			t.Expect[id] = x
			t.Mutated = append(t.Mutated, id)
		}
		for _, id := range vd.Leak {
			t.Withheld[id] = true
			t.Mutated = append(t.Mutated, id)
		}
		_ = os.WriteFile(filepath.Join(out, "variants", vd.Name+".md"), []byte(p+"\n"), 0o644)
		j, _ := json.MarshalIndent(t, "", "  ")
		_ = os.WriteFile(filepath.Join(out, "truth", vd.Name+".json"), j, 0o644)
	}
	return nil
}

func readVariant(out, name string) (string, vTruth) {
	b, _ := os.ReadFile(filepath.Join(out, "variants", name+".md"))
	var t vTruth
	j, _ := os.ReadFile(filepath.Join(out, "truth", name+".json"))
	_ = json.Unmarshal(j, &t)
	return strings.TrimSpace(string(b)), t
}
