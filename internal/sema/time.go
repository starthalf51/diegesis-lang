package sema

import (
	"regexp"
	"strconv"
	"time"

	"story-lang/internal/ir"
	"story-lang/internal/syntax"
)

func i64(v int64) *int64 { return &v }

func (b *builder) edge(m *module, from, to string, min, max *int64, pos syntax.Pos) {
	if from == "" || to == "" {
		return
	}
	m.out.TimeEdges = append(m.out.TimeEdges, ir.TimeEdge{From: from, To: to, Min: min, Max: max, SrcID: b.srcID(pos)})
}

// addPoint emits a point; every point is after the origin.
func (b *builder) addPoint(m *module, id, kind string, pos syntax.Pos) {
	m.out.Points = append(m.out.Points, ir.Point{ID: id, WorkID: b.work, Kind: kind, SrcID: b.srcID(pos)})
	b.edge(m, b.origin, id, i64(0), nil, pos)
}

// when turns one `when` into edges against a container's start/end points
// (start == end for a single point such as @x.born).
func (b *builder) when(m *module, w *syntax.When, start, end string) {
	min, max := i64(0), (*int64)(nil)
	if w.Min != nil {
		min, max = i64(w.Min.Sec), i64(w.Max.Sec)
	}
	switch w.Op {
	case "after":
		_, x := b.tref(m, w.Ref)
		b.edge(m, x, start, min, max, w.Pos)
	case "before":
		x, _ := b.tref(m, w.Ref)
		b.edge(m, end, x, min, max, w.Pos)
	case "during":
		xs, xe := b.tref(m, w.Ref)
		b.edge(m, xs, start, i64(0), nil, w.Pos)
		b.edge(m, end, xe, i64(0), nil, w.Pos)
	case "at":
		from, next, ok := parseDate(w.Date)
		if !ok {
			b.errf(w.Pos, "syntax", "bad date %q (want Y, Y-MM or Y-MM-DD, ? for unknown parts)", w.Date)
			return
		}
		b.edge(m, b.origin, start, i64(b.secs(from)), i64(b.secs(next)), w.Pos)
	}
}

func (b *builder) secs(t time.Time) int64 { return t.Unix() - b.epoch.Unix() }

var dateRe = regexp.MustCompile(`^(\d{1,4})(?:-(\d{2}|\?))?(?:-(\d{2}|\?))?$`)

// parseDate returns the start of the date and the start of the next unit
// of its precision (year, month or day).
func parseDate(s string) (time.Time, time.Time, bool) {
	g := dateRe.FindStringSubmatch(s)
	if g == nil {
		return time.Time{}, time.Time{}, false
	}
	y, _ := strconv.Atoi(g[1])
	mo, d := 1, 1
	step := [3]int{1, 0, 0}
	if g[2] != "" && g[2] != "?" {
		mo, _ = strconv.Atoi(g[2])
		step = [3]int{0, 1, 0}
		if g[3] != "" && g[3] != "?" {
			d, _ = strconv.Atoi(g[3])
			step = [3]int{0, 0, 1}
		}
	} else if g[3] != "" && g[3] != "?" {
		return time.Time{}, time.Time{}, false // day without month
	}
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	if mo < 1 || mo > 12 || t.Day() != d {
		return time.Time{}, time.Time{}, false
	}
	return t, t.AddDate(step[0], step[1], step[2]), true
}

// setEpoch fixes the origin: the work's `start <date>`, else the earliest
// `when at` date in the work.
func (b *builder) setEpoch(start *syntax.Start, dates []string) {
	if start != nil {
		if t, _, ok := parseDate(start.Date); ok {
			b.epoch, b.hasEpoch = t, true
			return
		}
	}
	for _, s := range dates {
		if t, _, ok := parseDate(s); ok && (!b.hasEpoch || t.Before(b.epoch)) {
			b.epoch, b.hasEpoch = t, true
		}
	}
}
