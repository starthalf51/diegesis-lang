---
module: macbeth.act1
import: [macbeth.cast]
mode: authored
---

# 1막의 무대 밖 사건

```sl
event battle "the battles against Macdonwald and Norway"
  when before 1.2
  captain sees: macdonwald_slain src 1.2.18
  # 로스는 파이프 전장에서 왔다(1.2.55). 맥베스는 코더가 도운 노르웨이군과 맞섰다(1.2.59-63).
  # 맥베스가 코더의 반역을 봤는지는 학계 논쟁이다(1.3.74 참조). 이 포팅은 봤다고 쓴다.
  ross sees: cawdor_traitor src 1.2.55
  macbeth sees: cawdor_traitor src 1.2.61

event cawdor_execution "Cawdor is executed"
  when after 1.2.decree
  when before 1.4
  dies cawdor
  witness sees: not alive(cawdor) src 1.4.5
  witness tells malcolm: not alive(cawdor) "That very frankly he confess'd his treasons" src 1.4.6

event macbeth_writes "Macbeth writes to his wife"
  when after 1.3.ross_news
  when before 1.5
  macbeth writes macbeth_letter src 1.5.1

event harbinger "a servant outrides Macbeth to Inverness"
  when after 1.4.departure                          # 맥베스는 1.4.60 에서 먼저 떠난다
  when before 1.5
  macbeth tells fellow: duncan_comes "I'll be myself the harbinger" src 1.4.51
  fellow tells messenger: duncan_comes "almost dead for breath" src 1.5.37
```
