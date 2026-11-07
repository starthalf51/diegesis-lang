package main

import (
	"fmt"
	"strings"
)

// Tok 은 한 모델의 토큰: In = input+cache_read+cache_write.
type Tok struct{ In, Out int64 }

// ArmStat 은 한 팔(ko-haera 실행만)의 집계. 건너뛴 실행은 N 에 들지 않는다.
type ArmStat struct {
	Arm, Renderer, Model string
	N, Passed            int
	PassRate             float64
	AvgCalls             float64
	Tokens               map[string]Tok
	Weighted             float64 // 장면당 평균 가중 토큰
	AvgTone, AvgChars    float64
	BeatRetries          int
	NewEvents, FactHall  int
	Errs                 int
}

type Decision struct{ Renderer, RenderModel, Why string }

type Summary struct {
	Used     int
	Scenes   []string
	Arms     []ArmStat
	Extra    []Run // 문체 교체·hapsho 단일 비교
	Decision Decision
}

func Summarize(runs []Run, w map[string][2]float64) Summary {
	var s Summary
	for _, a := range Arms {
		st := ArmStat{Arm: a.ID, Renderer: a.Renderer, Model: a.Model, Tokens: map[string]Tok{}}
		var calls, tone, chars, weighted float64
		for _, r := range runs {
			if r.Arm != a.ID || r.Skipped != "" {
				continue
			}
			st.N++
			if r.Passed {
				st.Passed++
			}
			if r.Err != "" {
				st.Errs++
			}
			calls += float64(r.Calls)
			tone += r.Tone
			chars += float64(r.Chars)
			st.BeatRetries += r.BeatRetries
			st.NewEvents += len(r.NewEvents)
			st.FactHall += len(r.FactHalluc)
			for _, mu := range []struct {
				m  string
				in int64
				o  int64
			}{{r.RenderModel, r.RenderUsage.Input + r.RenderUsage.CacheRead + r.RenderUsage.CacheWrite, r.RenderUsage.Output},
				{sonnet, r.ExtractUsage.Input + r.ExtractUsage.CacheRead + r.ExtractUsage.CacheWrite, r.ExtractUsage.Output}} {
				t := st.Tokens[mu.m]
				t.In, t.Out = t.In+mu.in, t.Out+mu.o
				st.Tokens[mu.m] = t
				weighted += float64(mu.in)*w[mu.m][0] + float64(mu.o)*w[mu.m][1]
			}
		}
		if st.N > 0 {
			n := float64(st.N)
			st.PassRate, st.AvgCalls, st.AvgTone, st.AvgChars, st.Weighted = float64(st.Passed)/n, calls/n, tone/n, chars/n, weighted/n
		}
		s.Arms = append(s.Arms, st)
	}
	for _, r := range runs {
		if !isArm(r.Arm) {
			s.Extra = append(s.Extra, r)
		}
	}
	s.Decision = Decide(s.Arms)
	return s
}

// Decide 는 브리프의 판정 규칙이다. 기본 렌더러 = 통과율이 가장 높은 팔(동률이면 가중 토큰이 적은 쪽)의 렌더러.
// 기본 렌더 모델 = 그 렌더러에서 haiku 통과율이 sonnet 대비 −10%p 이내면 haiku. 그 렌더러의 짝이 없으면 다른 렌더러의 짝으로 본다.
func Decide(arms []ArmStat) Decision {
	by := map[string]ArmStat{}
	var best *ArmStat
	for i := range arms {
		a := arms[i]
		if a.N == 0 {
			continue
		}
		by[a.Arm] = a
		if best == nil || a.PassRate > best.PassRate || (a.PassRate == best.PassRate && a.Weighted < best.Weighted) {
			best = &arms[i]
		}
	}
	if best == nil {
		return Decision{Why: "no runs"}
	}
	d := Decision{Renderer: best.Renderer}
	why := fmt.Sprintf("최고 통과율 %s %.0f%% (가중 토큰 %.0f)", best.Arm, best.PassRate*100, best.Weighted)
	pre := map[string]string{"single": "S", "layered": "L"}
	pairs := []string{pre[best.Renderer], pre[other(best.Renderer)]}
	for _, p := range pairs {
		son, ok1 := by[p+"-son"]
		hai, ok2 := by[p+"-hai"]
		if !ok1 || !ok2 {
			continue
		}
		d.RenderModel = sonnet
		if hai.PassRate >= son.PassRate-0.10-1e-9 {
			d.RenderModel = haiku
		}
		d.Why = fmt.Sprintf("%s; %s-hai %.0f%% vs %s-son %.0f%% → %s", why, p, hai.PassRate*100, p, son.PassRate*100, d.RenderModel)
		return d
	}
	d.RenderModel = best.Model
	d.Why = why + "; sonnet/haiku 짝 없음 → 최고 팔의 모델"
	return d
}

func other(r string) string {
	if r == "single" {
		return "layered"
	}
	return "single"
}

func short(m string) string { return strings.TrimSuffix(strings.TrimPrefix(m, "claude-"), "-5-5") }

// Table 은 README 에 옮길 결과 표(마크다운).
func Table(s Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "실제 호출 %d회\n\n", s.Used)
	for _, sc := range s.Scenes {
		fmt.Fprintf(&b, "- %s\n", sc)
	}
	b.WriteString("\n| 팔 | n | 통과 | 평균 호출 | sonnet 토큰 in/out | haiku 토큰 in/out | 가중 토큰/장면 | 어투 | 글자(목표 2500) | 비트 재시도 | 환각 new_event/사실형 | 오류 |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, a := range s.Arms {
		so, ha := a.Tokens[sonnet], a.Tokens[haiku]
		fmt.Fprintf(&b, "| %s | %d | %d/%d (%.0f%%) | %.1f | %d/%d | %d/%d | %.0f | %.2f | %.0f | %d | %d/%d | %d |\n",
			a.Arm, a.N, a.Passed, a.N, a.PassRate*100, a.AvgCalls, so.In, so.Out, ha.In, ha.Out, a.Weighted, a.AvgTone, a.AvgChars,
			a.BeatRetries, a.NewEvents, a.FactHall, a.Errs)
	}
	if len(s.Extra) > 0 {
		b.WriteString("\n| 추가 | 장면 | 백엔드 | 통과 | 호출 | 렌더 모델 in/out | sonnet in/out | 어투 |\n|---|---|---|---|---|---|---|---|\n")
		for _, r := range s.Extra {
			ru, xu := r.RenderUsage, r.ExtractUsage
			fmt.Fprintf(&b, "| %s | %s | %s | %v | %d | %s %d/%d | %d/%d | %.2f |%s\n", r.Arm, r.Scene, r.Backend, r.Passed, r.Calls,
				short(r.RenderModel), ru.Input+ru.CacheRead+ru.CacheWrite, ru.Output, xu.Input+xu.CacheRead+xu.CacheWrite, xu.Output, r.Tone, r.Skipped)
		}
	}
	d := s.Decision
	fmt.Fprintf(&b, "\n판정: 렌더러 %s, 렌더 모델 %s — %s\n", d.Renderer, d.RenderModel, d.Why)
	return b.String()
}

func isArm(id string) bool {
	for _, a := range Arms {
		if a.ID == id {
			return true
		}
	}
	return false
}
