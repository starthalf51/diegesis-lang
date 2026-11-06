---
module: macbeth.cast
mode: authored
---

# 맥베스 — 등장인물과 선언

원문: `texts/macbeth-mit.txt` (MIT Shakespeare, http://shakespeare.mit.edu/macbeth/full.html).
`src` 는 그 판의 막.장.줄 번호다. 포팅 범위: 1막 전체, 2막 1·2장, 4.1 의 왕들의 행렬.

```sl
work macbeth "The Tragedy of Macbeth"
  unit day
  name ko "맥베스"
  name en "The Tragedy of Macbeth"

character macbeth "Macbeth" major
  name ko "맥베스"
  name en "Macbeth"
  believes holds(thane_of_glamis, macbeth)   # 1.3.73 "By Sinel's death I know I am thane of Glamis"
  believes cawdor_in_favour                  # 극 이전 상식: 코더 영주는 번창하는 충신이다(1.3.74-75)
  believes alive(cawdor)                     # 극 이전 상식: 1.3.74 "the thane of Cawdor lives"(1.3.113 에서도 되풀이)
character lady_macbeth "Lady Macbeth" major
  name ko "맥베스 부인"
  name en "Lady Macbeth"
  believes holds(thane_of_glamis, macbeth)   # 1.5.15 "Glamis thou art" — 남편의 작위는 극 이전부터 안다
character banquo "Banquo" major
  name ko "뱅쿼"
  name en "Banquo"
character fleance "Fleance" minor
  name ko "플리언스"
  name en "Fleance"
character duncan "Duncan" major
  name ko "덩컨"
  name en "Duncan"
character malcolm "Malcolm" minor
  name ko "맬컴"
  name en "Malcolm"
character donalbain "Donalbain" extra
  name ko "도널베인"
  name en "Donalbain"
character ross "Ross" minor
  name ko "로스"
  name en "Ross"
character angus "Angus" minor
  name ko "앵거스"
  name en "Angus"
  believes alive(cawdor)                     # 1.3.115 "Who was the thane lives yet": 1.2 에서 떠날 때 코더는 살아 있었다
character lennox "Lennox" extra
  name ko "레녹스"
  name en "Lennox"
character macduff "Macduff" minor
  name ko "맥더프"
  name en "Macduff"
character captain "a bleeding Sergeant" extra
  name ko "피 흘리는 하사"
  name en "a bleeding Sergeant"
character cawdor "the elder Thane of Cawdor" minor   # 무대에 한 번도 안 나온다
  name ko "전(前) 코더 영주"
  name en "the Thane of Cawdor"
character witness "one that saw him die" extra       # 1.4.5 맬컴이 이야기를 들은 처형 목격자
  name ko "처형을 본 사람"
  name en "one that saw him die"
character messenger "a Messenger" extra
  name ko "전령"
  name en "a Messenger"
character fellow "one of my fellows" extra           # 1.5.37 맥베스를 앞질러 달려온 하인
  name ko "먼저 달려온 하인"
  name en "one of my fellows"
character witch1 "First Witch" extra
  name ko "첫째 마녀"
  name en "First Witch"
character witch2 "Second Witch" extra
  name ko "둘째 마녀"
  name en "Second Witch"
character witch3 "Third Witch" extra
  name ko "셋째 마녀"
  name en "Third Witch"
group witches "the Weird Sisters" major
  name ko "운명의 세 자매"
  name en "the Weird Sisters"
  members witch1, witch2, witch3
  knows oracle                                        # 정보 흐름 검사 면제. 출처는 기록에 남는다

place desert "a desert place"
  name ko "황야"
  name en "a desert place"
place camp "a camp near Forres"
  name ko "포레스 근처의 진영"
  name en "a camp near Forres"
place fife "the field at Fife"
  name ko "파이프 들판"
  name en "Fife"
place heath "a heath near Forres"
  name ko "포레스 근처의 황무지"
  name en "a heath near Forres"
place forres "Forres. The palace"
  name ko "포레스 궁"
  name en "the palace at Forres"
place inverness "Macbeth's castle, Inverness"
  name ko "인버네스, 맥베스의 성"
  name en "Macbeth's castle at Inverness"
place cavern "a cavern, the witches' cauldron"
  name ko "마녀들의 동굴"
  name en "a cavern"
distance camp heath 0d..1d
distance fife camp 0d..1d

role thane_of_glamis "Thane of Glamis", "Glamis"
  held macbeth from start
role thane_of_cawdor "Thane of Cawdor", "Cawdor"
  held cawdor until @1.2.decree
  held macbeth from @1.2.decree                       # 법적으로는 1.2부터 맥베스의 것이다. 본인은 1.3에서 안다
role king "the King", "Scotland"
  held duncan from start until @duncan.died
  # 원문: 2.4.38-39 맥더프 "He is already named, and gone to Scone / To be invested", 3.1.1 뱅쿼 "Thou hast it now: king".
  # 포팅한 장면 중 맥베스가 왕인 것은 4.1 이다: 4.1.109-110 "our high-placed Macbeth / Shall live the lease of nature",
  # 4.1.113-114 "shall Banquo's issue ever / Reign in this kingdom?". 지명(2.4)은 포팅 범위 밖이라 보유는 4.1 부터 적는다
  # (그 앞의 시점을 지어내지 않는다 — 2.2 끝과 4.1 사이의 보유는 이 포팅이 말하지 않는다. 수정 1회차)
  held macbeth from 4.1
role prince_of_cumberland "the Prince of Cumberland"
  held malcolm from @1.4.cumberland

fact cawdor_traitor "Cawdor aided Norway against Duncan" true
fact cawdor_in_favour "the Thane of Cawdor is a prosperous gentleman, no traitor" = not cawdor_traitor   # "the thane of Cawdor lives, / A prosperous gentleman"
fact king_hereafter = holds(king, macbeth)
fact banquo_line "Banquo's descendants shall be kings" true
fact macdonwald_slain "Macbeth killed the rebel Macdonwald" true
fact duncan_comes "Duncan lodges at Inverness tonight" true
# 살해의 뜻(독자 모델·극적 아이러니, Task 7). 1.3.148 맥베스 방백 "My thought, whose murder yet is but fantastical",
# 1.4.57-58 "Stars, hide your fires; / Let not light see my black and deep desires", 1.5.43 부인 "the fatal entrance of Duncan",
# 1.7.77-79 계획. 참이다: 2.2.18 "I have done the deed".
fact murder_plot "the Macbeths have Duncan's murder in mind" true
# 1.6.17-23 부인의 환대 "All our service / In every point twice done and then done double … We rest your hermits" —
# 1.5.72-74 "look like the innocent flower, / But be the serpent under't" 의 실행. 충성만 있다는 거짓 겉모습이다
fact hosts_loyal "Macbeth's house means Duncan nothing but loyal service" = not murder_plot

artifact macbeth_letter "Macbeth's letter"
  contains holds(thane_of_cawdor, macbeth), king_hereafter
  "They met me in the day of success"
```
