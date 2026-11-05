---
module: macbeth.act1
import: [macbeth.cast]
scene: "1.2"
location: camp
when: after start
characters: [duncan, malcolm, lennox]
status: draft
mode: authored
---

```sl
enter captain
captain tells duncan: "Macbeth unseamed Macdonwald"
enter ross, angus
ross tells duncan: cawdor_traitor "Assisted by that most disloyal traitor / The Thane of Cawdor"
beat decree:
  duncan orders ross: not alive(cawdor) "go pronounce his present death"
  duncan decrees: holds(thane_of_cawdor, macbeth) "with his former title greet Macbeth"
  duncan sends ross, angus to macbeth with holds(thane_of_cawdor, macbeth)
```

A bleeding soldier is carried into the camp near Forres and tells the king
how Macbeth carved his way through the rebels.

<!-- beat:decree -->

Duncan names the traitor's title for the victor.
