package sema

import (
	"regexp"
	"strconv"

	"diegesis/internal/ir"
	"diegesis/internal/syntax"
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
	e0 := len(m.out.TimeEdges)
	defer func() {
		for i := e0; i < len(m.out.TimeEdges); i++ {
			b.whenEdges = append(b.whenEdges, i)
		}
	}()
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
		day, prec, ok := parseDate(w.Date)
		if !ok {
			b.errf(w.Pos, "syntax", "bad date %q (want Y, Y-MM or Y-MM-DD, ? for unknown parts)", w.Date)
			return
		}
		lo := (day - b.epoch) * 86400
		b.edge(m, b.origin, start, i64(lo), i64(lo+prec*86400-1), w.Pos) // half-open window
	}
}

var dateRe = regexp.MustCompile(`^(\d{1,4})(?:-(\d{2}|\?))?(?:-(\d{2}|\?))?$`)

// monthDays is the no-leap calendar: dates use 365-day years so they agree
// with durations (1y = 365d).
var monthDays = [12]int64{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// parseDate returns the date's first day as a day number on the no-leap
// calendar (year*365 + day of year) and its precision in days (365 for a
// year, the month's length for a month, 1 for a day).
func parseDate(s string) (int64, int64, bool) {
	g := dateRe.FindStringSubmatch(s)
	if g == nil {
		return 0, 0, false
	}
	y, _ := strconv.ParseInt(g[1], 10, 64)
	day, prec := y*365, int64(365)
	if g[2] == "" || g[2] == "?" {
		return day, prec, g[3] == "" || g[3] == "?" // a day needs a month
	}
	mo, _ := strconv.Atoi(g[2])
	if mo < 1 || mo > 12 {
		return 0, 0, false
	}
	for _, n := range monthDays[:mo-1] {
		day += n
	}
	prec = monthDays[mo-1]
	if g[3] == "" || g[3] == "?" {
		return day, prec, true
	}
	d, _ := strconv.ParseInt(g[3], 10, 64)
	if d < 1 || d > monthDays[mo-1] {
		return 0, 0, false
	}
	return day + d - 1, 1, true
}

// setEpoch fixes the origin day: the work's `start <date>`, else the
// earliest `when at` date in the work.
func (b *builder) setEpoch(start *syntax.Start, dates []string) {
	if start != nil {
		if d, _, ok := parseDate(start.Date); ok {
			b.epoch, b.hasEpoch = d, true
			return
		}
	}
	for _, s := range dates {
		if d, _, ok := parseDate(s); ok && (!b.hasEpoch || d < b.epoch) {
			b.epoch, b.hasEpoch = d, true
		}
	}
}
