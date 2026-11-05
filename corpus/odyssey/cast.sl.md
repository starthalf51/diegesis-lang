---
module: odyssey.cast
mode: authored
---

# 오디세이 — 등장인물과 선언

원문: `texts/butler-odyssey.txt` (Samuel Butler 역 『The Odyssey』 1900, Project Gutenberg #1727,
https://www.gutenberg.org/cache/epub/1727/pg1727.txt). 13·14·16·19·21·22·23권을 그대로 옮겼고, `src` 는
`<권>.<단락>` 이다(권 제목·요지 줄 다음의 첫 단락이 1). 영어 이름은 Butler 의 표기(Ulysses, Minerva, Euryclea)다.
ID 는 그리스식(odysseus, athena, eurycleia)으로 둔다.

포팅 범위: 정체 알아보기의 줄기만. 13권(이타카 도착, 아테나의 변장), 14권(에우마이오스의 오두막, 크레타 이야기),
16권(텔레마코스에게 밝힘), 19권(페넬로페와의 대화, 에우리클레이아의 흉터), 21권(에우마이오스·필로이티오스에게
밝힘), 22권(구혼자들에게 밝힘), 23권(침대 시험). 15·17·18·20권과 활 시험의 세부, 구혼자 살해의 세부는 뺐다.

```sl
work odyssey "The Odyssey — the recognitions"
  unit day
  name ko "오디세이아 — 알아보기"
  name en "The Odyssey — the recognitions"

character odysseus "Odysseus", "Ulysses" major
  name ko "오디세우스"
  name en "Ulysses"
  believes bed_secret          # 23.18 "a marvellous curiosity which I made with my very own hands"
character athena "Athena", "Minerva" major
  name ko "아테나"
  name en "Minerva"
  knows omniscient             # 13.26 "who have been ever with you, who kept watch over you"
character penelope "Penelope" major
  name ko "페넬로페"
  name en "Penelope"
  believes bed_secret          # 23.19 "our bed (which no human being has ever seen but you and I and a single maidservant"
character telemachus "Telemachus" major
  name ko "텔레마코스"
  name en "Telemachus"
character eumaeus "Eumaeus" major
  name ko "에우마이오스"
  name en "Eumaeus"
character eurycleia "Eurycleia", "Euryclea" major
  name ko "에우리클레이아"
  name en "Euryclea"
character philoetius "Philoetius" minor
  name ko "필로이티오스"
  name en "Philoetius"
character antinous "Antinous" minor
  name ko "안티노오스"
  name en "Antinous"
character eurymachus "Eurymachus" minor
  name ko "에우리마코스"
  name en "Eurymachus"
group suitors "the suitors"
  name ko "구혼자들"
  name en "the suitors"
  members antinous, eurymachus

place phorcys_haven "the haven of Phorcys"
  name ko "포르키스 포구"
  name en "the haven of Phorcys"
place eumaeus_hut "the swineherd's hut"
  name ko "돼지치기의 오두막"
  name en "the swineherd's hut"
place cloister "the cloister of Ulysses' house"
  name ko "오디세우스 집의 회랑"
  name en "the cloister"
place outer_yard "outside the gates"
  name ko "바깥뜰"
  name en "outside the gates and the outer yard"
place upper_room "Penelope's upper room"
  name ko "페넬로페의 윗방"
  name en "the upper room"

fact beggar_is_odysseus "the old beggar is Ulysses" true
fact odysseus_home "Ulysses has come back to Ithaca" true
fact stranger_is_cretan "the stranger is a Cretan (his lying tale)" false
fact bed_secret "the bed is built on the stump of a living olive and cannot be moved" true
```
