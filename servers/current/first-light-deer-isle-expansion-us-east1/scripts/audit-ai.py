"""Read-only AI policy audit; --write-report refreshes the checked-in evidence."""

import argparse
import collections
import csv
import hashlib
import json
from pathlib import Path
import xml.etree.ElementTree as ET

from audit_ai_loadouts import audit_loadouts

SERVER = Path(__file__).resolve().parents[1]
MISSION = SERVER / "empty.deerisle"
LOADOUTS = SERVER / "profiles/ExpansionMod/Loadouts"
VALIDATION = SERVER / "validation"


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def require(condition, message):
    if not condition:
        raise AssertionError(message)



def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main(write_report=False):
    patrol_file = MISSION / "expansion/settings/AIPatrolSettings.json"
    settings = read_json(patrol_file)
    patrols = settings["Patrols"]
    ai = read_json(SERVER / "profiles/ExpansionMod/Settings/AISettings.json")
    # Definitions, faction, kit, group size, cap, accuracy, threat/noise/flank,
    # speed, eligibility. The policy is independent of the authoring harness.
    policy = {
        "Civilian": (43, "Civilian", "DI_Civilian", 1, 6, .03, .15, 100, 50, 25, "WALK", 1),
        "PoliceStation": (4, "Guards", "DI_Police", 2, 3, .08, .25, 200, 100, 75, "JOG", 1),
        "PoliceCar": (2, "Guards", "DI_Police", 2, 1, .08, .25, 200, 100, 75, "JOG", 1),
        "Firefighter": (1, "Brawlers", "DI_Firefighter", 2, 3, .03, .15, 100, 50, 50, "JOG", 1),
        "Doctor": (3, "Civilian", "DI_Doctor", 1, 3, .03, .15, 100, 50, 25, "WALK", 1),
        "Medic": (2, "Civilian", "DI_Medic", 1, 3, .03, .15, 100, 50, 25, "WALK", 1),
        "NATO": (23, "East", "DI_NATO", 3, 6, .10, .35, 300, 150, 150, "JOG", .25),
        "Militia": (6, "Mercenaries", "DI_Militia", 2, 8, .05, .20, 200, 100, 100, "JOG", .35),
    }
    classes = {
        "PoliceStation": {"Land_City_PoliceStation", "Land_City_PoliceStation_Enoch", "Land_Village_PoliceStation", "Land_Village_PoliceStation_Enoch"},
        "PoliceCar": {"Land_Wreck_sed01_aban1_police", "Land_Wreck_sed01_aban2_police"},
        "Firefighter": {"Land_City_FireStation"},
        "Doctor": {"Land_City_Hospital", "Land_Village_HealthCare", "Land_Village_HealthCare_Enoch"},
        "Medic": {"Land_Medical_Tent_Big", "Land_WF_field_hospital_WEST_ep1"},
        "NATO": {"Land_Barrack_ep1", "Land_Mil_Airfield_HQ", "Land_Mil_ATC_Big", "Land_Mil_ATC_Small", "Land_Mil_Barracks_Round", "Land_Tisy_Barracks", "Land_Tisy_Garages3", "Land_Tisy_HQ"}
            | {f"Land_Mil_Barracks{i}" for i in range(1, 7)}
            | {f"Land_Mil_Tent_Big1_{i}" for i in range(1, 6)}
            | {f"Land_Mil_Tent_Big2_{i}" for i in range(1, 3)}
            | {"Land_Mil_Tent_Big3", "Land_Mil_Tent_Big4"},
        "Militia": {"Land_Tenement_Big", "Land_Tenement_Medium", "Land_Tenement_Small", "Land_Tenement_Small_Brick", "Land_Tenement_Small_Brown", "Land_Tenement_Small_Yellow"},
    }
    require(len(patrols) == 84, "Expected exactly 84 new definitions")
    require(len({p["Name"] for p in patrols}) == 84, "Patrol names must be unique")
    require(settings["m_Version"] == 32 and ai["m_Version"] == 20, "Schema versions changed")
    categories = settings["LoadBalancingCategories"]
    require(set(categories) == set(policy) | {"Global"}, "Unexpected or missing categories")
    require(categories["Global"] == [{"MinPlayers": 0, "MaxPlayers": 255, "MaxPatrols": 33}], "Global cap mismatch")
    summary = []
    for role, rule in policy.items():
        count, faction, kit, size, cap, low, high, threat, noise, flank, speed, chance = rule
        selected = [p for p in patrols if p["LoadBalancingCategory"] == role]
        require(len(selected) == count, f"{role} definition count")
        require(categories[role] == [{"MinPlayers": 0, "MaxPlayers": 255, "MaxPatrols": cap}], f"{role} cap")
        expected = {
            "Faction": faction, "Loadout": kit, "NumberOfAI": size, "NumberOfAIMax": 0,
            "AccuracyMin": low, "AccuracyMax": high, "ThreatDistanceLimit": threat,
            "NoiseInvestigationDistanceLimit": noise, "MaxFlankingDistance": flank,
            "Speed": speed, "Chance": chance, "EnableFlankingOutsideCombat": int(role == "NATO"),
            "Behaviour": "ROAMING_LOCAL", "UnderThreatSpeed": "SPRINT", "Persist": 0,
            "CanBeLooted": 1, "CanBeTriggeredByAI": 0, "CanSpawnInContaminatedArea": 0,
            "UnlimitedReload": 6, "MinDistRadius": 200, "MaxDistRadius": 900,
            "DespawnRadius": 1100, "DespawnTime": 300, "RespawnTime": 1800,
            "DamageMultiplier": 1, "DamageReceivedMultiplier": 1, "HeadshotResistance": 0,
            "ShoryukenChance": 0, "SniperProneDistanceThreshold": 0,
            "UseRandomWaypointAsStartPoint": 0, "Units": [], "DefaultStance": "STANDING",
            "Formation": "Vee" if role == "NATO" else "File", "FormationScale": 1.5,
            "FormationLooseness": 0,
        }
        looting = "BANDAGES"
        if role in ("Civilian", "Doctor", "Medic"):
            looting = "WEAPONS_FIREARMS | WEAPONS_MELEE | BANDAGES | UPGRADE"
        elif role == "Firefighter":
            looting = "WEAPONS_MELEE | BANDAGES | UPGRADE"
        expected["LootingBehaviour"] = looting
        expected["MinSpreadRadius"], expected["MaxSpreadRadius"] = ((0, 0) if role == "Civilian" else (10, 20) if role == "PoliceCar" else (20, 40))
        for patrol in selected:
            require(patrol["Name"].startswith(f"DI_{role}_"), f"Unclear name: {patrol['Name']}")
            for field, value in expected.items():
                require(patrol.get(field) == value, f"{patrol['Name']}: unexpected {field}")
            if role != "Civilian":
                require(patrol["Waypoints"] == [], f"Object patrol has absolute waypoints: {patrol['Name']}")
        if role != "Civilian":
            require({p["ObjectClassName"] for p in selected} == classes[role], f"{role} anchor allowlist")
        summary.append({"Role": role, "Definitions": count, "GroupSize": size, "PatrolCap": cap, "LivingAICap": size * cap, "Accuracy": [low, high]})
    require(sum(r["PatrolCap"] for r in summary) == 33, "Patrol budget arithmetic")
    require(sum(r["LivingAICap"] for r in summary) == 60, "Living AI budget arithmetic")
    spawn_points = ET.parse(MISSION / "cfgplayerspawnpoints.xml").findall("./fresh/generator_posbubbles/pos")
    civilians = [p for p in patrols if p["LoadBalancingCategory"] == "Civilian"]
    require(len(spawn_points) == 43, "Fresh-spawn source changed")
    for patrol, point in zip(civilians, spawn_points):
        require(patrol["ObjectClassName"] == "" and patrol["Waypoints"] == [[float(point.attrib["x"]), 0, float(point.attrib["z"])]], "Civilian coordinate mismatch")
    for field, value in {"Enabled": 1, "AccuracyMin": .05, "AccuracyMax": .25, "MinDistRadius": 200, "MaxDistRadius": 900, "DespawnRadius": 1100, "DespawnTime": 300, "RespawnTime": 1800, "ThreatDistanceLimit": 200, "NoiseInvestigationDistanceLimit": 100, "MaxFlankingDistance": 100, "EnableFlankingOutsideCombat": 0, "DamageMultiplier": 1, "DamageReceivedMultiplier": 1, "ShoryukenChance": 0}.items():
        require(settings[field] == value, f"Patrol file fallback: {field}")
    for field, value in {"AccuracyMin": .05, "AccuracyMax": .25, "ThreatDistanceLimit": 200, "NoiseInvestigationDistanceLimit": 100, "MaxFlankingDistance": 100, "EnableFlankingOutsideCombat": 0, "CanRecruitFriendly": 0, "CanRecruitGuards": 0, "PlayerFactions": ["West"], "DamageMultiplier": 1, "DamageReceivedMultiplier": 1, "ShoryukenChance": 0}.items():
        require(ai[field] == value, f"Global AI setting: {field}")

    loadouts = audit_loadouts(SERVER)

    # Inspect every JSON reference, including inactive quest/player templates.
    json_files = [p for p in SERVER.rglob("*.json") if VALIDATION not in p.parents]
    reference_count = 0
    def check_references(value, source):
        nonlocal reference_count
        if isinstance(value, dict):
            for key, child in value.items():
                if key in {"Loadout", "NPCLoadoutFile", "Include"} and isinstance(child, str) and child:
                    require((LOADOUTS / (child + ".json")).is_file(), f"Unresolved {key}={child} in {source}")
                    reference_count += 1
                check_references(child, source)
        elif isinstance(value, list):
            for child in value:
                check_references(child, source)
    for path in json_files:
        check_references(read_json(path), path.relative_to(SERVER).as_posix())
    require(read_json(SERVER / "profiles/ExpansionMod/Settings/QuestSettings.json")["EnableQuests"] == 0, "Expansion quests enabled")
    require(read_json(MISSION / "expansion/settings/SpawnSettings.json")["EnableSpawnSelection"] == 0, "Spawn selection enabled")
    locations = read_json(MISSION / "expansion/settings/AILocationSettings.json")["RoamingLocations"]
    require(len(locations) == 58 and all(p["Enabled"] == 1 for p in locations), "Roaming locations changed")
    preserved = read_json(VALIDATION / "ai-variety-preserved-hashes.json")
    for entry in preserved:
        require(digest(SERVER / entry["Path"]) == entry["Sha256"], f"Preserved file changed: {entry['Path']}")

    mapped = collections.Counter(g.attrib["name"] for g in ET.parse(MISSION / "mapgrouppos.xml").findall("group"))
    prototypes = {g.attrib["name"].lower(): {u.attrib["name"] for u in g.findall("usage")} for g in ET.parse(MISSION / "mapgroupproto.xml").findall("group")}
    event = ET.parse(MISSION / "db/events.xml").find("./event[@name='StaticPoliceCar']")
    require({c.attrib["type"] for c in event.findall("./children/child")} == classes["PoliceCar"] and event.findtext("active") == "1", "Police event anchors")
    event_positions = ET.parse(MISSION / "cfgeventspawns.xml").find("./event[@name='StaticPoliceCar']")
    require(len(event_positions.findall("pos")) == 35, "Police event position pool changed")
    object_rows = []
    for role, names in classes.items():
        for name in sorted(names):
            require(role == "PoliceCar" or mapped[name] > 0, f"No listed map placements: {name}")
            if role == "NATO":
                require("Military" in prototypes.get(name.lower(), set()), f"No military usage: {name}")
            object_rows.append({"Role": role, "ObjectClassName": name, "ListedStaticPlacements": mapped[name], "DynamicEvent": "StaticPoliceCar" if role == "PoliceCar" else "", "DynamicEventPotentialPositions": 35 if role == "PoliceCar" else 0})
    for role, count in {"PoliceStation": 33, "Firefighter": 20, "Doctor": 34, "Medic": 33, "NATO": 408, "Militia": 228}.items():
        require(sum(r["ListedStaticPlacements"] for r in object_rows if r["Role"] == role) == count, f"Placement inventory changed: {role}")
    report = {
        "Result": "PASS", "Verification": "Static configuration and source-data audit; no game or live server launched",
        "PatrolDefinitions": 84, "CoordinateDefinitions": 43, "ObjectDefinitions": 41,
        "AILoadoutFiles": 39, "RoleSelectors": 7, "AIArchetypes": 32, "PreservedPlayerLoadouts": 3, "RemovedLegacyAILoadouts": 24,
        "MaximumActivePatrols": 33, "MaximumLivingAI": 60, "Roles": summary,
        "WeaponCases": loadouts["WeaponCases"], "InventoryClassesInEconomyCatalogs": loadouts["InventoryClassCount"],
        "LoadoutVariety": {k: v for k, v in loadouts.items() if k not in {"Inventory", "WeaponCases"}},
        "ParsedServerJSONFiles": len(json_files), "ResolvedLoadoutReferences": reference_count,
        "PreservedFiles": preserved, "RoamingLocations": 58,
        "SourceSha256": {p: digest(MISSION / p) for p in ("cfgplayerspawnpoints.xml", "mapgrouppos.xml", "mapgroupproto.xml", "db/events.xml", "cfgeventspawns.xml")},
        "PendingRuntimeAcceptance": ["all 32 archetypes and 45 weapon branches", "powered optics and utility devices", "smallest-bag medical cargo packing and firearm readiness", "independent per-AI selection and specialist frequency", "faction relationships and existing player groups", "police aggression", "scavenging and unlimited reload target mask", "object placement and navigation", "local roaming and timing", "police event lifecycle cleanup", "performance at populated-server load"],
    }
    if write_report:
        (VALIDATION / "ai-overhaul-audit.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        (VALIDATION / "ai-variety-audit.json").write_text(json.dumps({"Result": "PASS", **loadouts}, indent=2) + "\n", encoding="utf-8")
        (VALIDATION / "ai-variety-files.txt").write_text("\n".join(sorted(p.name for p in LOADOUTS.glob("DI_*.json"))) + "\n", encoding="utf-8")
        with (VALIDATION / "ai-variety-inventory.csv").open("w", newline="", encoding="utf-8") as stream:
            writer = csv.DictWriter(stream, fieldnames=["ClassName", "Archetypes"])
            writer.writeheader()
            writer.writerows({"ClassName": r["ClassName"], "Archetypes": ";".join(r["Archetypes"])} for r in loadouts["Inventory"])
        with (VALIDATION / "ai-overhaul-object-classes.csv").open("w", newline="", encoding="utf-8") as stream:
            writer = csv.DictWriter(stream, fieldnames=list(object_rows[0]))
            writer.writeheader()
            writer.writerows(object_rows)
    print(json.dumps({key: report[key] for key in ("Result", "PatrolDefinitions", "AILoadoutFiles", "AIArchetypes", "MaximumActivePatrols", "MaximumLivingAI", "InventoryClassesInEconomyCatalogs", "ParsedServerJSONFiles")}, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write-report", action="store_true")
    main(parser.parse_args().write_report)
