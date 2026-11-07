---
module: haejin.cast
mode: authored
---

# 여울섬 — 11화 추가 선언

```sl
thing mangsari "Malsun's old net bag"
  name ko "오말순의 낡은 망사리"
  attr mangsari.at = malsun_house

fact shard_fits "정수아의 유리 조각이 깨진 등대 렌즈의 이가 빠진 자리에 꼭 맞는다" true
fact lens_broken_that_night "렌즈 귀퉁이는 해진호가 가라앉은 밤 꺼진 불을 서둘러 다시 켜려던 누군가가 깨뜨렸다" true
fact box_contents "반짇고리 안에는 오말순이 오래 감춰 둔 그물바늘과 망사리를 깁던 낡은 실 한 타래가 들어 있다" true
fact mangsari_stitched "오말순은 그 그물바늘로 낡은 망사리 안감에 무언가를 기워 넣었다" true
fact page_kyungtae_relit "찢긴 장부 쪽에는 새벽 한 시 무렵 도경태가 등대에 올라와 불을 다시 켜다 렌즈 귀퉁이를 깨뜨렸다고 적혀 있다" true

malsun believes malsun_keeps_page
malsun believes box_contents
malsun believes mangsari_stitched
```
