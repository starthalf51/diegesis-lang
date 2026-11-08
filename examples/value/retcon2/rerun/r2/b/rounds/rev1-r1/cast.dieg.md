---
module: yeoul.cast
mode: authored
---

# 여울섬 — 등장인물과 선언

```sl
work yeoul "여울섬"
  unit day
  name ko "여울섬"

character seoyun "Han Seoyun" major
  name ko "한서윤"
  attr seoyun.age = 29
  believes haejin_sank
  believes father_died
  believes seoyun_came_home
  doubts official_cause
character malsun "Oh Malsun" major
  name ko "오말순"
  attr malsun.age = 78
  believes tide_song
character minjae "Kang Minjae" major
  name ko "강민재"
  attr minjae.age = 29
  believes malsun_dementia
  believes vote_scheduled
  believes clinic_close
  believes minjae_leaves
  believes taeo_arrives
character taeo "Yoon Taeo" major
  name ko "윤태오"
  attr taeo.age = 30
  believes malsun_dementia
  believes vote_scheduled
  believes clinic_close
  believes taeo_arrives
character gyeongtae "Do Gyeongtae" major
  name ko "도경태"
  attr gyeongtae.age = 63
  believes vote_scheduled
  believes demolition
  believes haejin_sank
  believes light_out
  believes yeongho_refused
  believes lens_smashed
  believes not official_cause
  believes not hospital_alibi
character haeun "Do Haeun" major
  name ko "도하은"
  attr haeun.age = 31
character yeongho "Baek Yeongho" major
  name ko "백영호"
  attr yeongho.age = 70
  believes light_out
  believes yeongho_refused
  believes lens_smashed
  believes ledger_record
character jihwan "Ryu Jihwan" minor
  name ko "류지환"
  attr jihwan.age = 35
  believes vote_scheduled
  believes demolition
  believes jihwan_stays
character gicheol "Moon Gicheol" minor
  name ko "문기철"
  attr gicheol.age = 58
  believes haejin_sank
character sua "Jeong Sua" minor
  name ko "정수아"
  attr sua.age = 13
  believes sua_collects

parent seoyun of malsun

place dock "the dock"
  name ko "여울섬 선착장"
place malsun_house "Malsun's house"
  name ko "오말순의 집"
place lighthouse "the lighthouse"
  name ko "여울섬 등대"
place ruin "the abandoned house"
  name ko "섬 끝 폐가"
place clinic "the health clinic"
  name ko "보건지소"
place school "the branch school"
  name ko "여울분교"
place minbak "Gicheol's guesthouse"
  name ko "문기철의 민박집"

thing vote_notice "the vote notice"
  name ko "주민 투표 공고문"
  attr vote_notice.at = dock
thing tin_box "a rusty tin box"
  name ko "녹슨 양철 상자"
  attr tin_box.at = malsun_house
thing ledger_page "the torn ledger page"
  name ko "찢긴 등대 근무 장부 쪽"
thing glass_shard "a thick glass shard"
  name ko "두꺼운 유리 조각"
thing stair_nail "the new nail"
  name ko "열세 번째 계단의 새 못"
  attr stair_nail.at = lighthouse

fact vote_scheduled "열이틀 뒤 섬 전체를 리조트 개발사에 팔지 정하는 주민 투표가 열린다" true
fact demolition "투표가 가결되면 등대와 폐가를 포함한 섬 건물은 곧바로 철거된다" true
fact clinic_close "투표가 가결되면 보건지소도 문을 닫는다" true
fact jihwan_stays "개발사 현장 책임자 류지환은 투표가 끝날 때까지 섬에 머문다" true
fact haejin_sank "삼십 년 전 여객선 해진호가 여울섬 앞바다에서 침몰해 열한 명이 죽었다" true
fact father_died "서윤의 아버지는 해진호 사고로 죽었다" true
fact seoyun_came_home "서윤은 치매 초기인 외할머니를 돌보려고 서울에서 섬으로 돌아왔다" true
fact malsun_dementia "오말순은 치매 초기라 옛일과 오늘 일을 자주 뒤섞는다" true
fact sua_collects "정수아는 바닷가에서 물건을 주워 모은다" true
fact tide_song "옛 노래에 '열한 물때에 불이 잔다'라는 구절이 있다" true
fact official_cause "해진호 사고는 기상 악화로 인한 단순 사고였다" false
fact hospital_alibi "사고 날 밤 도경태는 육지 병원에 입원해 있었다" false
fact light_out "해진호가 침몰한 밤 등대 불은 두 시간 동안 꺼져 있었다" true
fact yeongho_refused "그 밤 도경태는 백영호에게 등대 불을 끄라고 시켰지만 백영호는 끝내 거절했다" true
fact lens_smashed "백영호가 거절하자 도경태가 직접 등실에 올라가 렌즈를 깨뜨려 등대 불을 꺼뜨렸고 그때 오른손을 깊게 베었다" true
fact ledger_record "등대 불이 꺼진 일은 근무 장부에 적혔고 그 쪽은 찢겨 사라졌다" true
fact malsun_keeps "오말순은 찢긴 장부 쪽을 삼십 년 동안 보관해 왔다" true
fact minjae_leaves "강민재는 근무 기간이 끝나 오늘 마지막 배로 섬을 떠나 육지로 돌아간다" true
fact taeo_arrives "강민재 대신 윤태오가 보건지소에 새로 부임해 서윤을 돕게 된다" true
```
