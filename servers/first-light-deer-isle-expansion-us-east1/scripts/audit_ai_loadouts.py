"""Branch-aware, exact-probability audit of the Deer Isle starting inventory.

Models intended successful item creation, not engine compatibility or cargo fit.
All candidates must have Chance=1; repeated tickets supply explicit weights.
"""
import collections
from fractions import Fraction
import json
import xml.etree.ElementTree as ET

ARCHETYPES = {
    "Civilian": dict(Resident=25, Worker=20, Commuter=20, Hiker=15, Traveller=10, Forager=10),
    "Police": dict(Patrol=45, Station=25, Traffic=20, Response=10),
    "Firefighter": dict(FireCrew=50, RescueCrew=35, StationCrew=15),
    "Medic": dict(Ambulance=40, Trauma=25, Field=20, Supply=15),
    "Doctor": dict(General=40, Emergency=30, Surgeon=20, Clinic=10),
    "NATO": dict(Rifleman=40, Pointman=15, FieldMedic=15, Engineer=10, TeamLeader=10, Marksman=10),
    "Militia": dict(Hunter=30, Homesteader=25, Scavenger=20, Poacher=15, Veteran=10),
}
WEAPONS = {
    "Police": {"Patrol": ["Glock19", "CZ75", "MP5K"], "Station": ["MP5K", "UMP45", "Mp133Shotgun"],
               "Traffic": ["Glock19", "CZ75", "FNX45"], "Response": ["MP5K", "UMP45", "Mp133Shotgun"]},
    "NATO": {"Rifleman": ["M4A1", "M16A2", "Aug", "FAMAS"], "Pointman": ["MP5K", "UMP45", "Aug"],
             "FieldMedic": ["M4A1", "M16A2", "Aug"], "Engineer": ["M4A1", "FAMAS", "UMP45"],
             "TeamLeader": ["M4A1", "Aug"], "Marksman": ["FAL", "M14"]},
    "Militia": {"Hunter": ["CZ527", "Winchester70", "Mosin9130"],
                "Homesteader": ["Izh18", "Izh18Shotgun", "Izh43Shotgun", "Repeater"],
                "Scavenger": ["Ruger1022", "Izh18Shotgun", "Mp133Shotgun"],
                "Poacher": ["CZ527", "Mosin9130", "SKS"], "Veteran": ["SKS", "Mosin9130", "Mp133Shotgun"]},
}
AMMUNITION = {
    "Glock19": "Mag_Glock_15Rnd", "CZ75": "Mag_CZ75_15Rnd", "FNX45": "Mag_FNX45_15Rnd",
    "MP5K": "Mag_MP5_30Rnd", "UMP45": "Mag_UMP_25Rnd", "Mp133Shotgun": "Ammo_12gaPellets",
    "M4A1": "Mag_STANAG_30Rnd", "M16A2": "Mag_STANAG_30Rnd", "Aug": "Mag_Aug_30Rnd",
    "FAMAS": "Mag_FAMAS_25Rnd", "FAL": "Mag_FAL_20Rnd", "M14": "Mag_M14_20Rnd",
    "CZ527": "Mag_CZ527_5rnd", "Winchester70": "Ammo_308Win", "Mosin9130": "Ammo_762x54",
    "Izh18": "Ammo_762x39", "Izh18Shotgun": "Ammo_12gaPellets", "Izh43Shotgun": "Ammo_12gaPellets",
    "Repeater": "Ammo_357", "Ruger1022": "Mag_Ruger1022_15Rnd", "SKS": "Ammo_762x39",
}
SYNTHETIC = {"ARCHETYPE", "BAG", "HEADWEAR", "MASK", "UNIFORM", "UTILITY", "WEAPON", "FURNITURE"}
BAGS = {
    "Civilian": {None: 50, "Slingbag_Black": 10, "Slingbag_Brown": 10, "Slingbag_Gray": 10, "TaloonBag_Blue": 10, "TaloonBag_Green": 10},
    "Police": {None: 70, "Slingbag_Black": 15, "DuffelBagSmall_Green": 15},
    "Firefighter": {None: 60, "DrysackBag_Orange": 20, "DuffelBagSmall_Green": 20},
    "Medic": {"DuffelBagSmall_Medical": 50, "CanvasBag_Medical": 50},
    "Doctor": {"DuffelBagSmall_Medical": 50, "CanvasBag_Medical": 50},
    "NATO": {name: Fraction(100, 6) for name in ("AssaultBag_Green", "AssaultBag_Black", "CoyoteBag_Green", "CoyoteBag_Brown", "HuntingBag", "AliceBag_Green")},
    "Militia": {None: 40, "Slingbag_Brown": 15, "Slingbag_Black": 15, "HuntingBag": 15, "TaloonBag_Green": 15},
}


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def walk(node):
    yield node
    for slot in node.get("InventoryAttachments", []):
        for item in slot["Items"]:
            yield from walk(item)
    for field in ("InventoryCargo", "Sets"):
        for item in node.get(field, []):
            yield from walk(item)


