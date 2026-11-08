---
module: yeoul.cast
mode: authored
---

# 여울섬 — 5화 추가 선언

```sl
character haesol "Kim Haesol" minor
  name ko "김해솔"
  attr haesol.age = 27
  believes haesol_posted

place police_post "the coast guard post"
  name ko "여울섬 해경 출장소"

fact taeo_recalled "윤태오는 본원 발령을 받아 아침 첫 배로 섬을 떠났다" true
fact haesol_posted "김해솔은 여울섬 해경 출장소에 새로 부임한 순경이다" true
fact haesol_helps "해솔은 태오가 떠난 자리를 이어 서윤의 조사를 돕기로 한다" true
fact gicheol_rescued "삼십 년 전 그 밤 문기철은 어선을 몰고 나가 해진호 생존자를 건져 올렸다" true
fact visit_gicheol "두 사람은 그 밤 구조에 나섰던 문기철을 찾아가 보기로 한다" true
fact gicheol_no_light "구조하던 밤 문기철은 등대 불빛을 본 기억이 없다" true
fact gicheol_hesitates "문기철은 등대 이야기가 나오자 말끝을 흐리고 서둘러 화제를 돌린다" true
fact haesol_record_plan "해솔은 출장소에 보관된 해진호 사고 기록을 찾아보기로 한다" true
fact record_weather "해경 기록에는 해진호 침몰 원인이 기상 악화로만 적혀 있다" true
fact record_no_light "해경 기록에는 등대에 관한 말이 한 줄도 없다" true
fact song_light_dark "서윤은 옛 노래의 '불이 잔다'가 그 밤 등대 불이 꺼져 있었다는 뜻이라고 짐작한다" true

artifact haegyeong_record "the coast guard accident record"
  name ko "해진호 사고 해경 기록철"
  contains record_weather, record_no_light

seoyun believes gicheol_rescued
gicheol believes gicheol_rescued
gicheol believes gicheol_no_light
```
