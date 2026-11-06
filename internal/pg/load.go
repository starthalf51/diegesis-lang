package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"diegesis/internal/ir"
)

// Load 는 IR 하나를 한 트랜잭션으로 적재한다. 작품 행을 지우고(CASCADE 로 하위 행까지)
// 모든 테이블을 COPY 한 뒤 derive 를 부른다. 판정과 파생은 전부 PG 가 한다(ADR-010).
func Load(ctx context.Context, db *pgxpool.Pool, w *ir.Work) error {
	if w == nil || len(w.Works) != 1 || w.Works[0].ID == "" {
		return fmt.Errorf("load: IR must have exactly one works row")
	}
	if w.Schema != ir.Schema {
		return fmt.Errorf("load: IR schema %d, want %d", w.Schema, ir.Schema)
	}
	id := w.Works[0].ID
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM public.work WHERE id=$1`, id); err != nil {
		return err
	}
	for _, t := range copies(w, id) {
		if len(t.rows) == 0 {
			continue
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"public", t.name}, t.cols, pgx.CopyFromRows(t.rows)); err != nil {
			return fmt.Errorf("load %s: %w", t.name, err)
		}
	}
	if _, err := tx.Exec(ctx, `SELECT public.derive($1)`, id); err != nil {
		return fmt.Errorf("derive: %w", err)
	}
	return tx.Commit(ctx)
}

type table struct {
	name string
	cols []string
	rows [][]any
}

// nz: 빈 참조는 NULL. sid: src_id 0(합성 행)은 NULL. arr: nil 목록은 '{}'.
func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func sid(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func arr(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func each[T any](xs []T, f func(T) []any) [][]any {
	out := make([][]any, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

func cols(s ...string) []string { return append([]string{"work_id"}, s...) }

// copies 는 FK 순서(work → src → 나머지)로 테이블을 나열한다. 열 순서는 ir 구조체 필드 순서다.
func copies(w *ir.Work, id string) []table {
	r := func(v ...any) []any { return append([]any{id}, v...) }
	ord := 0
	return []table{
		{"work", []string{"id", "title", "unit", "start", "src_id"}, each(w.Works, func(x ir.WorkRow) []any {
			return []any{x.ID, x.Title, x.Unit, x.Start, sid(x.SrcID)}
		})},
		{"src", cols("id", "module_id", "file", "line", "col"), each(w.Src, func(x ir.Src) []any {
			return r(x.ID, nz(x.Module), x.File, x.Line, x.Col)
		})},
		{"module", cols("id", "name", "paths", "imports"), each(w.Modules, func(x ir.Module) []any {
			return r(x.ID, x.Name, arr(x.Paths), arr(x.Imports))
		})},
		{"entity", cols("id", "module_id", "kind", "tier", "name", "knows", "src_id"), each(w.Entities, func(x ir.Entity) []any {
			return r(x.ID, nz(x.ModuleID), x.Kind, x.Tier, x.Name, nz(x.Knows), sid(x.SrcID))
		})},
		{"group_member", cols("group_id", "member_id", "src_id"), each(w.GroupMembers, func(x ir.GroupMember) []any {
			return r(x.GroupID, x.MemberID, sid(x.SrcID))
		})},
		{"name", cols("owner_id", "lang", "text", "ord", "src_id"), each(w.Names, func(x ir.Name) []any {
			return r(x.OwnerID, x.Lang, x.Text, x.Ord, sid(x.SrcID))
		})},
		{"role", cols("id", "names", "shared", "src_id"), each(w.Roles, func(x ir.Role) []any {
			return r(x.ID, arr(x.Names), x.Shared, sid(x.SrcID))
		})},
		{"role_holder", cols("role_id", "entity_id", "holder_is_role", "from_tref", "until_tref", "from_pt", "until_pt", "shared", "src_id"),
			each(w.RoleHolders, func(x ir.RoleHolder) []any {
				return r(x.RoleID, x.EntityID, x.HolderIsRole, x.FromTref, x.UntilTref, nz(x.FromPt), nz(x.UntilPt), x.Shared, sid(x.SrcID))
			})},
		{"point", cols("id", "kind", "src_id"), each(w.Points, func(x ir.Point) []any {
			return r(x.ID, x.Kind, sid(x.SrcID))
		})},
		{"time_edge", cols("ord", "from_pt", "to_pt", "min", "max", "waived", "waiver_rule", "waiver_reason", "src_id", "target_kind", "target_id"),
			each(w.TimeEdges, func(x ir.TimeEdge) []any {
				ord++
				return r(ord, x.From, x.To, x.Min, x.Max, x.Waived, x.WaiverRule, x.WaiverReason, sid(x.SrcID), x.TargetKind, x.TargetID)
			})},
		{"scene", cols("id", "module_id", "kind", "title", "place_id", "thread_id", "start_pt", "end_pt", "discourse_ord", "discourse_after", "shown", "src_id"),
			each(w.Scenes, func(x ir.Scene) []any {
				return r(x.ID, nz(x.ModuleID), x.Kind, x.Title, nz(x.PlaceID), nz(x.ThreadID), nz(x.StartPt), nz(x.EndPt),
					x.DiscourseOrd, nz(x.DiscourseAfter), x.Shown, sid(x.SrcID))
			})},
		{"presence", cols("entity_id", "scene_id", "place_id", "from_pt", "to_pt", "src_id"), each(w.Presence, func(x ir.Presence) []any {
			return r(x.EntityID, x.SceneID, nz(x.PlaceID), nz(x.FromPt), nz(x.ToPt), sid(x.SrcID))
		})},
		{"act", cols("id", "scene_id", "beat_id", "ord", "pt", "actor_id", "actor_is_role", "verb", "action", "objects", "to", "with",
			"prop_id", "quote", "because_id", "src_ref", "src_id"), each(w.Acts, func(x ir.Act) []any {
			return r(x.ID, nz(x.SceneID), nz(x.BeatID), x.Ord, x.Pt, x.ActorID, x.ActorIsRole, x.Verb, x.Action, arr(x.Objects),
				arr(x.To), arr(x.With), nz(x.PropID), x.Quote, nz(x.BecauseID), x.SrcRef, sid(x.SrcID))
		})},
		{"audience", cols("act_id", "entity_id"), each(w.Audience, func(x ir.Audience) []any {
			return r(x.ActID, x.EntityID)
		})},
		{"belief", cols("entity_id", "verb", "prop_id", "scene_id", "at_pt", "because_pt", "src_id"), each(w.Beliefs, func(x ir.Belief) []any {
			return r(x.EntityID, x.Verb, nz(x.PropID), nz(x.SceneID), nz(x.AtPt), nz(x.BecausePt), sid(x.SrcID))
		})},
		{"fact", cols("id", "text", "truth", "def", "src_id"), each(w.Facts, func(x ir.Fact) []any {
			var def any // typed nil 포인터는 JSON null 이 되므로 SQL NULL 로 바꾼다
			if x.Def != nil {
				def = x.Def
			}
			return r(x.ID, x.Text, x.Truth, def, sid(x.SrcID))
		})},
		{"artifact", cols("id", "module_id", "name", "contains", "text", "src_id"), each(w.Artifacts, func(x ir.Artifact) []any {
			return r(x.ID, nz(x.ModuleID), x.Name, arr(x.Contains), arr(x.Text), sid(x.SrcID))
		})},
		{"setup", cols("id", "kind", "prop_id", "text", "scene_id", "at_pt", "src_id"), each(w.Setups, func(x ir.Setup) []any {
			return r(x.ID, x.Kind, nz(x.PropID), x.Text, nz(x.SceneID), nz(x.AtPt), sid(x.SrcID))
		})},
		{"payoff", cols("setup_id", "scene_id", "at_pt", "module_id", "src_id"), each(w.Payoffs, func(x ir.Payoff) []any {
			return r(x.SetupID, nz(x.SceneID), nz(x.AtPt), nz(x.ModuleID), sid(x.SrcID))
		})},
		{"account", cols("id", "event_id", "source_id", "src_id"), each(w.Accounts, func(x ir.Account) []any {
			return r(x.ID, x.EventID, nz(x.SourceID), sid(x.SrcID))
		})},
		{"parent", cols("parent_id", "child_id", "src_id"), each(w.Parents, func(x ir.Parent) []any {
			return r(x.ParentID, x.ChildID, sid(x.SrcID))
		})},
		{"distance", cols("place_a", "place_b", "min", "max", "src_id"), each(w.Distances, func(x ir.Distance) []any {
			return r(x.PlaceA, x.PlaceB, x.Min, x.Max, sid(x.SrcID))
		})},
		{"reveal", cols("prop_id", "scene_id", "at_pt", "src_id"), each(w.Reveals, func(x ir.Reveal) []any {
			return r(nz(x.PropID), nz(x.SceneID), nz(x.AtPt), sid(x.SrcID))
		})},
		{"withhold", cols("prop_id", "scene_id", "until_pt", "src_id"), each(w.Withholds, func(x ir.Withhold) []any {
			return r(nz(x.PropID), nz(x.SceneID), nz(x.UntilPt), sid(x.SrcID))
		})},
		{"directive", cols("scene_id", "key", "values", "src_id"), each(w.Directives, func(x ir.Directive) []any {
			return r(nz(x.SceneID), x.Key, arr(x.Values), sid(x.SrcID))
		})},
		{"waiver", cols("rule", "target_kind", "target_id", "reason", "src_id"), each(w.Waivers, func(x ir.Waiver) []any {
			return r(x.Rule, x.TargetKind, nz(x.TargetID), x.Reason, sid(x.SrcID))
		})},
		{"attribute", cols("id", "entity_id", "key", "value", "kind", "at_pt", "account_id", "src_ref", "src_id"),
			each(w.Attributes, func(x ir.Attribute) []any {
				return r(x.ID, x.EntityID, x.Key, x.Value, x.Kind, nz(x.AtPt), nz(x.AccountID), x.SrcRef, sid(x.SrcID))
			})},
	}
}
