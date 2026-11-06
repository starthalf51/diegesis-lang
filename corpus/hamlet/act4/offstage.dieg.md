---
module: hamlet.act4
import: [hamlet.cast, hamlet.act3]
mode: authored
---

# 4막의 무대 밖 사건: 바다와 오필리어

```sl
# 극중극 날 밤 폴로니어스가 죽고(3.4), 왕은 새벽에 햄릿을 배에 태운다:
# 4.1.30-31 "The sun no sooner shall the mountains touch, / But we will ship him hence", 4.3.60 "I'll have him hence to-night"
event sails "Hamlet is shipped for England"
  when after 3.2 by 0d..1d
  lasts 0d..1d

# 4.6.14-15 "Ere we were two days old at sea, a pirate of very warlike appointment gave us chase"
event pirates "pirates take Hamlet prisoner"
  when after sails by 0d..2d
  lasts 0d..1d

# 4.6.19-21 "They have dealt with me like thieves of mercy … I am to do a good turn for them": 해적이 그를 덴마크에 내려 준다.
# 원문은 그 길이를 말하지 않는다. 일부러 느슨한 상한 1년을 둔다(4.5 에서 레어티즈가 파리에서 돌아오는 시간도 들어간다).
# 판정(1.2 의 20세 → 5.1 의 30세)은 1.2 → 5.1 상한이 8년 아래이기만 하면 성립하므로 이 상한에 기대지 않는다.
event lands "Hamlet is set ashore and writes to Horatio and the King"
  when after pirates by 0d..1y
  lasts 0d..1d
  hamlet sees: hamlet_returned src 4.7.46
  hamlet writes letter_horatio src 4.6.12
  hamlet writes letter_king src 4.7.46

# 4.7.179 "your sister's drown'd, Laertes" — 4.7 바로 전. 왕비가 어떻게 알았는지 원문은 말하지 않는다(4.7.181-199 의 묘사)
event drowning "Ophelia drowns in the brook"
  when after 3.2
  when before 4.7
  dies ophelia
  gertrude believes ophelia_drowned
```
