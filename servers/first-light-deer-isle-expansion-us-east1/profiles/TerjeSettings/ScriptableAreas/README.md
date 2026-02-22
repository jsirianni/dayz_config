### Psionic area (TerjePsionicScriptableArea)

**Classname:** TerjePsionicScriptableArea

This type of area impairs the sanity of the character in it and causes visual and auditory hallucinations. The zone has a power gradient between the inner and outer radii.

*IMPORTANT: It is recommended that the area size should be less then network synchronization bubble size (1200m by default) for the client-side visual effects works correctly.*

Supported `Data` parameters:

- `HeightMin`: The area is represented by a cylinder. The minimum height parameter defines the lower boundary of the area.
- `HeightMax`: The area is represented by a cylinder. The maximum height parameter defines the upper boundary of the area.
- `OuterRadius`: Outer radius of the area. The power will linearly lerped between the outer and inner radiuses.
- `InnerRadius`: Inner radius of the area. Inside this radius, the power is constant.
- `Power`: The power with which the area will affect the player. (recommended value is from 1 to 5).


### Skills experience modifier area (TerjeExperienceModScriptableArea)

**Classname:** TerjeExperienceModScriptableArea
**Filter**: A list of skill ids separated by comma that will be affected by this zone. (Example `hunt,fish`). Leave this field empty so that the experience modifier is applied to all skills at once.

This area type overwrites the global experience modifier for all players inside the area. Use this type of area for example if you want the player to get more experience for the hunting skill in a certain place on the map. This zone is a server-side zone and does not contain client logic, so it can have absolutely any radius.

Supported `Data` parameters:

- `HeightMin`: The area is represented by a cylinder. The minimum height parameter defines the lower boundary of the area.
- `HeightMax`: The area is represented by a cylinder. The maximum height parameter defines the upper boundary of the area.
- `Radius`: Radius of the area.
- `Power`: Value of experience multiplier.


