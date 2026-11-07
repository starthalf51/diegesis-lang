---
module: yeoulseom.cast
mode: authored
---

# 여울섬 — 등장인물과 선언

열이틀 뒤 섬 매각 주민 투표가 열린다. 삼십 년 전 해진호 침몰의 진실은 등대와 한 노인의 기억 속에 묻혀 있다.

```sl
work yeoulseom "여울섬"
  unit day
  name ko "여울섬"

character seoyun "Han Seoyun" major
  name ko "한서윤"
  believes father_died
  doubts official_cause
character malsun "Oh Malsun" major
  name ko "오말순"
  believes song_lyric
  believes malsun_keeps_page
character minjae "Kang Minjae" major
  name ko "강민재"
  believes malsun_dementia
  believes vote_in_12
  believes official_cause
  believes father_died
character gyeongtae "Do Gyeongtae" major
  name ko "도경태"
  believes vote_in_12
  believes gyeongtae_on_island
  believes ordered_off
  believes lamp_off
character haeun "Do Haeun" major
  name ko "도하은"
  believes vote_in_12
character youngho "Baek Youngho" major
  name ko "백영호"
  believes lamp_off
  believes ordered_off
  believes page_torn
character jihwan "Ryu Jihwan" minor
  name ko "류지환"
  believes vote_in_12
  believes demolition
character gicheol "Moon Gicheol" minor
  name ko "문기철"
  believes father_died
character sua "Jeong Sua" minor
  name ko "정수아"

place island "Yeoul Island"
  name ko "여울섬"
place malsun_house "Malsun's house"
  name ko "오말순의 집"
place village_hall "village hall"
  name ko "마을회관"
place health_post "health post"
  name ko "보건지소"
place lighthouse "lighthouse"
  name ko "여울섬 등대"
place ruin_house "abandoned house"
  name ko "섬 끝 폐가"
place minbak "Gicheol's guesthouse"
  name ko "문기철의 민박집"
place school "branch school"
  name ko "여울섬 분교"
place harbor "harbor"
  name ko "여울섬 선착장"

thing logbook_page "torn logbook page"
  name ko "등대 근무 장부의 찢긴 쪽"
thing new_nail "new nail on the thirteenth step"
  name ko "등대 계단 열세 번째 칸의 새 못"
thing glass_shard "thick glass shard"
  name ko "두꺼운 유리 조각"
thing haejin "ferry Haejin"
  name ko "해진호"

role village_head "village head"
  held gyeongtae from start

fact father_died "서윤의 아버지는 삼십 년 전 해진호 사고로 죽었다" true
fact official_cause "해진호 사고는 기상 악화로 인한 단순 사고였다" false
fact vote_in_12 "섬 전체를 리조트 개발사에 팔지 정하는 주민 투표가 열이틀 뒤에 열린다" true
fact demolition "투표가 가결되면 등대와 폐가를 포함한 섬 건물은 바로 철거된다" true
fact malsun_dementia "외할머니 오말순은 치매 초기다" true
fact song_lyric "외할머니가 흥얼거리는 옛 노래에 '열한 물때에 불이 잔다'는 구절이 있다" true
fact lamp_off "해진호가 침몰한 밤 등대 불은 두 시간 동안 꺼져 있었다" true
fact ordered_off "백영호가 도경태의 지시로 등대 불을 껐다" true
fact page_torn "그 사실이 적힌 등대 근무 장부의 쪽은 찢겨 사라졌다" true
fact malsun_keeps_page "오말순은 찢긴 장부 쪽을 삼십 년 동안 보관해 왔다" true
fact hospital_alibi "사고 날 밤 도경태는 육지 병원에 입원해 있었다" false
fact gyeongtae_on_island "사고 날 밤 도경태는 섬에 있었다" true
```

```sl
attr seoyun.age = 29
attr malsun.age = 78
attr minjae.age = 29
attr gyeongtae.age = 63
attr haeun.age = 31
attr youngho.age = 70
attr jihwan.age = 35
attr gicheol.age = 58
attr sua.age = 13
attr island.vote_day = 12
attr haejin.dead = 11
```
