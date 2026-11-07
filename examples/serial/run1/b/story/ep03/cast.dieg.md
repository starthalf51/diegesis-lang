---
module: haejin.cast
mode: authored
---

# 여울섬 — 3화 추가 선언

```sl
place beach "the beach below the lighthouse"
  name ko "등대 아래 바닷가"

fact haeun_teaches "도하은은 여울섬 분교에서 아이들을 가르친다" true
fact sua_on_beach "정수아는 아침마다 등대 아래 바닷가에 나가 파도에 밀려온 것을 줍는다" true
fact sua_found_shard "정수아는 그 유리 조각을 등대 아래 바닷가 자갈 틈에서 주웠다" true
fact shard_thick "유리 조각은 손가락 마디만큼 두껍고 한쪽 면에 둥근 홈이 겹겹이 패어 있다" true
fact shard_ordinary "그 유리 조각은 바다에 쓸려 온 흔한 병 조각일 뿐이다" false
fact sua_keeps_shards "정수아는 바닷가에서 주운 것을 과자 상자에 모아 둔다" true
fact sua_saw_figure "며칠 전 밤 정수아는 누군가 손전등을 들고 버려진 등대로 올라가는 것을 보았다" true
fact someone_climbs "요즘 누군가 버려진 등대를 몰래 오르내린다" true

haeun believes haeun_teaches
haeun believes sua_on_beach
sua believes sua_found_shard
sua believes sua_keeps_shards
sua believes sua_saw_figure
sua believes lighthouse_dark_now
```
