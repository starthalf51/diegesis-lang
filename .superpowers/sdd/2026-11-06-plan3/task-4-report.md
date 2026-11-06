# Task 4 보고

## 변경
- `rules/object-location.sql`(warn, target act): 목적어 X(thing·artifact)의 그 시점 유효 `X.at`(account 밖, precedes=yes, 사이에 낀 더 나중 단언 제외)이 있고 장면 place_id 와 하나도 같지 않으면 경고. 메시지 `<actor> <action|verb> <X> in <place>, but <X> is at <X.at>` (does 면 동작 이름을 씀: "seojin ring bell ...").
- `rules/reveal-false.sql`(warn): `render_fact_true`(별칭·부정 추적)로 거짓인 명제의 reveal. target_kind = `reveal`(진단 표가 받는 종류, 대상 id = 장면 ID, 억제도 같은 키).
- sema: `shows` 목적어에 thing·artifact 허용(`objectKinds`), 무대 밖 shows 의 청중에서 사물 제외(`presence.go`). 믿음 파생 없음(shows 는 기존에도 제외).
- README `X.at` 규약 한 줄, grammar.ebnf shows 주석, `TestRules` 규칙 목록 갱신.

## 테스트 (먼저 실패 확인 후 통과)
- `TestObjectLocationWarns`, `TestObjectLocationMovedThing`, `TestRevealFalseWarns`(internal/pg/rules_plan3_test.go), `TestShowsThing`(internal/sema/fix6b_test.go; 사물이 청중에 없음도 확인).
- `go test ./... && go vet ./...` 전부 ok (dash ro_test 포함, SECURITY DEFINER 없음).

## 코퍼스 7편 dieg test
모두 PASS. 새 규칙 경고 0건(lint 로 직접 확인) -> 탐지율·mutation 비율 변화 없음(macbeth .81, chibi .90, bible .82, odyssey .88, hamlet .88, othello .96, holmes .97). 코퍼스에 X.at 단언이 없어 object-location 은 해당 없음, reveal-false 도 0건. 웨이버·코퍼스 수정 없음.

## idea-to-novel
`examples/idea-to-novel/b-diegesis/story` 는 위치 속성을 `attr bell.hung_at = tower`(cast:74)로 썼고 `.at` 이 아니라서 규칙이 **발동하지 않는다**(예제는 수정 안 함). 임시 사본에서 `hung_at -> at` 로 바꿔 lint 하면 `scenes/s9.dieg.md:39: warn object-location: seojin ring bell in staffroom, but bell is at tower` 가 나온다. 즉 규칙은 맞고 예제의 키 이름이 규약과 다르다.
