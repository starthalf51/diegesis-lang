---
module: hamlet.cast
mode: authored
---

# 햄릿 — 등장인물과 선언

원문: `texts/hamlet-mit.txt` (MIT Shakespeare, http://shakespeare.mit.edu/hamlet/full.html). `src` 는 그 판의 막.장.줄이다.

정답 셋 코퍼스다. 이 작품은 결함이 **있어야** 합격이다: 1.2 에서 대학생(비텐베르크 복학)인 햄릿이 5.1 에서
무덤 파는 사람의 말로는 서른 살이다. 1.2 → 5.1 의 이야기 시간은 원문 단서로 몇 달이고 길어야 1년 남짓이다.

포팅 범위: 나이와 시간 단서를 담은 장면과, 그 단서가 기대는 정보 흐름만 — 1.2(궁정, 유령 소식), 1.5(유령의 고발),
3.2(극중극, "twice two months"), 4.6·4.7(바다에서 온 편지, 오필리어의 죽음), 5.1(무덤 파는 사람, 장례).
무대 밖 사건: 선왕의 죽음, 첫 목격(1.1), 호레이쇼에게 털어놓기, 출항·해적·상륙, 오필리어의 익사, 검시.

```sl
work hamlet "Hamlet, Prince of Denmark"
  unit day
  name ko "햄릿"
  name en "Hamlet, Prince of Denmark"

character hamlet "Hamlet" major
  name ko "햄릿"
  name en "Hamlet"
character claudius "Claudius", "the King" major
  name ko "클로디어스"
  name en "King Claudius"
  believes claudius_murdered                   # 3.3.39-41 "O, my offence is rank … A brother's murder": 그가 했다
character gertrude "Gertrude", "the Queen" major
  name ko "거트루드"
  name en "Queen Gertrude"
character ghost "the Ghost" major              # 선왕의 혼령. 선왕(old_hamlet)과 따로 둔다: 죽은 뒤에 말한다
  name ko "유령"
  name en "Ghost"
  believes claudius_murdered                   # 1.5.66 "Upon my secure hour thy uncle stole"
character old_hamlet "King Hamlet" minor       # 무대에 나오지 않는다. 극 이전에 죽었다
  name ko "선왕 햄릿"
  name en "the late King Hamlet"
character horatio "Horatio" major
  name ko "호레이쇼"
  name en "Horatio"
character marcellus "Marcellus" extra
  name ko "마셀러스"
  name en "Marcellus"
character bernardo "Bernardo" extra
  name ko "버나도"
  name en "Bernardo"
character ophelia "Ophelia" major
  name ko "오필리어"
  name en "Ophelia"
character laertes "Laertes" major
  name ko "레어티즈"
  name en "Laertes"
character sailor "First Sailor" extra
  name ko "선원"
  name en "First Sailor"
character messenger "a Messenger" extra
  name ko "전령"
  name en "Messenger"
character clown1 "First Clown", "the gravedigger" minor
  name ko "무덤 파는 사람"
  name en "First Clown"
character clown2 "Second Clown" extra
  name ko "둘째 광대"
  name en "Second Clown"
character crowner "the crowner" extra          # 5.1.4 "the crowner hath sat on her" — 무대에 나오지 않는다
  name ko "검시관"
  name en "the crowner"
character priest "First Priest" extra
  name ko "사제"
  name en "First Priest"

place castle "a room of state in the castle, Elsinore"
  name ko "엘시노어 성의 알현실"
  name en "a room of state in the castle"
place platform "the platform before the castle"
  name ko "성 앞 망대"
  name en "the platform"
place hall "a hall in the castle"
  name ko "성의 홀"
  name en "a hall in the castle"
place churchyard "a churchyard"
  name ko "묘지"
  name en "a churchyard"
place sea "at sea"
  name ko "바다"
  name en "at sea"

fact claudius_murdered "Claudius murdered King Hamlet" true
fact ghost_walks "the dead king's ghost walks on the platform" true
fact king_guilty "the King's guilt shows at the play" true
fact hamlet_returned "Hamlet is back in Denmark" true
fact ophelia_drowned "Ophelia is drowned" true

artifact letter_horatio "Hamlet's letter to Horatio"
  contains hamlet_returned
  "Ere we were two days old at sea, a pirate of very warlike appointment gave us chase."
artifact letter_king "Hamlet's letter to the King"
  contains hamlet_returned
  "High and mighty, You shall know I am set naked on your kingdom."

born hamlet
```
