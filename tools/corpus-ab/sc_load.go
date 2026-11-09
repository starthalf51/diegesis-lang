package main

// Scorecard spike: reads only already loaded works from PG (no reload → no work_pkey duplicates).

import (
	"context"
	"os"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/internal/render"
)

type baseDef struct{ Pair, Work, Scene, Backend, ProsePath string }

var scBases = []baseDef{
	{"bible/mark_denial.ko-gojeon", "bible", "mark_denial", "ko-gojeon", ""},
	{"chibi/45.88.ko-hapsho", "chibi", "45.88", "ko-hapsho", ""},
	{"macbeth/1.3.ko-haera", "macbeth", "1.3", "ko-haera", ""},
	{"odyssey/19.1.ko-haera", "odyssey", "19.1", "ko-haera", ""},
	{"haejin/s3.ko-haera", "haejin", "haejin/haejin.scenes/s3", "ko-haera", "examples/renderer-ab/S-son/haejin_s3.ko-haera.md"},
}

func scScene(ctx context.Context, d baseDef) (render.Scene, render.Backend, string, error) {
	pool, err := pg.Connect(ctx)
	if err != nil {
		return render.Scene{}, render.Backend{}, "", err
	}
	defer pool.Close()
	b, err := pg.RenderScene(ctx, pool, d.Work, d.Scene)
	if err != nil {
		return render.Scene{}, render.Backend{}, "", err
	}
	s, err := render.ParseScene(b)
	if err != nil {
		return s, render.Backend{}, "", err
	}
	bs, err := render.LoadBackends("styles")
	if err != nil {
		return s, render.Backend{}, "", err
	}
	var be render.Backend
	for _, x := range bs {
		if x.ID == d.Backend {
			be = x
		}
	}
	var prose string
	if d.ProsePath != "" {
		raw, _ := os.ReadFile(d.ProsePath)
		prose = strings.TrimSpace(string(raw))
	} else {
		prose, _ = readP1(d.Pair)
	}
	return s, be, prose, nil
}
