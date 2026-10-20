# Nasdara Expansion AI Implementation

Last updated: 2026-10-19

Status: initial plan recorded; scanner implementation has not started.

## Objective and scope

Produce hundreds of useful Expansion AI spawn locations for Nasdara through an
automated batch process. The operator must not have to inspect, teleport to, or
approve individual coordinates. Locations must provide sufficiently flat,
outdoor ground, correct elevation, character clearance, and a usable walking
exit. Validate complete group placements as well as their anchors.

The intended workflow uses one local DayZ client session with Nasdara loaded in
DayZ Editor. It does not require launching a dedicated server for the geometry
scan. A custom scanner still needs to be implemented and checked against the
installed game, Editor, and map versions. Actual Expansion AI spawning and
behavior are a later validation concern, distinct from geometry acceptance.

Nasdara is the target for all implementation work in this plan. Patrol and
equipment patterns from other servers may inform design; their coordinates
must not be transferred to Nasdara.

## Current evidence and constraints

- This server currently uses CF and VPPAdminTools. Expansion AI is not yet part
  of its documented mod setup. See [README.md](README.md).
- The checked-in mission is an override of an installed stock Nasdara mission;
  only `mpmissions/dayzOffline.nasdara/db/messages.xml` is retained. Obtain the
  complete matching mission data for analysis rather than assuming it is in
  this checkout. The README records an earlier stock reference at upstream
  commit `f220b2918b1d79a5055b1ee9be70c0ec1516e876`.
- The upstream Nasdara files inspected during planning contain 333 vehicle
  event positions and 20 fresh-player spawn area centers. These are search
  seeds, not verified AI spawn points. Recalculate these counts when choosing
  the implementation's source snapshot.
- Player spawn centers describe search grids. Object placements and loot
  layouts provide useful context but are not complete collision geometry or
  a terrain heightmap. Object origins and loot positions are not safe AI
  standing positions by themselves.
- Current upstream Expansion placement code can resolve against buildings,
  vehicles, roads, terrain, and other objects. Setting waypoint height to zero
  alone does not establish outdoor ground placement.
- DayZ Editor supports script execution and discovers installed worlds from
  `CfgWorlds`. Nasdara compatibility and reliable unattended scans have not
  been demonstrated locally.
- DayZ exposes navmesh sampling and path queries with movement filters. Their
  availability and behavior in the selected offline setup must be established.
- No scanner or in-game validation has been run. Geometry acceptance will
  describe the scanned map and object configuration; later vehicles, player
  construction, or changed map objects can alter clearance.

## Decisions already made

- Automate acceptance, rejection, retries, replacement candidates, and exports.
- Treat missing or inconclusive checks as unresolved, retry them, then reject
  them with a reason if they cannot be completed. Never silently skip a
  required check or count an unresolved location as accepted.
- Start with fixed, explicitly checked positions and zero random waypoint
  spread. Account for group formation offsets separately.
- Preserve source provenance, map/build information, configuration, results,
  and checkpoints so a scan can be audited and resumed.
- Keep target counts and area quotas configurable. A request for 300 accepted
  locations is an example, not an agreed requirement.
- Report an explicit shortfall when an area's search budget is exhausted.
  Individual rejected points do not become operator review tasks.

## Prerequisite: prove the offline scanning environment

- [ ] Identify the local DayZ installation and installed Nasdara version/assets.
- [ ] Confirm compatible DayZ Editor, CF, and Dabs Framework versions and load
      Nasdara locally without a dedicated server.
- [ ] Establish a scanner entry point, local input/output paths, and a start
      action that does not require repeated script entry.
- [ ] Demonstrate terrain, surface identity, object collision, and navigation
      queries in that session before running a large batch.
- [ ] Demonstrate movement between scan areas and a reliable readiness check
      for streamed collision geometry. An empty object query alone is not
      evidence that an area is loaded and clear.
- [ ] Check the acceptance logic against automatically identifiable examples
      of blocked, elevated, interior, and usable ground positions. Verify that
      unavailable geometry or navigation produces an unresolved result.

If a required offline capability is unavailable, record the evidence and revise
the execution approach here before claiming that it can validate spawn points.

## Step 1: Generate thousands of candidates

Status: not started.

