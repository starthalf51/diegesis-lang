---
module: yeoulseom.cast
mode: authored
---

# 여울섬 — 9화 추가 선언

강민재는 3화에 육지로 떠나 퇴장했고 다시 나오지 않는다. 서윤을 돕는 자리는 분교 교사 윤유진이 맡는다. 할머니의 노래를 받아 적는 일도 유진이 한다.

진상은 선언 파일의 것을 그대로 따른다. 등대 불은 백영호가 술에 취해 잠들며 렌즈를 깨뜨려 꺼졌고, 그 밤 도경태는 섬에 있었다. 할머니 노래의 뒷구절은 불이 꺼진 그 물때에 선창에 서 있던 이장을 기억한다.

```sl
artifact song_notes "transcribed song"
  name ko "유진이 받아 적은 할머니의 노래"
  contains song_lyric, song_verse2
  "열한 물때에 불이 잔다"

fact song_verse2 "외할머니 노래의 뒷구절에 '불 잔 물때에 이장 비옷이 선창에 섰다'는 말이 있다" true
fact eleven_tide_lamp "노래의 '열한 물때'는 해진호가 가라앉은 밤 등대 불이 꺼진 시각이다" true
fact yujin_transcribe "유진은 할머니의 노래를 처음부터 끝까지 받아 적기로 한다" true
fact notes_not_proof "받아 적은 노래만으로는 이장을 몰아세울 증거가 되지 않는다" true
fact visit_tomorrow "서윤은 내일 아침 받아 적은 노래를 들고 이장의 집으로 찾아간다" true

malsun believes song_verse2
malsun believes eleven_tide_lamp
yujin believes yujin_transcribe
yujin believes notes_not_proof
seoyun believes visit_tomorrow
```
