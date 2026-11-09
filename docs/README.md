> 공개용 스냅샷(2026-11-09). 설계 기준은 프로젝트 설계 노트.

# Diegesis 문서

- [language](language.md) — 언어의 개념 모델, 문법 원칙, 예제
- [file-format](file-format.md) — `.dieg.md` 파일 형식: 프론트매터, 언어 블록, 산문
- [compiler](compiler.md) — 컴파일 파이프라인, 시간 모델, 지식 파생, CLI
- [checks](checks.md) — 오류/경고 층위, 린트 규칙, 서사 패턴, 억제, `dieg test`
- [storage](storage.md) — PostgreSQL 스키마, 마이그레이션, 독자 모델, 지표, AGE 그래프
- [rendering](rendering.md) — 렌더러, 검수 카드, 상태 요약, 문장 패치, 문체 백엔드

Start with the [quickstart](quickstart.md).

문법의 단일 기준은 [`spec/grammar.ebnf`](../spec/grammar.ebnf) 다. `<!-- check -->` 표시가 붙은 `sl` 예제는 `go test ./cmd/dieg` 의 `TestDocsExamplesCompile` 이 컴파일해서 확인한다.
