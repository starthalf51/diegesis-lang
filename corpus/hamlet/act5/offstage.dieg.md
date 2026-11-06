---
module: hamlet.act5
import: [hamlet.cast, hamlet.act4]
mode: authored
---

# 5막의 무대 밖 사건

```sl
# 5.1.142-147 무덤 파는 사람: "I came to't that day that our last king Hamlet overcame Fortinbras … it was the very
# day that young Hamlet was born". 세 사건(승리, 그의 첫 근무, 햄릿의 출생)은 같은 날이다(출생 뒤 하루 안)
event fortinbras_overcome "King Hamlet overcomes old Fortinbras"
  when after @hamlet.born by 0d..1d          # "that day": 같은 날
  lasts 0d..1d
event sexton_begins "the gravedigger comes to his trade"
  when after @hamlet.born by 0d..1d          # "that day": 같은 날
  lasts 0d..1d

# 5.1.4 "the crowner hath sat on her, and finds it Christian burial"
event inquest "the crowner sits on Ophelia"
  when after drowning
  when before 5.1
  crowner sees: ophelia_drowned src 5.1.4
  crowner tells clown2: ophelia_drowned src 5.1.4
```
