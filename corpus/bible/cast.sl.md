---
module: bible.cast
mode: authored
---

# 성경 — 등장인물과 선언

원문: `texts/web.txt` (World English Bible Classic, 퍼블릭 도메인, https://ebible.org/eng-web/).
`src` 는 `<책>.<장>.<절>` 이다(gen, matt, mark, luke, john).
포팅 범위: 창세기 5장 전체, 7:6, 9:28-29, 11:10-26(족보), 마태 26:31-35·69-75, 마가 14:27-31·66-72,
누가 22:31-34·54-62, 요한 13:36-38·18:15-27(베드로의 부인).

시간 원점(`start`)은 아담이 지어진 해(Anno Mundi 0)다. 1y = 365d. 족보의 나이는 모두 정확한 값(`Ny..Ny`)이다.

```sl
work bible "The Holy Bible (World English Bible)"
  unit year
  name ko "성경"
  name en "The Holy Bible"

# 창세기 5장: 아담 → 노아
character adam "Adam" major
  name ko "아담"
  name en "Adam"
character seth "Seth" minor
  name ko "셋"
  name en "Seth"
character enosh "Enosh" minor
  name ko "에노스"
  name en "Enosh"
character kenan "Kenan" minor
  name ko "게난"
  name en "Kenan"
character mahalalel "Mahalalel" minor
  name ko "마할랄렐"
  name en "Mahalalel"
character jared "Jared" minor
  name ko "야렛"
  name en "Jared"
character enoch "Enoch" minor
  name ko "에녹"
  name en "Enoch"
character methuselah "Methuselah" minor
  name ko "므두셀라"
  name en "Methuselah"
character lamech "Lamech" minor
  name ko "라멕"
  name en "Lamech"
character noah "Noah" major
  name ko "노아"
  name en "Noah"
character shem "Shem" minor
  name ko "셈"
  name en "Shem"
character ham "Ham" extra
  name ko "함"
  name en "Ham"
character japheth "Japheth" extra
  name ko "야벳"
  name en "Japheth"

# 창세기 11:10-26: 셈 → 아브람
character arpachshad "Arpachshad" minor
  name ko "아르박삿"
  name en "Arpachshad"
character shelah "Shelah" minor
  name ko "셀라"
  name en "Shelah"
character eber "Eber" minor
  name ko "에벨"
  name en "Eber"
character peleg "Peleg" minor
  name ko "벨렉"
  name en "Peleg"
character reu "Reu" minor
  name ko "르우"
  name en "Reu"
character serug "Serug" minor
  name ko "스룩"
  name en "Serug"
character nahor "Nahor" minor                 # 스룩의 아들(11:22). 데라의 아들 나홀(11:26)과 다른 사람
  name ko "나홀"
  name en "Nahor"
character terah "Terah" minor
  name ko "데라"
  name en "Terah"
character abram "Abram" major
  name ko "아브람"
  name en "Abram"
character nahor_son_of_terah "Nahor" extra   # 11:26 "Abram, Nahor, and Haran"
  name ko "나홀(데라의 아들)"
  name en "Nahor"
character haran "Haran" extra
  name ko "하란"
  name en "Haran"

# 베드로의 부인
character jesus "Jesus" major
  name ko "예수"
  name en "Jesus"
  knows oracle                                # 부인을 미리 말한다(마 26:34). 정보 흐름 검사 면제, 출처는 남긴다
character peter "Peter" major
  name ko "베드로"
  name en "Peter"
  believes peter_with_jesus                   # 그는 예수의 제자다(마 26:33 "I will never be made to stumble")
  believes peter_knows_jesus
  believes peter_ready_to_die                 # 마 26:35 "Even if I must die with you" — 스스로 굳게 믿는다
character other_disciple "another disciple" extra   # 요 18:15 "known to the high priest"
  name ko "다른 제자"
  name en "another disciple"

# 묻는 사람들. 복음서마다 다른 사람이다(기록끼리 상태가 섞이지 않도록 따로 선언한다)
character matt_maid "a maid" extra            # 마 26:69
  name ko "여종"
  name en "a maid"
character matt_other_maid "someone else" extra   # 마 26:71 (헬라어 여성형 — "another maid")
  name ko "다른 여종"
  name en "someone else"
group matt_bystanders "those who stood by" extra   # 마 26:71 "those who were there", 26:73
  name ko "곁에 섰던 사람들"
  name en "those who stood by"
character mark_maid "one of the maids of the high priest" extra   # 막 14:66, 14:69
  name ko "대제사장의 여종 하나"
  name en "one of the maids of the high priest"
group mark_bystanders "those who stood by" extra   # 막 14:69-70
  name ko "곁에 섰던 사람들"
  name en "those who stood by"
character luke_maid "a certain servant girl" extra   # 눅 22:56
  name ko "한 여종"
  name en "a certain servant girl"
character luke_man "someone else" extra       # 눅 22:58 "Man, I am not!"
  name ko "다른 사람"
  name en "someone else"
character luke_other "another" extra          # 눅 22:59
  name ko "또 한 사람"
  name en "another"
character john_doorkeeper "the maid who kept the door" extra   # 요 18:16-17
  name ko "문 지키는 여종"
  name en "the maid who kept the door"
group john_servants "the servants and the officers" extra      # 요 18:18, 18:25 "They said therefore to him"
  name ko "종들과 하속들"
  name en "the servants and the officers"
character malchus_relative "a relative of him whose ear Peter had cut off" extra   # 요 18:26
  name ko "말고의 친척"
  name en "a relative of Malchus"

# 기록의 출처
thing matthew "The Gospel according to Matthew"
thing mark "The Gospel according to Mark"
thing luke "The Gospel according to Luke"
thing john "The Gospel according to John"

fact peter_with_jesus "Peter was with Jesus" true
fact peter_disciple = peter_with_jesus        # "one of them" / "one of his disciples" — 같은 고발이다
fact peter_knows_jesus "Peter knows Jesus" true
fact peter_will_deny "Peter will deny Jesus three times before the rooster crows" true
fact peter_ready_to_die "Peter will go with Jesus to death"
fact rooster_crows "the rooster crowed" true
fact rooster_crows_twice "the rooster crowed the second time" true
fact jesus_looks_at_peter "the Lord turned and looked at Peter" true
fact doorkeeper_admits_peter "the maid who kept the door let Peter in" true
```
