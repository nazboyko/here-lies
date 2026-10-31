# Eval results

Real photos from Wikimedia Commons, the real pipeline, the real model. Run on 2026-10-31.

- Model: `gemma4:e4b` through Ollama 0.35.1
- Machine: Apple M5 Max, 64 GB memory

## Summary

| | |
|---|---|
| Photos | 17 |
| Gate decision correct (thing / person / animal / living plant / unclear) | 16 of 17 |
| Obituaries printed | 13 |
| CHECK passed on the first draft | 12 of 13 |
| Rewritten once after CHECK | 1 |
| Repaired by code after the retry (fallback) | 0 |
| Details listed on printed obituaries | 65 |
| Details really in the photo (reviewed by eye) | 53 true, 4 false, 8 unsure |
| Causes of death resting on a detail marked false | 2 |
| Median seconds, LOOK (all photos) | 0.8 |
| Median seconds, WRITE + CHECK (obituaries) | 2.4 |

Causes of death built on a detail that is not in the photo: 10-warped-cassette-adapter.jpg ("missing screw hole"); 11-abandoned-umbrella.jpg ("torn fabric"). CHECK can prove that every sentence points at a listed detail; it cannot prove the detail itself is real. That part is the vision model's job, and this review measures it.

## Per photo

| photo | expected | subject | object | details | first-try pass | retried | fallback | soft mismatches | look s | write s | details_true (reviewed) |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 01-cracked-phone.jpg | thing | thing | smartphone | 5 | yes | no | no | 0 | 0.8 | 2.3 | 4/5 (0 false, 1 unsure) |
| 02-frayed-charger-cable.jpg | thing | thing | white cable | 5 | no | yes | no | 1 | 0.8 | 5.0 | 5/5 |
| 03-worn-out-shoes.jpg | thing | thing | pair of worn canvas shoes | 6 | yes | no | no | 0 | 1.0 | 2.6 | 5/6 (0 false, 1 unsure) |
| 04-burnt-toast.jpg | thing | thing | slice of toast | 5 | yes | no | no | 0 | 0.8 | 2.5 | 4/5 (0 false, 1 unsure) |
| 05-broken-mug.jpg | thing | thing | white creamer jug | 5 | yes | no | no | 0 | 0.8 | 2.4 | 4/5 (1 false, 0 unsure) |
| 06-broken-drinking-glass.jpg | thing | thing | glass vase | 6 | yes | no | no | 1 | 0.8 | 2.6 | 5/6 (1 false, 0 unsure) |
| 07-collapsing-jack-o-lanterns.jpg | thing | thing | carved pumpkins | 5 | yes | no | no | 1 | 0.8 | 2.4 | 4/5 (0 false, 1 unsure) |
| 08-burnt-light-bulb.jpg | thing | thing | light bulb | 6 | yes | no | no | 0 | 0.9 | 2.2 | 6/6 |
| 09-dead-succulent.jpg | thing | thing | succulent plant in terracotta pot | 6 | yes | no | no | 1 | 1.0 | 2.4 | 6/6 |
| 10-warped-cassette-adapter.jpg | thing | thing | black electronic adapter | 5 | yes | no | no | 0 | 0.8 | 2.3 | 3/5 (1 false, 1 unsure) |
| 11-abandoned-umbrella.jpg | thing | thing | red umbrella | 4 | yes | no | no | 1 | 0.7 | 2.2 | 2/4 (1 false, 1 unsure) |
| 12-cat.jpg | animal | animal | – | notice: Still alive. |  |  |  |  | 0.7 |  |  |
| 13-dog.jpg | animal | animal | – | notice: Still alive. |  |  |  |  | 0.7 |  |  |
| 14-healthy-monstera.jpg | living_plant | living_plant | – | notice: This plant is still with us. |  |  |  |  | 0.8 |  |  |
| 15-person-portrait.jpg | person | person | – | notice: We don't write obituaries for people. |  |  |  |  | 0.7 |  |  |
| 16-intact-mug.jpg | thing | thing | poured roasted coffee beans | 2 | yes | no | no | 0 | 0.6 | 2.0 | 2/2 |
| 17-blurry-unclear.jpg | unclear | thing | bulbous plant structure | 5 | yes | no | no | 0 | 1.0 | 2.4 | 3/5 (0 false, 2 unsure) |

## Review notes

- **01-cracked-phone.jpg**: The red is the side frame; the front bezel is black.
- **03-worn-out-shoes.jpg**: The canvas is grey-beige; 'light brown' is a stretch.
- **04-burnt-toast.jpg**: The centres are charred; the crust edges are mostly golden.
- **05-broken-mug.jpg**: It is a mug, not a creamer jug, and it has no spout. The 'yellow line' is a printed pattern that runs along the break.
- **06-broken-drinking-glass.jpg**: It is a drinking glass, not a vase. No scratches are visible.
- **07-collapsing-jack-o-lanterns.jpg**: Rot is real (the small one has collapsed). Dirt on the pumpkins is not clear.
- **10-warped-cassette-adapter.jpg**: The real injury is heat warping, which was missed. 'Missing screw hole' is not visible, and it became the cause of death.
- **11-abandoned-umbrella.jpg**: The umbrella is collapsed, but no tear is visible. 'Torn fabric' became the cause of death.
- **16-intact-mug.jpg**: The mug is intact, so natural causes is right; the obituary went to the coffee beans.
- **17-blurry-unclear.jpg**: Should have been refused as unclear. It is a blurred green glow, read as a plant.
- **02-frayed-charger-cable.jpg**, first draft rejected by CHECK: life sentence 3 cites no detail: every sentence must mention a detail and list its id

## Variability: the same photo, three runs

**01-cracked-phone.jpg**

| run | name | details |
|---|---|---|
| 1 | Eleanor B. Screenphone | spiderweb cracks across screen; multiple surface scratches; general surface grime; black screen; red bottom bezel |
| 2 | Miss Clara Digit-Phone | cracked screen; spiderweb cracks on display; scratches on back casing; black glossy screen; red bottom bezel |
| 3 | Miss Elara Digit-Screenworth | cracked screen; spiderweb cracks on display; scratches on back casing; black screen; red border |

**04-burnt-toast.jpg**

| run | name | details |
|---|---|---|
| 1 | Algernon Brownsworth, S. Toast | burnt surface on both slices; charred edges; discoloration on toast; golden brown crust; square shape |
| 2 | Mr. Percival Crispington, Crust | burnt edges; charred surface; discoloration; golden brown crust; square shape |
| 3 | Edmund Crispy Toastsworth | burnt edges; charred surface; discoloration; golden brown color; square shape |

**07-collapsing-jack-o-lanterns.jpg**

| run | name | details |
|---|---|---|
| 1 | Mr. Josiah Gourdington | rotting; dirt; orange color; jack-o'-lantern faces; stem on one pumpkin |
| 2 | Mr. Josiah Gourdington | rotting; dirt; orange color; jack-o'-lantern faces; stem on largest pumpkin |
| 3 | Silas Gourdington III | cracks on pumpkin skin; rotting appearance of small pumpkin; dirt on pumpkin surfaces; orange color; jack-o'-lantern faces; stem on larger pumpkin |

