```sl
module macbeth.cast

work macbeth "The Tragedy of Macbeth"
  unit day

character macbeth "Macbeth" major
character lady_macbeth "Lady Macbeth" major
character banquo "Banquo" major
character duncan "Duncan" major
character ross "Ross" minor
character angus "Angus" minor
character malcolm "Malcolm" minor
character lennox "Lennox" extra
character captain "a bleeding Captain" extra
character cawdor "the elder Thane of Cawdor" minor   # 무대에 한 번도 안 나온다
character witch1 "First Witch" extra
character witch2 "Second Witch" extra
character witch3 "Third Witch" extra
group witches "the Weird Sisters" major
  members witch1, witch2, witch3
  knows oracle                                        # 정보 흐름 검사 면제. 출처는 기록에 남는다

place camp "a camp near Forres"
place heath "a heath"
place inverness "Macbeth's castle, Inverness"
place cavern "a cavern, the witches' cauldron"
distance camp heath 0d..1d

role thane_of_glamis "Thane of Glamis", "Glamis"
  held macbeth from start
role thane_of_cawdor "Thane of Cawdor", "Cawdor"
  held cawdor until @1.2.decree
  held macbeth from @1.2.decree                       # 법적으로는 1.2부터 맥베스의 것이다. 본인은 1.3에서 안다
role king "the King", "Scotland"
  held duncan from start

fact cawdor_traitor "Cawdor aided Norway against Duncan" true
fact king_hereafter = holds(king, macbeth)
fact banquo_line "Banquo's descendants shall be kings" true

artifact macbeth_letter "Macbeth's letter"
  contains holds(thane_of_cawdor, macbeth), king_hereafter
  "They met me in the day of success"
```
