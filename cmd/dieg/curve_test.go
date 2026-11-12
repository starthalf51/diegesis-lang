package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestValleys(t *testing.T) {
	if got, want := Valleys([]float64{5, 5, 5, 1, 5}), []bool{false, false, false, true, false}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	if got := Valleys([]float64{4, 4, 6, 6}); !reflect.DeepEqual(got, []bool{true, true, false, false}) {
		t.Errorf("boundary: got %v", got)
	}
	if got := Valleys([]float64{0.1, 0.1, 0.1}); !reflect.DeepEqual(got, []bool{false, false, false}) {
		t.Errorf("flat float: got %v", got)
	}
	if got := Valleys([]float64{3, 3, 3}); !reflect.DeepEqual(got, []bool{false, false, false}) {
		t.Errorf("flat: got %v", got)
	}
}

func TestCurveFormat(t *testing.T) {
	rows := markValleys([]curveRow{{"a", 1, 2, 5, false}, {"b", 3, 1, 5, false}, {"c", 4, 1, 5, false}, {"d", 5, 1, 1, false}, {"e", 6, 1, 5, false}})
	var buf bytes.Buffer
	writeCurve(&buf, rows, false)
	want := "a 2 5.00\nb 1 5.00\nc 1 5.00\nd 1 1.00  valley\ne 1 5.00\nvalleys: d\n"
	if buf.String() != want {
		t.Errorf("got %q want %q", buf.String(), want)
	}
	buf.Reset()
	writeCurve(&buf, rows, true)
	var js []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &js); err != nil || len(js) != 5 || js[3]["valley"] != true || js[0]["module"] != "a" || js[0]["first_ord"] != 1.0 {
		t.Errorf("json: %v %s", err, buf.String())
	}
	buf.Reset()
	writeCurve(&buf, nil, false)
	if !strings.HasSuffix(buf.String(), "valleys: \n") {
		t.Errorf("empty: %q", buf.String())
	}
}

func TestGroupChapters(t *testing.T) {
	// Out of order, unequal n: weighted mean (2.75) != unweighted (3); min first_ord != first seen.
	in := []curveRow{
		{"mobydick.ch033", 12, 1, 5, false},
		{"mobydick.ch032b", 14, 3, 2, false}, {"mobydick.ch032c", 18, 4, 3, false}, {"mobydick.ch032a", 10, 1, 4, false},
		{"mobydick.epilogue", 40, 1, 1, false}, {"macbeth.act1", 50, 3, 2, false},
	}
	got := groupChapters(in)
	want := []curveRow{{"mobydick.ch032", 10, 8, 2.75, false}, {"mobydick.ch033", 12, 1, 5, false}, {"mobydick.epilogue", 40, 1, 1, false}, {"macbeth.act1", 50, 3, 2, false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