class Stats:
    def __init__(self, low=None, high=None, mean=None):
        self.low = collections.Counter(low or {})
        self.high = collections.Counter(high or {})
        self.mean = collections.Counter(mean or {})

    def add(self, other):
        return Stats(self.low + other.low, self.high + other.high, self.mean + other.mean)

    @classmethod
    def event(cls, key):
        return cls({key: 1}, {key: 1}, {key: Fraction(1)})

    @classmethod
    def choice(cls, branches):
        require(bool(branches), "Empty candidate pool")
        keys = set().union(*(b.high for b in branches))
        return cls({k: min(b.low[k] for b in branches) for k in keys},
                   {k: max(b.high[k] for b in branches) for k in keys},
                   {k: sum((b.mean[k] for b in branches), Fraction()) / len(branches) for k in keys})


def analyze(node, kits, path="", stack=()):
    require(node["Chance"] == 1, "Exact ticket analysis requires Chance=1")
    result = Stats()
    cls = node["ClassName"]
    if cls and cls not in SYNTHETIC:
        result = result.add(Stats.event(("item", cls)))
    include = node["Include"]
    if include:
        require(include in kits, f"Missing include {include}")
        require(include not in stack, f"Include cycle: {' -> '.join(stack + (include,))}")
        result = result.add(analyze(kits[include], kits, path, stack + (include,)))
    for cargo in node["InventoryCargo"]:
        result = result.add(analyze(cargo, kits, path + "/cargo:" + cargo["ClassName"], stack))
    groups = collections.defaultdict(list)
    for branch in node["Sets"]:
        require(branch["ClassName"] in SYNTHETIC, "Every set must have an exclusive group name")
        groups[branch["ClassName"]].append(branch)
    for branches in groups.values():
        result = result.add(Stats.choice([analyze(b, kits, path, stack) for b in branches]))
    for slot in node["InventoryAttachments"]:
        name = slot["SlotName"]
        require(slot["Items"], "Empty attachment group")
        require(name or len(slot["Items"]) == 1, "Automatic slot resolution must have one unambiguous item")
        branches = []
        for item in slot["Items"]:
            slot_path = path + "/" + (name or "auto:" + item["ClassName"])
            stats = Stats.event(("occupied", slot_path)).add(Stats.event(("slot", slot_path, item["ClassName"])))
            branches.append(stats.add(analyze(item, kits, slot_path, stack)))
        result = result.add(Stats.choice(branches))
    for key, maximum in result.high.items():
        require(key[0] != "occupied" or maximum <= 1, f"Multiple owners of slot {key}")
    return result


def product(left, right):
    return {tuple(sorted(a + b)) for a in left for b in right}


def visible(node, kits):
    """Conservative distinct appearances: humanoid slots only; no cargo/health.

    Weapon furniture/optics are deliberately omitted, so reported counts are
    lower bounds on complete visible loadout variety, never inflated by ammo.
    """
    result = {()}
    if node["Include"]:
        result = product(result, visible(kits[node["Include"]], kits))
    groups = collections.defaultdict(list)
    for branch in node["Sets"]:
        groups[branch["ClassName"]].append(branch)
    for branches in groups.values():
        choices = set().union(*(visible(b, kits) for b in branches))
        result = product(result, choices)
    for slot in node["InventoryAttachments"]:
        result = product(result, {((slot["SlotName"], item["ClassName"]),) for item in slot["Items"]})
    return result


def item_values(stats, field="mean"):
    return {key[1]: value for key, value in getattr(stats, field).items() if key[0] == "item" and value}


def slot_values(stats, path):
    return {key[2]: value for key, value in stats.mean.items() if key[:2] == ("slot", path) and value}


