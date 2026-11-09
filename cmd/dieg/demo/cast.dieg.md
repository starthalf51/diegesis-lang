---
module: demo.cast
mode: authored
---

# 등대의 열쇠 — 등장인물과 선언

Diegesis 데모용 짧은 이야기. 의도적으로 고칠 거리 둘이 들어 있다.

```sl
work lighthouse "The Lighthouse Key"
  unit day
  name ko "등대의 열쇠"
  name en "The Lighthouse Key"

character haru "Haru" major
  name ko "하루"
  name en "Haru"
character mina "Mina" minor
  name ko "미나"
  name en "Mina"
  believes tower_locked
character dobi "Dobi" minor
  name ko "도비"
  name en "Dobi"
  believes storm_coming
place harbor "the harbor"
  name ko "항구"
place tower "the lighthouse"
  name ko "등대"
thing key "the brass key"
  name ko "놋쇠 열쇠"
  name en "the brass key"

fact storm_coming "폭풍이 열흘 뒤에 온다"
fact key_in_lamp "도비는 열쇠를 등불 밑에 숨겼다"
fact tower_locked "등대 문은 잠겨 있다" true
```
