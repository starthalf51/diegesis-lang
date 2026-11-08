---
module: yeoul.cast
mode: authored
---

# 여울섬 — 12화 추가 선언

```sl
character yuna "Lim Yuna" minor
  name ko "임유나"
  attr yuna.age = 34
  believes yuna_posted
  believes yuna_helps
  believes yuna_will_show

fact eungyeol_called "차은결은 투표 날 새벽 먼바다 조난 신고를 받고 경비정에 올라 섬을 떠난다" true
fact handover "은결은 진료부와 파편 꾸러미, 백영호 진술서를 임유나에게 모두 넘긴다" true
fact yuna_posted "임유나는 해진호 사고를 다시 들여다보러 섬에 온 해양안전심판원 조사관이다" true
fact yuna_helps "유나는 은결 대신 서윤의 조사를 끝까지 돕기로 한다" true
fact yuna_will_show "유나는 투표장에서 진료부와 진술서를 마을 사람들 앞에 내놓겠다고 한다" true
fact malsun_forgets "투표 날 아침 오말순은 장부 쪽도 자개 함도 무엇 하나 기억하지 못한다" true
fact haeun_came "하은은 투표장에 가기 전에 아버지 몰래 서윤을 찾아왔다" true
fact margin_note "장부 다음 쪽 뒷면 귀퉁이에는 백영호가 '이튿날 오후 도경태 육지 병원으로 나감. 사흘 전부터 입원해 있었다 하라 함'이라고 덧적어 둔 글씨가 있다" true
fact forged_admission "도경태는 보건지소에서 손을 꿰맨 그날 오후 육지 병원에 들어가 입원 날짜를 사고 사흘 전으로 고쳐 적게 했다" true
fact haeun_realizes "하은은 아버지가 사고 날 밤 육지 병원에 누워 있었다는 말이 처음부터 꾸며 낸 거짓이라는 것을 깨닫는다" true
fact haeun_resolve "하은은 투표장에서 마을 사람들 앞에 그 장부 쪽을 직접 펼치겠다고 한다" true
fact alibi_collapses "도경태가 삼십 년 동안 내세운 입원 이야기는 마을 사람들 앞에서 무너진다" true

eungyeol believes eungyeol_called
eungyeol believes handover
yeongho believes margin_note
yeongho believes forged_admission
haeun believes haeun_came
```
