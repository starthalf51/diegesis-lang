---
module: yeoul.cast
mode: authored
---

# 여울섬 — 11화 추가 선언

```sl
character eungyeol "Cha Eungyeol" minor
  name ko "차은결"
  attr eungyeol.age = 32
  believes haesol_transferred
  believes eungyeol_posted
  believes eungyeol_holds
  believes step_renailed
  believes eungyeol_helps

thing old_ham "an old mother-of-pearl box"
  name ko "낡은 자개 함"
  attr old_ham.at = malsun_house

fact haesol_transferred "김해솔은 육지 해경서로 급히 발령이 나 새벽 첫 배로 섬을 떠났다" true
fact eungyeol_posted "차은결은 해솔의 후임으로 여울섬 해경 출장소에 부임한 순경이다" true
fact eungyeol_holds "은결은 해솔이 남긴 진료부와 파편 꾸러미, 백영호 진술서를 넘겨받아 출장소에 보관한다" true
fact eungyeol_helps "은결은 해솔이 하던 대로 서윤의 조사를 돕기로 한다" true
fact entry_cut "찢긴 장부 쪽의 맨 아래 줄은 '도경태가 불을 끄라 한 까닭은'에서 끊기고 그 다음 쪽은 찢겨 나가 없다" true
fact next_page_under_step "장부의 다음 쪽도 렌즈 파편과 함께 열세 번째 계단 판자 밑에 숨겨져 있다" false
fact step_renailed "파편 꾸러미를 꺼낸 뒤 열세 번째 계단 판자는 그 새 못으로 도로 박아 두었다" true
fact step_empty "새 못을 뽑고 판자를 들어 올리자 밑에는 먼지 앉은 빈자리만 있다" true
fact seoyun_thinks_malsun "서윤은 장부 쪽을 삼십 년 지켜 온 할머니라면 다음 쪽도 함께 간직했으리라 생각한다" true
fact ham_kept "오말순은 시집올 때 가져온 낡은 자개 함을 이불장 깊숙이 넣어 두었다" true
fact malsun_hid_page "오말순은 백영호에게서 받은 장부 다음 쪽을 양철 상자와 따로 자개 함에 숨겨 두었다" true
fact smuggle_motive "도경태가 그 밤 등대 불을 끄라 한 것은 섬 뒤 개펄 포구로 들어오던 자기 밀수선이 불빛에 드러나지 않게 하려는 것이었다" true
fact false_beacon "등대 불이 꺼진 사이 해진호는 밀수선이 흔든 불빛을 등대로 잘못 알고 암초 쪽으로 뱃머리를 돌렸다" true
fact seoyun_realizes "서윤은 아버지가 탄 배를 암초로 이끈 것이 꺼진 등대만이 아니라 밀수선의 거짓 불빛이었다는 것을 깨닫는다" true

artifact ledger_next "the next ledger page"
  name ko "찢긴 장부 다음 쪽"
  contains smuggle_motive, false_beacon

malsun believes ham_kept
malsun believes malsun_hid_page
```
