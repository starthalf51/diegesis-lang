---
module: bible.genesis
import: [bible.cast]
mode: authored
---

# 창세기 5장·11:10-26 — 아담에서 아브람까지

나이는 모두 본문 그대로다. "X lived N years, then became the father of Y" → `born y when after @x.born by Ny..Ny`,
"All the days of X were N years, then he died" → `dies x when after @x.born by Ny..Ny`.
11장은 남은 햇수(" lived N years after he became the father of Y")를 주므로 수명 = 낳은 나이 + 남은 햇수다.

- 에녹은 죽지 않는다(5:24 "he was not found, for God took him"). `dies` 대신 무대 밖 사건 `enoch_taken` 으로 쓴다.
- 홍수는 노아 600세(7:6). 끝은 9:28 "Noah lived three hundred fifty years after the flood" 와 9:29 수명 950 으로
  같은 해에 고정된다. 11:10 "two years after the flood" 는 이 시점에서 잰다.
- 5:32 "Noah was five hundred years old, then Noah became the father of Shem, Ham, and Japheth" 를 셈에게 문자 그대로
  적용하면 11:10(홍수 2년 뒤 셈 100세 → 노아 502세에 태어남)과 맞지 않는다. 문자 그대로 적고 사유를 달아 억제한다.
  함과 야벳은 본문에 따로 나이가 없어 `parent` 만 쓴다.
- 11:26 "Terah lived seventy years, and became the father of Abram, Nahor, and Haran" 도 같은 꼴이다. 범위 안에서는
  충돌이 없으므로 아브람에게 문자 그대로 적는다(11:32·12:4 와의 긴장은 범위 밖이다). 나홀·하란은 `parent` 만.
- 한 사람의 `born` 은 한 번만 쓸 수 있다. 11:10 은 아르박삿의 출생에 두 기준(홍수 + 2년, 셈 + 100년)을 주므로
  무대 밖 사건 `shem_fathers_arpachshad` 에 두 `when` 을 달고 그 안에서 `born arpachshad` 한다. 사건의 길이는
  11:11("Shem lived five hundred years after") + 셈의 수명 600 으로 0 이 된다(시작 = 끝 = 아르박삿 출생).
- 데라·아브람·나홀·하란의 죽음은 범위 밖(11:32, 25:7)이다.

```sl
born adam when after start by 0y..0y                       # 5:1-2 "In the day that God created man"

parent adam of seth
born seth when after @adam.born by 130y..130y              # 5:3
dies adam when after @adam.born by 930y..930y              # 5:5

parent seth of enosh
born enosh when after @seth.born by 105y..105y             # 5:6
dies seth when after @seth.born by 912y..912y              # 5:8

parent enosh of kenan
born kenan when after @enosh.born by 90y..90y              # 5:9
dies enosh when after @enosh.born by 905y..905y            # 5:11

parent kenan of mahalalel
born mahalalel when after @kenan.born by 70y..70y          # 5:12
dies kenan when after @kenan.born by 910y..910y            # 5:14

parent mahalalel of jared
born jared when after @mahalalel.born by 65y..65y          # 5:15
dies mahalalel when after @mahalalel.born by 895y..895y    # 5:17

parent jared of enoch
born enoch when after @jared.born by 162y..162y            # 5:18
dies jared when after @jared.born by 962y..962y            # 5:20

parent enoch of methuselah
born methuselah when after @enoch.born by 65y..65y         # 5:21

event enoch_taken "Enoch walked with God, and he was not found, for God took him"
  when after @enoch.born by 365y..365y                     # 5:23-24

parent methuselah of lamech
born lamech when after @methuselah.born by 187y..187y      # 5:25
dies methuselah when after @methuselah.born by 969y..969y  # 5:27

parent lamech of noah
born noah when after @lamech.born by 182y..182y            # 5:28-29
dies lamech when after @lamech.born by 777y..777y          # 5:31

parent noah of shem, ham, japheth                          # 5:32
#! allow time-cycle: "5:32 은 세 아들 중 첫아들이 난 해(노아 500세)를 적은 것으로 읽는 것이 일반적이다. 셈이 500세에 났을 필요는 없다 — 11:10 '홍수 2년 뒤 셈 100세' 로 셈은 노아 502세에 났다."
born shem when after @noah.born by 500y..500y              # 5:32 (문자 그대로)

event flood "the flood of waters came on the earth"
  when after @noah.born by 600y..600y                      # 7:6
  when before @noah.died by 350y..350y                     # 9:28
dies noah when after @noah.born by 950y..950y              # 9:29

# 11:10 "Shem was one hundred years old when he became the father of Arpachshad two years after the flood"
parent shem of arpachshad
event shem_fathers_arpachshad "Shem became the father of Arpachshad"
  when after flood by 2y..2y                               # 11:10 "two years after the flood"
  when after @shem.born by 100y..100y                      # 11:10 "Shem was one hundred years old"
  when before @shem.died by 500y..500y                     # 11:11 "Shem lived five hundred years after"
  born arpachshad
dies shem when after @shem.born by 600y..600y              # 11:10-11: 100 + 500

parent arpachshad of shelah
born shelah when after @arpachshad.born by 35y..35y        # 11:12
dies arpachshad when after @arpachshad.born by 438y..438y  # 11:12-13: 35 + 403

parent shelah of eber
born eber when after @shelah.born by 30y..30y              # 11:14
dies shelah when after @shelah.born by 433y..433y          # 11:14-15: 30 + 403

parent eber of peleg
born peleg when after @eber.born by 34y..34y               # 11:16
dies eber when after @eber.born by 464y..464y              # 11:16-17: 34 + 430

parent peleg of reu
born reu when after @peleg.born by 30y..30y                # 11:18
dies peleg when after @peleg.born by 239y..239y            # 11:18-19: 30 + 209

parent reu of serug
born serug when after @reu.born by 32y..32y                # 11:20
dies reu when after @reu.born by 239y..239y                # 11:20-21: 32 + 207

parent serug of nahor
born nahor when after @serug.born by 30y..30y              # 11:22
dies serug when after @serug.born by 230y..230y            # 11:22-23: 30 + 200

parent nahor of terah
born terah when after @nahor.born by 29y..29y              # 11:24
dies nahor when after @nahor.born by 148y..148y            # 11:24-25: 29 + 119

parent terah of abram, nahor_son_of_terah, haran           # 11:26
born abram when after @terah.born by 70y..70y              # 11:26 (문자 그대로)
```
