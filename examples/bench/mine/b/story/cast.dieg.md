---
module: gangdo.cast
mode: authored
---

# 일흔두 시간 — 등장인물과 선언

1987년 12월, 이듬해 봄 문을 닫을 광업소. 동편 3갱도가 무너지고 구조대가 길을 뚫기까지 일흔두 시간이 걸린다.

```sl
work gangdo "일흔두 시간"
  unit day
  start 1987-12-14
  name ko "일흔두 시간"

character suah "Kang Suah" major
  name ko "강수아"
  attr suah.age = 14
  believes mine_closing
  believes props_rotten
character chulsu "Kang Chulsu" major
  name ko "강철수"
  attr chulsu.age = 41
  believes mine_closing
  believes props_rotten
  believes inspection_skipped
  believes yeongho_offbook
  believes yeongho_with_chulsu
  believes vent_air
character hwang "Hwang Manseok" major
  name ko "황만석"
  attr hwang.age = 56
  believes mine_closing
  believes inspection_skipped
  believes yeongho_offbook
  believes yeongho_with_chulsu
  believes yeongho_months
  believes closure_money
  believes forge_plan
character migyeong "Seo Migyeong" major
  name ko "서미경"
  attr migyeong.age = 29
  believes mine_closing
  believes yeongho_offbook
  believes yeongho_months
character park "Park Taesik" major
  name ko "박태식"
  attr park.age = 45
character yeongho "Min Yeongho" major
  name ko "민영호"
  attr yeongho.age = 17
  believes yeongho_offbook
  believes yeongho_with_chulsu
  believes no_tag
character mangil "Oh Mangil" minor
  name ko "오만길"
  attr mangil.age = 52
character dongsu "Choi Dongsu" minor
  name ko "최동수"
  attr dongsu.age = 33

group villagers "the villagers" extra
  name ko "마을 사람들"
  believes mine_closing
group rescuers "the rescue team" extra
  name ko "구조대원들"

role director "the mine director"
  name ko "광업소장"
  held hwang from start

place suah_house "Suah's company house"
  name ko "수아네 사택"
place pithead "the pithead yard"
  name ko "갱구 앞 광장"
place director_office "the director's office"
  name ko "광업소장실"
place east_face "the east No.3 face"
  name ko "동편 3갱도 막장"
place migyeong_room "Migyeong's lodging room"
  name ko "미경의 하숙방"

thing lunchbox "tin lunchbox"
  name ko "양은 도시락"
thing tag_board "the name-tag board"
  name ko "명패판"
  attr tag_board.at = pithead
thing stove "iron stove"
  name ko "무쇠 난로"
  attr stove.at = director_office
thing harmonica "Yeongho's harmonica"
  name ko "영호의 하모니카"
thing envelope "Yeongho's wage envelopes"
  name ko "영호의 일당 봉투"

fact mine_closing "광업소는 내년 봄에 문을 닫는다" true
fact props_rotten "동편 3갱도 버팀목이 썩어 가고 있었다" true
fact cavein "동편 3갱도 천장이 무너져 갱구 쪽 길이 막혔다" true
fact inspection_done "동편 3갱도는 지난주에 안전 점검을 마쳤다" false
fact inspection_skipped "동편 3갱도의 안전 점검은 하지 않은 채 미뤄졌다" true
fact only_three "갇힌 사람은 강철수, 오만길, 최동수 세 사람뿐이다" false
fact four_trapped "갱도 안에는 장부에 없는 민영호까지 네 사람이 갇혀 있다" true
fact yeongho_offbook "민영호는 소장이 장부에 올리지 않고 현금 일당으로 쓰는 열일곱 살 인부다" true
fact yeongho_with_chulsu "민영호가 강철수를 따라 동편 3갱도에 들어갔다" true
fact yeongho_months "민영호는 석 달째 장부 없이 동편 막장에 들어갔다" true
fact yeongho_in_town "민영호는 그날 읍내에 나가 갱에 들어가지 않았다" false
fact no_tag "민영호에게는 명패판에 걸 명패가 없다" true
fact rescue_72h "구조대가 무너진 길을 뚫으려면 일흔두 시간이 걸린다" true
fact ledger_sent "점검 장부는 이미 본사에 올려보냈다" false
fact hwang_took_ledger "소장이 사고 난 밤 갱구 사무소에서 점검 장부를 몰래 가져왔다" true
fact hwang_home "소장은 사고 난 밤 내내 사택에서 나오지 않았다" false
fact forge_plan "소장은 장부를 말려 지난주 날짜로 가짜 점검 기록을 써넣으려 한다" true
fact closure_money "폐광 보상금은 사고 기록이 없어야 제대로 나온다" true
fact job_real "소장이 미경을 서울 본사 경리로 넣어 줄 수 있다" false
fact air_short "막장 안 공기가 갈수록 탁해진다" true
fact vent_air "옆 갱도 환기구 쪽에는 아직 바깥 공기가 조금 들어온다" true
fact yeongho_side "민영호가 공기가 남은 옆 갱도 환기구 쪽으로 옮겨 갔다" true
fact note_left "강철수가 수아의 도시락 밥 밑에 쪽지를 넣어 두었다" true

artifact note "the lunchbox note"
  name ko "도시락 쪽지"
  contains yeongho_with_chulsu, inspection_skipped
  "수아야, 오늘은 영호 데리고 동편 3갱도 들어간다. 점검 끝나면 소장님이 수당 준다니 고기 사 오마. 밥 다 먹어라."
artifact ledger "the east-side inspection ledger"
  name ko "동편 점검 장부"
  contains inspection_skipped, yeongho_offbook
  "12월 7일 동편 3갱도 점검 — (빈칸)"
  "현금 일당: 민영호(17)"
```
