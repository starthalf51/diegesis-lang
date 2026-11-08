package pg_test

import (
	"context"
	"strings"
	"testing"

	"diegesis/internal/pg"
)

// 상자(box)는 hall 에 있고, 1화에서 비공개 사실(secret)을 a 가 보았다. 장면 2 의 요약을 본다.
const digestBody = `place hall "홀"
thing box "녹슨 상자"
  name ko "녹슨 상자"
  attr box.at = hall
fact secret "비밀문장XYZ" true
scene 1
  enter a, b
  withhold secret until 3
  a sees: secret
  a sees box
scene 2
  enter a, b
  a sees box
scene 3
  enter a, b
  a tells b: secret
`

func digestFor(t *testing.T) string {
	t.Helper()
	pool := open(t)
	id := compileLoad(t, pool, digestBody)
	d, err := pg.Digest(context.Background(), pool, id, "2")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDigestSections(t *testing.T) {
	d := digestFor(t)
	if !strings.HasPrefix(d, "## 이 시점의 세계 상태 (어기지 말 것)") {
		t.Errorf("heading: %q", d)
	}
	if !strings.Contains(d, "녹슨 상자(box): at = ") {
		t.Errorf("no thing location attribute:\n%s", d)
	}
	if strings.Contains(d, "독자가 이미 아는") || strings.Contains(d, "독자에게 이미 서술된") {
		t.Errorf("reader-knows section must be dropped:\n%s", d)
	}
}

func TestDigestOmitsWithheld(t *testing.T) {
	if d := digestFor(t); strings.Contains(d, "비밀문장XYZ") {
		t.Errorf("withheld fact text leaked:\n%s", d)
	}
}