def probabilities(values):
    return {str(k): {"Fraction": str(v), "Probability": round(float(v), 9)} for k, v in sorted(values.items(), key=lambda kv: str(kv[0]))}


def audit_loadouts(server):
    folder = server / "profiles/ExpansionMod/Loadouts"
    kits = {p.stem: json.loads(p.read_text(encoding="utf-8-sig")) for p in folder.glob("DI_*.json")}
    expected = {"DI_" + role for role in ARCHETYPES}
    expected |= {f"DI_{role}_{name}" for role, names in ARCHETYPES.items() for name in names}
    require(set(kits) == expected and len(kits) == 39, "Expected seven selectors and 32 archetypes")
    players = {"PlayerFemaleSuitLoadout", "PlayerMaleSuitLoadout", "PlayerSurvivorLoadout"}
    require({p.stem for p in folder.glob("*.json")} == expected | players, "Unexpected loadout files")
    catalog = {n.attrib["name"] for filename in ("db/types.xml", "modded_types/terje.xml")
               for n in ET.parse(server / "empty.deerisle" / filename).findall("type")}
    used = collections.defaultdict(set)
    reachable = set()

    def visit(name, stack=()):
        require(name in kits and name not in stack, f"Missing/cyclic include {stack + (name,)}")
        reachable.add(name)
        for n in walk(kits[name]):
            if n["Include"]:
                visit(n["Include"], stack + (name,))

    powered_classes = {"StunBaton", "Flashlight", "PersonalRadio", "Rangefinder", "M68Optic", "ReflexOptic"}
    for name, root in kits.items():
        require(root["ClassName"] == "" and root["Health"] == [], f"AI root must not change health: {name}")
        for n in walk(root):
            cls = n["ClassName"]
            require(n["Chance"] == 1, f"Eligibility is not a normalized weight: {name}/{cls}")
            require(not n["ConstructionPartsBuilt"], "Unexpected construction data")
            for supply in n["InventoryCargo"]:
                require(supply["Quantity"] == {"Min": .25 if supply["ClassName"] == "TetracyclineAntibiotics" else 1, "Max": .25 if supply["ClassName"] == "TetracyclineAntibiotics" else 1}, "Cargo not full")
            if not cls or cls in SYNTHETIC:
                require(not n["Health"] and n["Quantity"] == {"Min": 0, "Max": 0}, "Synthetic item changes health/quantity")
                continue
            require(cls in catalog and cls != "AugShort", f"Unavailable/disallowed class {cls}")
            require(not any(s in cls for s in ("Suppressor", "NVG", "Grenade", "Drum")), f"Forbidden equipment {cls}")
            require("Optic" not in cls or cls in {"M4_CarryHandleOptic", "BUISOptic", "M68Optic", "ReflexOptic", "ACOGOptic"}, f"Forbidden optic {cls}")
            require(n["Health"] == [{"Min": .6 if cls in AMMUNITION else .7, "Max": .9 if cls in AMMUNITION else 1, "Zone": ""}], f"Health for {name}/{cls}")
            qty = n["Quantity"]
            require(qty["Min"] == qty["Max"] and qty["Min"] in (0, .25, 1), f"Quantity {name}/{cls}")
            require(qty["Min"] != .25 or cls == "TetracyclineAntibiotics", "Unexpected partial supply")
            if cls.startswith(("Mag_", "Ammo_")) or cls in {"Battery9V", "BandageDressing", "SalineBagIV", "TetracyclineAntibiotics"}:
                require(qty["Min"] == (.25 if cls == "TetracyclineAntibiotics" else 1), f"Supply not full: {cls}")
            if cls in powered_classes:
                batteries = [i for s in n["InventoryAttachments"] if s["SlotName"] == "BatteryD" for i in s["Items"]]
                require(len(batteries) == 1 and batteries[0]["ClassName"] == "Battery9V", f"Unpowered {cls}")
            used[cls].add(name)

    role_reports, weapon_cases = [], []
    for role, weights in ARCHETYPES.items():
        selector_name = "DI_" + role
        visit(selector_name)
        selector = kits[selector_name]
        require(not selector["InventoryAttachments"] and not selector["InventoryCargo"] and not selector["Include"], f"Selector has equipment {role}")
        tickets = selector["Sets"]
        require(len(tickets) == 20 and all(t["ClassName"] == "ARCHETYPE" and not t["Sets"] and not t["InventoryAttachments"] and not t["InventoryCargo"] for t in tickets), f"Selector tickets {role}")
        counts = collections.Counter(t["Include"] for t in tickets)
        require(counts == {f"DI_{role}_{name}": weight // 5 for name, weight in weights.items()}, f"Archetype weights {role}")
        role_looks = set()
        for name in weights:
            full_name = f"DI_{role}_{name}"
            root = kits[full_name]
            require(all(not n["Include"] for n in walk(root)), "Archetypes must be self-contained")
            utility_sets = [n for n in root["Sets"] if n["ClassName"] == "UTILITY"]
            common_utilities = {
                "Civilian": [("TunaCan",), ("SodaCan_Cola",), ("Matchbox",), ("SewingKit",), ("Compass",), ("Heatpack",)],
                "Police": [("Handcuffs", "HandcuffKeys"), ("PersonalRadio", "Battery9V"), ("Flashlight", "Battery9V")],
                "Firefighter": [("DuctTape",), ("Heatpack",), ("Flashlight", "Battery9V"), ("BandageDressing",)],
                "Medic": [("TerjeBandageHemostatic",), ("DisinfectantAlcohol",), ("Heatpack",), ("TerjePlaster",)],
                "Doctor": [("TerjeStethoscope",), ("VitaminBottle",), ("DisinfectantAlcohol",), ("TerjeExpressTestAgents",)],
                "Militia": [("Compass",), ("CanOpener",), ("Matchbox",), ("DuctTape",), ("WeaponCleaningKit",), ("TunaCan",)],
            }
            nato_utilities = {
                "Rifleman": [("TunaCan",), ("Heatpack",)], "Pointman": [("Flashlight", "Battery9V"), ("DuctTape",)],
                "FieldMedic": [("TerjeBandageHemostatic",), ("SalineBagIV",)], "Engineer": [("WeaponCleaningKit",), ("ElectronicRepairKit",)],
                "TeamLeader": [("PersonalRadio", "Battery9V"), ("Binoculars",)], "Marksman": [("Rangefinder", "Battery9V"), ("Binoculars",)],
            }
            expected_bundles = nato_utilities[name] if role == "NATO" else common_utilities[role]
            observed_bundles = [tuple(sorted(n["ClassName"] for n in walk(b) if n["ClassName"] and n["ClassName"] not in SYNTHETIC)) for b in utility_sets]
            require(collections.Counter(observed_bundles) == collections.Counter(tuple(sorted(b)) for b in expected_bundles), f"Utility selection {full_name}")
            stats = analyze(root, kits)
            low, high = item_values(stats, "low"), item_values(stats, "high")
            outfits = visible(root, kits)
            role_looks.update(outfits)
            require(all(stats.low[("occupied", '/' + s)] == 1 for s in ("Body", "Legs", "Feet")), f"Missing basic clothing {full_name}")
            bags = slot_values(stats, "/Back")
            absent = 1 - sum(bags.values(), Fraction())
            if absent:
                bags[None] = absent
            require(bags == {k: Fraction(v, 100) for k, v in BAGS[role].items()}, f"Bag probabilities {full_name}")
            minimum_bandages = 2 if role in ("Doctor", "Medic") else 1
            require(low.get("BandageDressing", 0) == minimum_bandages, f"Bandage minimum {full_name}")
            if role in ("Civilian", "Doctor", "Medic", "Firefighter"):
                require(not set(high) & set(AMMUNITION), f"Unexpected firearm {full_name}")
            if role in ("Civilian", "Doctor", "Medic"):
                require(not any(slot_values(stats, '/' + s) for s in ("Vest", "Shoulder", "Hands", "Melee")), f"Unarmed/armor policy {full_name}")
            if role == "Militia":
                require(not slot_values(stats, "/Vest"), "Armored militia")
                allowed_tops = {"HuntingJacket_Brown", "HuntingJacket_Autumn", "HuntingJacket_Spring", "HuntingJacket_Summer", "Shirt_GreenCheck", "Shirt_PlaneBlack", "Hoodie_Brown", "Hoodie_Green", "Hoodie_Grey", "HikingJacket_Black", "HikingJacket_Green"}
                require(set(slot_values(stats, "/Body")) <= allowed_tops, "Militia earth-tone palette")
            if role == "Civilian" and name == "Worker":
                require(all("Check" in c for c in slot_values(stats, "/Body")), "Worker checked shirts")
            if role in ("Medic", "Doctor"):
                require(low.get("TerjeBandageHemostatic", 0) == 1 and low.get("SalineBagIV", 0) == 1, f"Medical baseline {full_name}")
                for outfit in outfits:
                    look = dict(outfit)
                    color = look["Body"].rsplit('_', 1)[1]
                    require(look["Legs"].endswith('_' + color), "Mismatched medical uniform")
                    if role == "Doctor":
                        require(look["Headgear"].endswith('_' + color), "Mismatched scrubs hat")
                if role == "Doctor":
                    for cls, count in {"TerjeSurgicalKit": 1, "TerjeAmpouleFlemoclav": 1, "TerjeAmpouleNovacaine": 1, "TerjeSyringeEmpty": 2, "TetracyclineAntibiotics": 1, "HipPack_Medical": 1, "SurgicalMask": 1}.items():
                        require(low.get(cls, 0) == count, f"Missing doctor supply {cls}/{name}")
            if role == "Police":
                require(low.get("PoliceVest") == 1 and low.get("StunBaton") == 1, "Police identity")
                require(not slot_values(stats, "/Headgear").keys() - {"PoliceCap", "DirtBikeHelmet_Police"}, "Police headgear")
                require(name != "Traffic" or slot_values(stats, "/Headgear") == {"DirtBikeHelmet_Police": 1}, "Traffic helmet")
                for outfit in outfits:
                    look = dict(outfit)
                    require((look["Body"], look["Legs"]) in {("PoliceJacket", "PolicePants"), ("PoliceJacketOrel", "PolicePantsOrel")}, "Mismatched police uniform")
            if role == "Firefighter":
                require(low.get("ReflexVest") == 1, "Missing firefighter visibility")
                expected_tools = {"FireCrew": {"FirefighterAxe", "FirefighterAxe_Black", "FirefighterAxe_Green"}, "RescueCrew": {"FirefighterAxe", "WoodAxe", "Crowbar"}, "StationCrew": {"FirefighterAxe", "Crowbar"}}[name]
                require(set(slot_values(stats, "/Melee")) == expected_tools, "Rescue tool pool")
                for outfit in outfits:
                    look = dict(outfit)
                    require(look["Body"].rsplit('_', 1)[1] == look["Legs"].rsplit('_', 1)[1], "Mismatched firefighter uniform")
            if role == "NATO":
                require(low.get("USMCJacket_Woodland") == 1 and low.get("USMCPants_Woodland") == 1, "NATO uniform")
                allowed_vests = {"PlateCarrierVest_Green", "PlateCarrierVest_Camo"} if name == "Marksman" else {"UKAssVest_Olive", "UKAssVest_Khaki", "UKAssVest_Camo"}
                require(set(slot_values(stats, "/Vest")) == allowed_vests, "Specialist armor policy")
                if name == "Marksman":
                    for vest in next(s["Items"] for s in root["InventoryAttachments"] if s["SlotName"] == "Vest"):
                        pouches = [i["ClassName"] for s in vest["InventoryAttachments"] for i in s["Items"]]
                        require(pouches == [vest["ClassName"].replace("Vest", "Pouches")], "Mismatched carrier pouches")
            weapon_groups = [n for n in root["Sets"] if n["ClassName"] == "WEAPON"]
            pool = WEAPONS.get(role, {}).get(name, [])
            require(len(weapon_groups) == len(pool), f"Weapon pool size {full_name}")
            observed = []
            for branch in weapon_groups:
                branch_stats = analyze(branch, kits)
                possible = set(item_values(branch_stats, "high")) & set(AMMUNITION)
                require(len(possible) == 1, "Weapon branch must contain exactly one firearm")
                weapon = possible.pop()
                observed.append(weapon)
                if weapon == "M4A1":
                    furniture = [n for n in walk(branch) if n["ClassName"] == "FURNITURE"]
                    observed_furniture = {tuple(sorted(n["ClassName"] for n in walk(f) if n["ClassName"] not in SYNTHETIC)) for f in furniture}
                    expected_furniture = {tuple(sorted(p)) for p in [("M4_PlasticHndgrd", "M4_OEBttstck"), ("M4_RISHndgrd", "M4_CQBBttstck"), ("M4_MPHndgrd", "M4_MPBttstck")]}
                    require(len(furniture) == 3 and observed_furniture == expected_furniture, "M4 furniture pairing")
                ammunition = AMMUNITION[weapon]
                ammunition_counts = {k: v for k, v in item_values(branch_stats, "high").items() if k.startswith(("Mag_", "Ammo_"))}
                require(ammunition_counts == {ammunition: 2} and branch_stats.low[("item", ammunition)] == 2, f"Ammunition on path {full_name}/{weapon}")
                attached = sum(v for k, v in branch_stats.mean.items() if k[0] == "slot" and k[1].endswith('/magazine') and k[2] == ammunition)
                require(attached == int(ammunition.startswith("Mag_")), f"Attached magazine {weapon}")
                optics = {k: v for k, v in item_values(branch_stats).items() if k in {"M4_CarryHandleOptic", "BUISOptic", "M68Optic", "ReflexOptic", "ACOGOptic"}}
                expected_optics = {}
                if role == "NATO" and name == "Marksman":
                    expected_optics = {"ACOGOptic": Fraction(1)}
                elif weapon in ("M4A1", "Aug"):
                    expected_optics = {"M68Optic": Fraction(1, 2), "ReflexOptic": Fraction(1, 2)} if name == "TeamLeader" else {"BUISOptic": Fraction(1)} if weapon == "Aug" else {"M4_CarryHandleOptic": Fraction(1, 2), "M68Optic": Fraction(1, 4), "ReflexOptic": Fraction(1, 4)}
                require(optics == expected_optics, f"Optic policy {full_name}/{weapon}")
                weapon_cases.append({"Archetype": full_name, "Weapon": weapon, "ConditionalProbability": str(Fraction(1, len(pool))), "Ammunition": ammunition, "AttachedMagazines": int(attached), "SpareMagazines": int(attached), "LooseFullStacks": 0 if attached else 2, "Optics": probabilities(optics)})
            require(set(observed) == set(pool), f"Weapon allowlist {full_name}")
        overall = analyze(selector, kits)
        expected_weapons = collections.Counter()
        for name, weapons in WEAPONS.get(role, {}).items():
            for weapon in weapons:
                expected_weapons[weapon] += Fraction(weights[name], 100 * len(weapons))
        actual_weapons = {k: v for k, v in item_values(overall).items() if k in AMMUNITION}
        require(actual_weapons == expected_weapons, f"Aggregate weapon probabilities {role}")
        threshold = 500 if role == "NATO" else 24
        require(len(role_looks) >= threshold, f"Insufficient visible variety {role}: {len(role_looks)}")
        bags = slot_values(overall, "/Back")
        if sum(bags.values(), Fraction()) < 1:
            bags["None"] = 1 - sum(bags.values(), Fraction())
        role_reports.append({"Role": role, "Archetypes": {name: weight / 100 for name, weight in weights.items()},
                             "VisibleCombinationsLowerBound": len(role_looks), "MinimumRequired": threshold,
                             "WeaponProbabilities": probabilities(actual_weapons), "BagProbabilities": probabilities(bags),
                             "GuaranteedItemCounts": dict(sorted(item_values(overall, "low").items()))})
        if role == "NATO":
            require(actual_weapons["FAL"] == actual_weapons["M14"] == Fraction(1, 20), "Battle rifle incidence")
            require(overall.mean[("item", "ACOGOptic")] == Fraction(1, 10), "Magnified optic incidence")
            require(sum(slot_values(overall, "/Vest").get(c, 0) for c in ("PlateCarrierVest_Green", "PlateCarrierVest_Camo")) == Fraction(1, 10), "Plate carrier incidence")
    require(reachable == expected, "Unreachable AI loadouts")
    return {"AIFileCount": len(kits), "RoleSelectors": 7, "Archetypes": 32, "ReachableAIFileCount": len(reachable),
            "InventoryClassCount": len(used), "Roles": role_reports, "WeaponCases": weapon_cases,
            "SpecialistNATOProbability": .1, "AugShortPolicy": "Excluded; standard Aug uses BUIS, TeamLeader Aug uses powered red dot",
            "ProbabilityModel": "Exact rational ticket/slot probabilities assuming successful native item creation",
            "VisualModel": "Distinct humanoid slot item classes; ignores cargo, health, quantity, weapon furniture and optics",
            "Inventory": [{"ClassName": cls, "Archetypes": sorted(names)} for cls, names in sorted(used.items())]}