- [ ] Select and record a complete Nasdara mission source snapshot appropriate
      for the installed map. Record commit/release and file checksums.
- [ ] Parse event positions, player spawn areas, mapped economy objects, and
      relevant hazard/underground definitions.
- [ ] Derive search areas around settlements, military sites, roads, and other
      useful patrol areas. Record each area's source and intended role.
- [ ] Generate a configurable candidate grid around those areas, with enough
      alternatives to replace failures. Do not assume an event's exact spawn
      coordinate is safe or unoccupied.
- [ ] Apply preliminary exclusions, including configured event footprints,
      hazards, and user-defined restricted areas. Document the limits of any
      object exclusion estimated without complete geometry.
- [ ] Assign stable candidate IDs, remove duplicates, and order candidates
      deterministically for reproducible scans.

Output: candidate coordinates and area metadata, an area quota definition, and a
source manifest. All coordinates remain unverified at this stage.

## Step 2: Load and scan areas automatically

Status: not started; depends on the offline environment prerequisite.

- [ ] Group candidates into spatial scan sections to reduce world movement.
- [ ] Move the local viewpoint/character automatically and wait for the
      section's geometry readiness criteria.
- [ ] Use a bounded queue and small batches so the client remains responsive.
- [ ] Add timeouts, bounded retries, and explicit unresolved outcomes.
- [ ] Save progress after each section and support resuming without losing
      completed results or duplicating accepted locations.
- [ ] Reject incompatible checkpoints when source data, map/build, loaded
      object mods, or acceptance settings change.

Output: resumable progress and a measured result for each processed candidate.
The operator starts a run; movement between individual points is automatic.

## Step 3: Reject unsuitable positions automatically

Status: not started.

- [ ] Resolve supporting surfaces and elevation; reject unwanted object
      surfaces, elevated structures, and unsupported positions.
- [ ] Measure local slope and abrupt height changes across the required
      standing footprint. A proposed initial slope limit is 5-8 degrees;
      select and validate the actual threshold during implementation.
- [ ] Reject interiors and overhead roofs/structures using appropriate world
      queries, rather than relying on a single raycast or a building name.
- [ ] Test standing-character clearance against relevant collision geometry.
- [ ] Reject water and configured hazards/restricted areas.
- [ ] Sample appropriate walkable navigation and require a usable nearby exit.
      Reject navigation that depends on disallowed interiors, swimming,
      ladders, or other movements outside the agreed spawn policy.
- [ ] Record measured values and specific rejection reasons such as
      `too_steep`, `uneven_ground`, `roof_overhead`, `object_surface`,
      `body_blocked`, `water`, `no_walkable_exit`, or `geometry_unresolved`.

Output: accepted or rejected geometry/navigation results with evidence. Numeric
limits, road treatment, clearance volumes, and query methods remain to be
implemented and validated, not assumed correct from the plan.

## Step 4: Validate complete group placements

Status: not started.

- [ ] Define supported group sizes, formations, scale, and orientation policy.
- [ ] Calculate and check every possible initial member placement using the
      same acceptance rules as the anchor.
- [ ] If initial orientation is uncontrolled, validate a conservative envelope
      covering the possible formation positions.
- [ ] Reject the group placement when any required member position fails.
- [ ] Keep random spread at zero initially. Require additional surrounding
      area checks before enabling any nonzero spread.
- [ ] Record which group configurations each accepted anchor supports.

Output: accepted group anchors with checked member positions or a checked
formation envelope. Anchor acceptance alone is insufficient.

## Step 5: Automatically replace failures

Status: not started.

- [ ] Set a requested accepted count and distribution across search areas.
- [ ] Select replacements automatically when a candidate or group fails.
- [ ] Enforce configurable separation and deduplication between selected
      anchors while preserving the requested area distribution.
- [ ] Continue until each area's quota is met or its bounded search budget is
      exhausted. Expand a search only within configured limits.
- [ ] Report unmet quotas and their rejection patterns; do not silently lower
      acceptance thresholds to reach a target.

Output: a selected spawn inventory plus explicit per-area completion/shortfall
results. Rejected candidates require no individual human approval.

## Step 6: Export ready-to-use results

Status: not started.

- [ ] Export accepted `[X, Y, Z]` coordinates, stable names, area assignments,
      supported group configurations, and measurement evidence.
