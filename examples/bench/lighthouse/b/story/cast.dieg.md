---
module: beacon.cast
mode: authored
---

# 등대 일지 — 등장인물과 선언

태풍으로 뱃길이 끊긴 사흘 동안 외딴섬 등대에서 벌어지는 이야기다.
삼촌의 거짓말("아홉 시 순찰 중 추락")은 은솔과 서미경 선생만 믿는다. 방순례 할머니는 그 밤 불빛을 보았기에 처음부터 믿지 않는다.
진실: 아버지는 18시 20분에 일지 마지막 쪽을 쓰고, 18시 40분 전망대에서 삼촌에게 밀려 떨어졌다. 그 밤 불은 한 번도 돌지 않았다.

```sl
work beacon "등대 일지"
  name ko "등대 일지"

character eunsol "한은솔" major
  name ko "한은솔"
  attr eunsol.age = 14
  attr eunsol.relation = "등대지기 한정우의 외동딸"
  believes father_logs_daily
  believes patrol_flashlight
character jeongwoo "한정우" major
  name ko "한정우"
  attr jeongwoo.age = 44
  attr jeongwoo.relation = "등대지기, 은솔의 아버지"
  believes boxes_hidden
  believes boxes_found
character jeongseok "한정석" major
  name ko "한정석"
  attr jeongseok.age = 38
  attr jeongseok.relation = "정우의 동생, 은솔의 삼촌, 등대 보조원"
  believes road_cut
  believes smuggling
  believes boxes_hidden
  believes dark_signal
  believes motor_works
character sunrye "방순례" minor
  name ko "방순례"
  attr sunrye.age = 71
  attr sunrye.relation = "등대 맞은편 언덕에 사는 해녀 출신 할머니"
  believes father_never_missed
character mikyung "서미경" minor
  name ko "서미경"
  attr mikyung.age = 29
  attr mikyung.relation = "섬 분교 선생님, 은솔의 담임"
character jaehee "윤재희" minor
  name ko "윤재희"
  attr jaehee.age = 35
  attr jaehee.relation = "해양경찰 경사"
character gwangho "박광호" minor
  name ko "박광호"
  attr gwangho.age = 52
  attr gwangho.relation = "어선 선장, 삼촌의 밀수 동업자"
  believes smuggling
  believes boxes_hidden
  believes dark_signal

parent jeongwoo of eunsol

place keeper_house "관사"
  name ko "등대 관사"
place lamp_room "등롱실"
  name ko "등롱실"
place gallery "전망대 난간"
  name ko "전망대 난간"
place storeroom "기름 창고"
  name ko "기름 창고"
place sunrye_house "순례 할머니 집"
  name ko "순례 할머니 집"
place pier "선착장"
  name ko "선착장"

thing flashlight "아버지의 손전등"
  name ko "아버지의 손전등"
  attr flashlight.at = keeper_house
thing motor "회전 모터"
  name ko "등명기 회전 모터"
  attr motor.at = lamp_room
thing boxes "상자들"
  name ko "기름통 뒤의 상자들"
  attr boxes.at = storeroom
thing radio "무전기"
  name ko "관사 무전기"
  attr radio.at = keeper_house

fact road_cut "태풍으로 뱃길이 사흘 동안 끊겼다" true
fact father_lost "아버지는 태풍 첫날 밤 바다에 떨어져 돌아오지 않았다" true
fact fell_at_nine "아버지는 밤 아홉 시 순찰 중에 바다에 떨어졌다" false
fact pushed "삼촌이 그날 저녁 여섯 시 사십 분, 전망대 난간에서 아버지를 밀었다" true
fact light_dark "그날 밤 등대 불빛은 한 번도 돌지 않았다" true
fact light_turned "사흘째 밤, 등대 불빛이 다시 돌았다" true
fact father_never_missed "정우는 이십 년 동안 불을 거른 밤이 하루도 없었다" true
fact father_logs_daily "아버지는 하루도 빠짐없이 등대 일지를 적었다" true
fact flashlight_left "아버지의 손전등이 관사 못에 그대로 걸려 있다" true
fact patrol_flashlight "아버지는 아홉 시 순찰 때마다 그 손전등을 들고 나갔다" true
fact spare_flashlight "아버지는 그날 밤 예비 손전등을 들고 나갔다" false
fact motor_broken "태풍에 등대 회전 모터가 고장 났다" false
fact motor_works "등대 회전 모터는 멀쩡히 돈다" true
fact uncle_alibi "삼촌은 그날 저녁 내내 관사에서 밥을 짓고 있었다" false
fact radio_broken "관사 무전기가 태풍에 고장 났다" false
fact radio_works "관사 무전기는 퓨즈만 빠져 있을 뿐 멀쩡하다" true
fact last_page "일지의 마지막 기록은 그날 18시 20분이고, 점등과 아홉 시 순찰 기록이 없다" true
fact boxes_found "아버지는 기름 창고에서 낯선 상자를 찾아내 삼촌에게 따지려 했다" true
fact boxes_hidden "기름 창고 기름통 뒤에 밀수품 상자가 숨겨져 있다" true
fact smuggling "삼촌은 박광호 선장과 밀수를 한다" true
fact dark_signal "등대 불을 꺼 두는 것은 박 선장의 배에 보내는 신호였다" true
fact gwangho_caught "박광호의 배가 바람이 잦아든 새벽 항구를 나서다 해경에 붙잡혔다" true

artifact logbook "등대 일지"
  name ko "등대 일지"
  contains boxes_found
  "18:20 기름 창고 기름통 뒤, 처음 보는 상자 여섯. 정석에게 묻겠다."
```
