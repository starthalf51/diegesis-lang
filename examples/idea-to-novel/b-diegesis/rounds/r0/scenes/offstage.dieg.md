---
module: bungyo.scenes
import: [bungyo.cast]
mode: authored
---

# 무대 밖 사건

서진이 보건실에서 잠든 사이(첫날 오후 두 시 반 무렵 ~ 세 시 이십 분)에 일어난 일들.
렌더하지 않는다. 누가 무엇을 언제 알게 되었는지만 적는다.

```sl
# 첫날 새벽: 주무관이 고갯길이 끊긴 것을 본다
event snowfall "밤새 내린 폭설"
  when before s1 by 1h..3h
  enter custodian
  custodian sees road_cut
  exit all

# 첫날 오후: 주무관은 정문 아래 비탈에서 눈을 치운다. 본 사람은 없다
event gate_work "정문 아래 비탈 제설"
  when after s3 by 5m..15m
  lasts 55m..70m
  enter custodian
  custodian does shovel
  attr custodian.alibi = "정문 아래 비탈길 제설"
  custodian believes custodian_at_gate
  custodian believes custodian_no_ring
  exit all

# 오후 두 시 오십이 분: 종탑 계단
event fall "종탑 계단에서"
  when after s3.end by 5m..15m
  lasts 5m..10m
  enter principal, teacher
  teacher says principal: "교장 선생님, 그 서류 제게 주세요."
  principal tells teacher: closure_plan "이미 정한 일이야, 정 선생."
  teacher does push principal because closure_plan
  dies principal
  attr watch.stopped_at = "오후 두 시 오십이 분"
  attr mitten.lost_at = tower
  teacher sees principal_died
  teacher believes teacher_pushed
  teacher believes teacher_left
  exit all

# 세 시 조금 전: 조리 실무사가 뒷문으로 들어오는 담임을 본다
event back_door "급식실 뒷문"
  when after @principal.died by 3m..8m
  enter teacher, cook
  cook sees teacher_returned
  teacher does wash
  teacher believes not window_snow
  exit all

# 오후 세 시: 종은 울리지 않는다. 하람은 썰매 언덕에서 끝까지 기다린다.
# 뒷방에서 물을 틀어 놓고 있던 담임은 주무관이 대신 쳤으리라 여긴다(bell_substitute)
event three_oclock "오후 세 시"
  when after s3.end by 25m..25m
  enter haram
  haram does wait
  haram sees bell_silent
  attr bell.rung = "아니오"
  attr rope.frost = "손자국 없이 얼어붙음"
  teacher believes bell_rang
  exit all
```
