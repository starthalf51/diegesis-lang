---
module: bungyo.cast
mode: authored
---

# 종이 울리지 않은 날 — 등장인물과 선언

폭설로 고갯길이 끊긴 백운초등학교 눈골분교. 이틀 동안의 이야기다.
첫날 오후 두 시 오십이 분에 교장이 죽는다. 담임은 "그 시각 서진이와 함께 있었다"고 말한다.
오십 년 동안 오후 세 시를 거른 적 없던 종은 그날 울리지 않았다.

```sl
work bungyo "종이 울리지 않은 날"
  unit day
  name ko "종이 울리지 않은 날"

character seojin "윤서진" major
  name ko "윤서진"
  attr seojin.age = 11
  attr seojin.grade = "5학년"
  attr seojin.home = "분교 아랫마을, 할머니와 둘이 산다"
character haram "이하람" minor
  name ko "이하람"
  attr haram.age = 11
  attr haram.grade = "5학년"
  attr haram.relation = "서진의 단짝, 분교에 단 둘인 5학년"
character teacher "정혜원" major
  name ko "정혜원"
  attr teacher.age = 34
  attr teacher.relation = "서진과 하람의 담임. 눈골분교 졸업생이자 박성규 교장의 옛 제자"
  believes bell_daily
  believes bell_substitute
character principal "박성규" major
  name ko "박성규"
  attr principal.age = 61
  attr principal.relation = "눈골분교 교장. 열한 살 때 이 분교의 종 당번이었다"
  believes bell_daily
  believes bell_substitute
  believes closure_plan
character custodian "김덕수" minor
  name ko "김덕수"
  attr custodian.age = 58
  attr custodian.relation = "분교 주무관. 장작과 제설과 종탑을 맡는다"
  believes bell_daily
  believes bell_substitute
character cook "최말순" minor
  name ko "최말순"
  attr cook.age = 63
  attr cook.relation = "분교 조리 실무사. 급식실 뒷문 옆에서 일한다"
  believes bell_daily
character officer "오태식" minor
  name ko "오태식"
  attr officer.age = 46
  attr officer.relation = "읍내 파출소 순경"

place yard "눈골분교 운동장"
  name ko "눈골분교 운동장"
place hallway "급식실 앞 복도"
  name ko "급식실 앞 복도"
place infirmary "보건실"
  name ko "보건실"
place tower "종탑"
  name ko "종탑"
place staffroom "교무실"
  name ko "교무실"
place sled_hill "뒷동산 썰매 언덕"
  name ko "뒷동산 썰매 언덕"
place gate "정문 아래 비탈길"
  name ko "정문 아래 비탈길"

thing bell "분교 종"
  name ko "분교 종"
  attr bell.material = "구리"
  attr bell.hung_at = tower
thing rope "종줄"
  name ko "종줄"
  attr rope.hung_at = tower
thing watch "교장의 회중시계"
  name ko "교장의 회중시계"
  attr watch.owner = principal
thing mitten "빨간 벙어리장갑"
  name ko "빨간 벙어리장갑"
  attr mitten.maker = "서진과 할머니"

artifact closure_form "폐교 동의서"
  name ko "폐교 동의서"
  contains closure_plan
  "눈골분교의 폐교에 동의함. 교장 박성규"

role headmaster "교장"
  name ko "눈골분교 교장"
  held principal from start until @principal.died
role homeroom "담임"
  name ko "5학년 담임"
  held teacher from start

fact road_cut "폭설로 읍내로 내려가는 고갯길이 끊겼다" true
fact bell_daily "분교의 종은 오십 년 동안 오후 세 시를 거른 날이 없었다. 박성규 교장이 열한 살 종 당번 때부터 쳐 온 종이다" true
fact bell_substitute "교장이 자리에 없는 날에는 김덕수 주무관이 대신 세 시 종을 친다" true
fact firewood_quarrel "그날 아침 김덕수 주무관과 박성규 교장이 장작 문제로 언성을 높였다" true
fact closure_plan "박성규 교장은 눈이 그치면 분교 폐교 동의서를 교육청에 보낼 작정이었다" true
fact seojin_fever "서진은 점심 무렵부터 열이 올랐다" true
fact principal_to_tower "박성규 교장은 세 시 종을 치기 전에 종탑 계단의 눈을 쓸려고 일찍 나갔다" true
fact teacher_left "정혜원 선생님은 서진이 잠든 사이 보건실을 비우고 종탑에 다녀왔다" true
fact teacher_alibi "정혜원 선생님은 두 시 반부터 줄곧 보건실에서 서진 곁을 떠나지 않았다" = not teacher_left
fact teacher_pushed "정혜원 선생님이 종탑 계단에서 박성규 교장을 밀쳤다" true
fact principal_died "박성규 교장이 종탑 계단 아래에서 숨졌다" true
fact teacher_returned "세 시 조금 전, 정혜원 선생님이 눈을 뒤집어쓴 채 급식실 뒷문으로 급히 들어왔다" true
fact bell_silent "그날 오후 세 시, 분교의 종은 울리지 않았다" true
fact bell_rang "그날 오후 세 시, 분교의 종이 울렸다" = not bell_silent
fact teacher_wet_hair "정혜원 선생님의 머리카락 끝이 녹은 눈에 젖어 있었다" true
fact window_snow "정혜원 선생님이 보건실 창문을 열었다가 들이친 눈에 젖었다" false
fact mitten_missing "정혜원 선생님의 손에 서진이 준 빨간 장갑이 없었다" true
fact custodian_at_gate "김덕수 주무관은 두 시 반 무렵부터 한 시간 남짓 정문 아래 비탈길의 눈을 치웠다" true
fact custodian_no_ring "김덕수 주무관은 그날 종탑 근처에 가지 않았고 종을 치지 않았다" true
fact custodian_suspect "김덕수 주무관이 박성규 교장을 해쳤다" false
fact watch_stopped "박성규 교장의 회중시계는 유리가 깨진 채 두 시 오십이 분에 멈춰 있었다" true
fact rope_frozen "종줄에는 서리가 손자국 하나 없이 하얗게 얼어붙어 있었다" true
fact mitten_at_tower "서진이 떠 준 빨간 벙어리장갑 한 짝이 종탑 계단 아래 눈 속에 묻혀 있었다" true
fact teacher_was_at_tower "정혜원 선생님은 그날 오후 종탑 계단에 왔었다" true
```
