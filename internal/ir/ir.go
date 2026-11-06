// Package ir is the compiler output: one slice per PG table (vault
// storage.md), rows in source order. IDs are stable `work/module/local`
// strings. src_id 0 means "synthetic" (no source line), loaded as NULL.
package ir

// Schema is bumped whenever a table or field changes.
const Schema = 3

type Work struct {
	Schema       int           `json:"schema"`
	Works        []WorkRow     `json:"works"`
	Modules      []Module      `json:"modules"`
	Src          []Src         `json:"src"`
	Entities     []Entity      `json:"entities"`
	GroupMembers []GroupMember `json:"group_members"`
	Names        []Name        `json:"names"`
	Roles        []Role        `json:"roles"`
	RoleHolders  []RoleHolder  `json:"role_holders"`
	Points       []Point       `json:"points"`
	TimeEdges    []TimeEdge    `json:"time_edges"`
	Scenes       []Scene       `json:"scenes"`
	Presence     []Presence    `json:"presence"`
	Acts         []Act         `json:"acts"`
	Audience     []Audience    `json:"audience"`
	Beliefs      []Belief      `json:"beliefs"`
	Facts        []Fact        `json:"facts"`
	Artifacts    []Artifact    `json:"artifacts"`
	Setups       []Setup       `json:"setups"`
	Payoffs      []Payoff      `json:"payoffs"`
	Accounts     []Account     `json:"accounts"`
	Parents      []Parent      `json:"parents"`
	Distances    []Distance    `json:"distances"`
	Reveals      []Reveal      `json:"reveals"`
	Withholds    []Withhold    `json:"withholds"`
	Directives   []Directive   `json:"directives"`
	Waivers      []Waiver      `json:"waivers"`
	Attributes   []Attribute   `json:"attributes"`
	PatternDecls []PatternDecl `json:"pattern_decls"`
}

type WorkRow struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Unit  string `json:"unit"`
	Start string `json:"start"` // raw `start <date>`; origin of the time axis
	SrcID int    `json:"src_id"`
}

type Module struct {
	ID      string   `json:"id"` // work/module
	WorkID  string   `json:"work_id"`
	Name    string   `json:"name"`
	Paths   []string `json:"paths"`
	Imports []string `json:"imports"` // module ids
}

type Src struct {
	ID     int    `json:"id"`
	Module string `json:"module_id"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col"`
}

// Entity kinds: character group place thing faction thread.
type Entity struct {
	ID       string `json:"id"`
	WorkID   string `json:"work_id"`
	ModuleID string `json:"module_id"`
	Kind     string `json:"kind"`
	Tier     string `json:"tier"`
	Name     string `json:"name"`
	Knows    string `json:"knows"` // oracle | omniscient | ""
	SrcID    int    `json:"src_id"`
}

type GroupMember struct {
	GroupID  string `json:"group_id"`
	MemberID string `json:"member_id"`
	SrcID    int    `json:"src_id"`
}

// Name: every name of an entity/role/fact/artifact. Lang "" = the decl's own
// strings (Ord 0 is the display name, the rest are aliases); otherwise `name <lang>`.
type Name struct {
	OwnerID string `json:"owner_id"`
	Lang    string `json:"lang"`
	Text    string `json:"text"`
	Ord     int    `json:"ord"`
	SrcID   int    `json:"src_id"`
}

type Role struct {
	ID     string   `json:"id"`
	WorkID string   `json:"work_id"`
	Names  []string `json:"names"`
	Shared bool     `json:"shared"`
	SrcID  int      `json:"src_id"`
}

// RoleHolder is raw: spans are resolved in PG. FromPt/UntilPt are the
// point ids the trefs name ("" = open end).
type RoleHolder struct {
	RoleID       string `json:"role_id"`
	EntityID     string `json:"entity_id"`
	HolderIsRole bool   `json:"holder_is_role"`
	FromTref     string `json:"from_tref"`
	UntilTref    string `json:"until_tref"`
	FromPt       string `json:"from_pt"`
	UntilPt      string `json:"until_pt"`
	Shared       bool   `json:"shared"`
	SrcID        int    `json:"src_id"`
}

// Point kinds: origin scene_start scene_end beat act born died.
type Point struct {
	ID     string `json:"id"`
	WorkID string `json:"work_id"`
	Kind   string `json:"kind"`
	SrcID  int    `json:"src_id"`
}

// TimeEdge: Min ≤ To − From ≤ Max (seconds). nil = unbounded.
type TimeEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Min          *int64 `json:"min"`
	Max          *int64 `json:"max"`
	Waived       bool   `json:"waived"`
	WaiverRule   string `json:"waiver_rule"`
	WaiverReason string `json:"waiver_reason"`
	SrcID        int    `json:"src_id"`
	// TargetKind/TargetID: waiver target of the statement whose own `when` made the edge ("" = order edge).
	TargetKind string `json:"target_kind,omitempty"`
	TargetID   string `json:"target_id,omitempty"`
}

