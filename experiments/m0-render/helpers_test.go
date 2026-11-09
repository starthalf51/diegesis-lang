package main

import (
	"context"
	"errors"
	"testing"

	. "github.com/starthalf51/diegesis-lang/internal/render"
)

// 장면·문체 픽스처는 internal/render/testdata 에 있다. min.json 은 실험의 -scenes 기본 디렉터리 밖에 둔다.
const fixtures = "../../internal/render/testdata/"

func mustLoad(t *testing.T, path string) Scene {
	t.Helper()
	s, err := LoadScene(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustStyle(t *testing.T, path string) StyleSample {
	t.Helper()
	s, err := LoadStyle(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

const validJSON = `{"acts":[],"facts_stated":[],"new_events":[]}`

var ctx = context.Background()

type fakeCompleter struct {
	replies []Result
	err     error
	calls   int
}

func (f *fakeCompleter) Complete(_ context.Context, _, _ string) (Result, error) {
	f.calls++
	if f.err != nil {
		return Result{}, f.err
	}
	if f.calls > len(f.replies) {
		return Result{}, errors.New("fakeCompleter: out of replies")
	}
	return f.replies[f.calls-1], nil
}

func testJob(t *testing.T) Job {
	t.Helper()
	return Job{Scene: mustLoad(t, fixtures+"min.json"), Variant: VariantIROnly, Lang: "en"}
}
