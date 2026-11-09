---
module: yeoul.cast
mode: authored
---

# 여울섬 — 8화 추가 선언

```sl
fact ledger_entry "찢긴 장부 쪽에는 해진호가 가라앉은 밤 '소등 지시 거부함. 도경태가 등실 렌즈를 깨뜨려 불 꺼짐'이라는 글씨와 백영호의 근무 서명이 적혀 있다" true
fact seoyun_suspects "서윤은 그 밤 등실 렌즈를 깨뜨려 등대 불을 꺼뜨린 사람이 도경태라고 의심한다" true
fact seoyun_links "서윤은 선착장 방파제 끝에서 나온 유리 조각과 깨진 렌즈 귀퉁이, 도경태 손의 베인 상처가 모두 그 한밤에 생긴 것이라고 짐작한다" true
fact seoyun_probe_plan "서윤은 날이 밝으면 도경태를 직접 떠보기로 한다" true
fact scar_from_net "도경태의 손바닥 흉터는 젊을 적 그물을 걷다 베인 것이다" false
fact jihwan_offer "개발사는 투표가 가결되면 오말순을 육지의 좋은 요양병원에 모시는 비용을 모두 대겠다고 한다" true
fact jihwan_asks_quiet "류지환은 서윤에게 투표가 끝날 때까지만 지난 일을 들추지 말아 달라고 청한다" true
fact jihwan_knows "서윤은 류지환이 도경태에게서 자기 이야기를 전해 들었다고 짐작한다" true
fact demolition_rush "도경태는 개발사에 투표가 가결되면 등대부터 하루라도 빨리 허물어 달라고 재촉한다" true
fact gyeongtae_share "투표가 가결되면 도경태는 개발사에서 이장 몫으로 따로 큰돈을 받기로 되어 있다" true
fact lighthouse_buries "도경태는 등대만 사라지면 삼십 년 전 그 밤 일은 영영 묻힌다고 말한다" true
fact haeun_shaken "하은은 아버지가 등대를 서둘러 허물려는 까닭이 돈만이 아닐지도 모른다고 느낀다" true
fact haeun_heard_nothing "하은은 방금 막 들어와 아무 말도 듣지 못했다" false

malsun believes malsun_keeps
gyeongtae believes not scar_from_net
gyeongtae believes demolition_rush
gyeongtae believes gyeongtae_share
gyeongtae believes lighthouse_buries
jihwan believes jihwan_offer
jihwan believes jihwan_asks_quiet
haeun believes not haeun_heard_nothing
```
