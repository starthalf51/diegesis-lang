---
module: othello.cast
mode: authored
---

# 오셀로 — 등장인물과 선언

원문: `texts/othello-mit.txt` (MIT Shakespeare, http://shakespeare.mit.edu/othello/full.html). `src` 는 그 판의 막.장.줄이다.

정답 셋 코퍼스다. 이 작품은 결함이 **있어야** 합격이다: 이중 시간(double time). 짧은 시계로는 키프로스 도착(2.1)부터
데스데모나의 죽음(5.2)까지 이틀 밤이다. 긴 시계의 단서는 그 사이에 여러 날이 지났다고 말한다(3.4.190 비앙카의 "a week").

포팅 범위: 시간 단서를 담은 장면과 손수건·질투의 정보 흐름만 — 1.3(원로원, "You must away to-night"), 2.1(도착),
2.3(그날 밤의 소동), 3.3(유혹, 손수건), 3.4(손수건, 비앙카), 4.1(손수건을 봄, "this night"), 5.2(죽음).
무대 밖 사건: 결혼(1.1-1.2 의 밤), 장군 부부의 대화(3.1.47), 손수건 떨어뜨리기(3.3.360), 카시오의 마지막 방문(3.4.190).

```sl
work othello "Othello, the Moor of Venice"
  unit day
  name ko "오셀로"
  name en "Othello, the Moor of Venice"

character othello "Othello", "the Moor" major
  name ko "오셀로"
  name en "Othello"
character desdemona "Desdemona" major
  name ko "데스데모나"
  name en "Desdemona"
  believes not desdemona_false               # 4.2.94 "No, as I am a Christian": 자신의 결백
character iago "Iago" major
  name ko "이아고"
  name en "Iago"
character emilia "Emilia" major
  name ko "에밀리아"
  name en "Emilia"
  believes not desdemona_false               # 4.2.13 "I durst, my lord, to wager she is honest"
character cassio "Cassio", "Michael Cassio" major
  name ko "카시오"
  name en "Michael Cassio"
character bianca "Bianca" minor
  name ko "비앙카"
  name en "Bianca"
character brabantio "Brabantio" minor
  name ko "브라반쇼"
  name en "Brabantio"
character duke "the Duke of Venice" minor
  name ko "베네치아 공작"
  name en "Duke of Venice"
character senator "First Senator" extra
  name ko "첫째 원로"
  name en "First Senator"
character montano "Montano" minor
  name ko "몬타노"
  name en "Montano"
character lodovico "Lodovico" minor
  name ko "로도비코"
  name en "Lodovico"

place senate "a council-chamber in Venice"
  name ko "베네치아 원로원 회의실"
  name en "a council-chamber"
place quay "a sea-port in Cyprus"
  name ko "키프로스 항구"
  name en "a sea-port in Cyprus"
place castle "the castle in Cyprus"
  name ko "키프로스 성"
  name en "the castle"
place bedchamber "a bedchamber in the castle"
  name ko "성의 침실"
  name en "a bedchamber"

fact married "Othello and Desdemona are married" true
fact cassio_cashiered "Cassio is dismissed from his lieutenancy" true
fact desdemona_false "Desdemona is false to Othello with Cassio" false
fact emilia_found_napkin "Emilia found the handkerchief and gave it to Iago" true
fact cassio_has_napkin "Cassio has Desdemona's handkerchief" true
```