- [ ] Export rejection reasons, run summaries, manifests, and checkpoints.
- [ ] Record the map/build, loaded object mods/mapping, source snapshot, scanner
      version, acceptance settings, and scan time with each run.
- [ ] Build a patrol generator that consumes the accepted inventory and an
      explicit faction/loadout/behavior/load-balancing policy.
- [ ] Keep generated patrol definitions traceable to accepted spawn IDs and
      enforce the checked formation/spread constraints.
- [ ] Verify loadout references and run matching `dzcli` validation before
      describing generated Expansion configuration as ready for use.
- [ ] Define a separate automated validation stage for actual Expansion AI
      spawning and movement when an appropriate runtime environment is
      available. Offline geometry acceptance must not be reported as passing
      that stage.

Output: machine-readable spawn inventory and reports, followed by generated,
validated Expansion patrol configuration when its gameplay policy is defined.

## Completion criteria

- One local offline Nasdara session can process a batch without per-point
  inspection, teleporting, script entry, or approval by the operator.
- Every accepted group passes all required geometry and navigation checks.
- Interruptions can resume safely, and incompatible cached results are refused.
- Requested quotas are met or accompanied by explicit, actionable shortfalls.
- Exports are reproducible and traceable to measured evidence and map versions.
- Actual AI runtime verification is identified separately from offline checks.

## Next implementation work

1. Establish the offline environment and query feasibility prerequisite.
2. Define the scanner input/result schema and acceptance settings.
3. Implement a small end-to-end batch before scaling to hundreds of anchors.
4. Use that batch's results to refine Steps 1-6 and document discovered limits.

Outstanding decisions include area quotas, patrol roles, group sizes, formation
policy, minimum separation, numerical acceptance thresholds, map-version
matching, collision readiness detection, and the scanner packaging/start method.

Follow the repository `dzcli` skill when inspecting or changing supported DayZ
configuration: bootstrap the local tool, inspect current values, preview
mutations with `--dry-run`, preserve baseline failures, and run matching
validators after changes. This initial change records a plan only.

## Maintaining this plan

Update this file in the same worktree as implementation changes and whenever
new evidence changes a capability, assumption, acceptance rule, dependency, or
workflow. Mark checklist items complete only after the corresponding work is
demonstrated. Record failed approaches and their practical implications when
they affect the remaining plan. Keep source links and verification limits next
to the decisions they support.

### Decision and discovery log

| Date | Decision or discovery | Effect |
| --- | --- | --- |
| 2026-10-19 | Nasdara selected as the implementation target. | Use Nasdara data and map geometry throughout. |
| 2026-10-19 | Individual manual spawn review is impractical. | Automate rejection, replacements, quotas, and reports. |
| 2026-10-19 | Offline Editor proposed as the scanning environment. | Prove query and streaming behavior before scaling. |
| 2026-10-19 | Economy XML and zero waypoint height cannot establish all placement requirements. | Require world geometry checks and measured evidence. |
| 2026-10-19 | Initial six-step plan recorded. | Implementation remains pending; update this document as work proceeds. |

## References

- [Bohemia Nasdara economy mission](https://github.com/BohemiaInteractive/DayZ-Central-Economy/tree/master/dayzOffline.nasdara)
- [Nasdara player spawn generation configuration](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/master/dayzOffline.nasdara/cfgplayerspawnpoints.xml)
- [DayZ Editor official Workshop page](https://steamcommunity.com/sharedfiles/filedetails/?id=2250764298)
- [Editor installed-world discovery](https://github.com/InclementDab/DayZ-Editor/blob/legacy/DayZEditor/Scripts/4_World/DayZEditor/gui/Dialogs/EditorMapSelectDialog.c)
- [Editor script execution](https://github.com/InclementDab/DayZ-Editor/blob/legacy/DayZEditor/Scripts/4_World/DayZEditor/classes/Commands/Hud/EditorScriptEditorCommand.c)
- [Bohemia navigation API](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/3_game/ai/aiworld.c)
- [Expansion placement implementation](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/experimental/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Settings/Patrols/ExpansionAIPatrolBase.c#L118)

Upstream links using branch names can change. Pin the versions actually used by
the scanner and record them in run manifests before relying on their behavior.
