"""Selection-analysis regression tests; no server configuration is modified."""
import copy
from fractions import Fraction
import unittest

from audit_ai_loadouts import analyze, visible


def node(cls="", *, include="", cargo=(), slots=None, sets=()):
    return {"ClassName": cls, "Include": include, "Chance": 1,
            "InventoryCargo": list(cargo), "InventoryAttachments": [
                {"SlotName": slot, "Items": items} for slot, items in (slots or {}).items()],
            "Sets": list(sets)}


class SelectionTests(unittest.TestCase):
    def test_repeated_tickets_weight_includes_without_duplicate_equipment(self):
        a = node("ARCHETYPE", include="A")
        b = node("ARCHETYPE", include="B")
        kits = {"A": node(slots={"Body": [node("ShirtA")]}),
                "B": node(slots={"Body": [node("ShirtB")]})}
        root = node(sets=[a, copy.deepcopy(a), copy.deepcopy(a), b])
        result = analyze(root, kits)
        self.assertEqual(result.mean[("item", "ShirtA")], Fraction(3, 4))
        self.assertEqual(result.mean[("item", "ShirtB")], Fraction(1, 4))
        self.assertEqual(result.high[("occupied", "/Body")], 1)
        self.assertEqual(len(visible(root, kits)), 2)

    def test_repeated_attachment_candidates_have_exact_odds(self):
        root = node(slots={"Headgear": [node("HatA"), node("HatA"), node("HatB")]})
        result = analyze(root, {})
        self.assertEqual(result.mean[("item", "HatA")], Fraction(2, 3))
        self.assertEqual(result.high[("occupied", "/Headgear")], 1)

    def test_empty_ticket_represents_absence(self):
        root = node(sets=[node("BAG"), node("BAG"), node("BAG", slots={"Back": [node("Bag")]})])
        result = analyze(root, {})
        self.assertEqual(result.low[("occupied", "/Back")], 0)
        self.assertEqual(result.mean[("item", "Bag")], Fraction(1, 3))
        self.assertEqual(len(visible(root, {})), 2)

    def test_correlated_uniforms_do_not_cross_mix(self):
        root = node(sets=[node("UNIFORM", slots={"Body": [node("BlueShirt")], "Legs": [node("BluePants")]}),
                          node("UNIFORM", slots={"Body": [node("WhiteShirt")], "Legs": [node("WhitePants")]})])
        self.assertEqual(len(visible(root, {})), 2)
        self.assertEqual(analyze(root, {}).high[("occupied", "/Body")], 1)

    def test_alternative_ammunition_is_not_summed_as_guaranteed_inventory(self):
        root = node(sets=[node("WEAPON", cargo=[node("AmmoA"), node("AmmoA")]),
                          node("WEAPON", cargo=[node("AmmoB"), node("AmmoB")])])
        result = analyze(root, {})
        self.assertEqual(result.low[("item", "AmmoA")], 0)
        self.assertEqual(result.high[("item", "AmmoA")], 2)
        self.assertEqual(result.mean[("item", "AmmoA")], 1)

    def test_independent_slots_multiply_visible_combinations(self):
        root = node(slots={"Feet": [node("ShoeA"), node("ShoeB")],
                           "Gloves": [node("GloveA"), node("GloveB"), node("GloveC")]})
        self.assertEqual(len(visible(root, {})), 6)

    def test_independent_groups_cannot_own_the_same_slot(self):
        root = node(sets=[node("UNIFORM", slots={"Body": [node("ShirtA")]}),
                          node("UTILITY", slots={"Body": [node("ShirtB")]})])
        with self.assertRaisesRegex(AssertionError, "Multiple owners"):
            analyze(root, {})

    def test_missing_include_is_rejected(self):
        with self.assertRaisesRegex(AssertionError, "Missing include"):
            analyze(node(include="Missing"), {})

    def test_indirect_include_cycle_is_rejected(self):
        kits = {"A": node(include="B"), "B": node(include="C"), "C": node(include="A")}
        with self.assertRaisesRegex(AssertionError, "Include cycle"):
            analyze(node(include="A"), kits)

    def test_fractional_eligibility_cannot_be_misread_as_weight(self):
        root = node()
        root["Chance"] = .5
        with self.assertRaisesRegex(AssertionError, "Chance=1"):
            analyze(root, {})


if __name__ == "__main__":
    unittest.main()