// Scene is any container with start/end points: Kind scene (Shown),
// event (off stage) or account (one record of an event).
type Scene struct {
	ID             string `json:"id"`
	WorkID         string `json:"work_id"`
	ModuleID       string `json:"module_id"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	PlaceID        string `json:"place_id"`
	ThreadID       string `json:"thread_id"`
	StartPt        string `json:"start_pt"`
	EndPt          string `json:"end_pt"`
	DiscourseOrd   int    `json:"discourse_ord"`   // file order of shown scenes, 1-based; 0 = not shown
	DiscourseAfter string `json:"discourse_after"` // `discourse after X` scene id; PG reorders
	Shown          bool   `json:"shown"`
	SrcID          int    `json:"src_id"`
}

// Presence: entity on stage between FromPt (last point before enter) and
// ToPt (first point after exit, or scene end).
type Presence struct {
	EntityID string `json:"entity_id"`
	SceneID  string `json:"scene_id"`
	PlaceID  string `json:"place_id"`
	FromPt   string `json:"from_pt"`
	ToPt     string `json:"to_pt"`
	SrcID    int    `json:"src_id"`
}

// Act is also a point (Pt = ID).
type Act struct {
	ID          string   `json:"id"`
	SceneID     string   `json:"scene_id"`
	BeatID      string   `json:"beat_id"`
	Ord         int      `json:"ord"`
	Pt          string   `json:"pt"`
	ActorID     string   `json:"actor_id"`
	ActorIsRole bool     `json:"actor_is_role"`
	Verb        string   `json:"verb"`
	Action      string   `json:"action"`
	Objects     []string `json:"objects"`
	To          []string `json:"to"`
	With        []string `json:"with"`
	PropID      string   `json:"prop_id"`
	Quote       string   `json:"quote"`
	BecauseID   string   `json:"because_id"`
	SrcRef      string   `json:"src_ref"`
	SrcID       int      `json:"src_id"`
}

type Audience struct {
	ActID    string `json:"act_id"`
	EntityID string `json:"entity_id"`
}

// Belief is an explicit `believes`/`doubts` statement at AtPt.
type Belief struct {
	EntityID  string `json:"entity_id"`
	Verb      string `json:"verb"`
	PropID    string `json:"prop_id"`
	SceneID   string `json:"scene_id"`
	AtPt      string `json:"at_pt"`
	BecausePt string `json:"because_pt"`
	SrcID     int    `json:"src_id"`
}

// Fact: declared facts (truth true|false|derived|unknown) and every
// proposition expression used anywhere (truth derived, id = work/<canonical>).
type Fact struct {
	ID     string   `json:"id"`
	WorkID string   `json:"work_id"`
	Text   string   `json:"text"`
	Truth  string   `json:"truth"`
	Def    *PropDef `json:"def"`
	Tag    string   `json:"tag,omitempty"` // `fact … tag <name>` (identity = 정체 사실, recognition 패턴)
	SrcID  int      `json:"src_id"`
}

// PropDef: exactly one of Pred (+Args), Not, Alias is set.
type PropDef struct {
	Pred  string   `json:"pred,omitempty"`
	Args  []string `json:"args,omitempty"`
	Not   string   `json:"not,omitempty"`
	Alias string   `json:"alias,omitempty"`
}

type Artifact struct {
	ID       string   `json:"id"`
	WorkID   string   `json:"work_id"`
	ModuleID string   `json:"module_id"`
	Name     string   `json:"name"`
	Contains []string `json:"contains"`
	Text     []string `json:"text"`
	SrcID    int      `json:"src_id"`
}

// Setup.AtPt is the last point before the statement ("" at top level).
type Setup struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	PropID  string `json:"prop_id"`
	Text    string `json:"text"`
	SceneID string `json:"scene_id"`
	AtPt    string `json:"at_pt"`
	SrcID   int    `json:"src_id"`
}

type Payoff struct {
	SetupID  string `json:"setup_id"`
	SceneID  string `json:"scene_id"`
	AtPt     string `json:"at_pt"`
	ModuleID string `json:"module_id"`
	SrcID    int    `json:"src_id"`
}

type Account struct {
	ID       string `json:"id"`
	EventID  string `json:"event_id"`
	SourceID string `json:"source_id"`
	SrcID    int    `json:"src_id"`
}

type Parent struct {
	ParentID string `json:"parent_id"`
	ChildID  string `json:"child_id"`
	SrcID    int    `json:"src_id"`
}

type Distance struct {
	PlaceA string `json:"place_a"`
	PlaceB string `json:"place_b"`
	Min    int64  `json:"min"`
	Max    int64  `json:"max"`
	SrcID  int    `json:"src_id"`
}

type Reveal struct {
	PropID  string `json:"prop_id"`
	SceneID string `json:"scene_id"`
	AtPt    string `json:"at_pt"`
	SrcID   int    `json:"src_id"`
}

type Withhold struct {
	PropID  string `json:"prop_id"`
	SceneID string `json:"scene_id"`
	UntilPt string `json:"until_pt"`
	SrcID   int    `json:"src_id"`
}

type Directive struct {
	SceneID string   `json:"scene_id"`
	Key     string   `json:"key"`
	Values  []string `json:"values"`
	SrcID   int      `json:"src_id"`
}

// Waiver target kinds: work entity role fact artifact scene beat act when
// presence setup payoff belief reveal withhold distance parent life directive.
type Waiver struct {
	Rule       string `json:"rule"`
	TargetKind string `json:"target_kind"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	SrcID      int    `json:"src_id"`
}

// Attribute is one `attr <entity>.<key> = <value>` assertion. Kind int|str|id
// (id values are resolved decl ids, or the bare word when undeclared). AtPt ""
// = true from the start (top level or a declaration block). AccountID is the
// account the statement sits in ("" = not inside an account).
type Attribute struct {
	ID        string `json:"id"`
	EntityID  string `json:"entity_id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Kind      string `json:"kind"`
	AtPt      string `json:"at_pt"`
	AccountID string `json:"account_id"`
	SrcRef    string `json:"src_ref"`
	SrcID     int    `json:"src_id"`
}

// PatternDecl is one author declaration `pattern <id> [at <scene>]`. ID is
// <scene or module id>#<pattern> (the waiver target of pattern-missing).
// SceneID "" = anywhere in the work.
type PatternDecl struct {
	ID      string `json:"id"`
	Pattern string `json:"pattern"`
	SceneID string `json:"scene_id"`
	SrcID   int    `json:"src_id"`
}
