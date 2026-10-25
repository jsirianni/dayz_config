# Firearms, Magazines, Attachments, And Ammo

Reviewed: 2026-05-31

Primary source: `empty.deerisle/db/types.xml`

Supporting source: `empty.deerisle/cfgspawnabletypes.xml`

This document lists firearm-related loot entries present in `types.xml`, grouped by firearm family. `cfgspawnabletypes.xml` was used only to confirm which magazines or parts are spawned attached to specific firearms when that relationship is visible locally.

Notes:

- Classname variants are grouped when they share the same apparent compatibility.
- `AmmoBox_*` entries are listed with their matching loose `Ammo_*` classnames.
- Several modded firearms exist in `types.xml` without a matching weapon entry in `cfgspawnabletypes.xml`; those are marked as unconfirmed instead of inventing compatibility.
- Some attachments referenced by `cfgspawnabletypes.xml` are not present in `types.xml`. Those are called out near the end because they can matter for spawned-in weapons but are not loot-economy entries in `types.xml`.

## Ammo Families

- `.22 LR`
  - `Ammo_22`
  - `AmmoBox_22_50Rnd`
- `.308 Win`
  - `Ammo_308Win`
  - `Ammo_308WinTracer`
  - `AmmoBox_308Win_20Rnd`
  - `AmmoBox_308WinTracer_20Rnd`
- `.357`
  - `Ammo_357`
  - `AmmoBox_357_20Rnd`
- `.380 ACP`
  - `Ammo_380`
  - `AmmoBox_380_35rnd`
- `.45 ACP`
  - `Ammo_45ACP`
  - `AmmoBox_45ACP_25rnd`
- `5.45x39mm`
  - `Ammo_545x39`
  - `Ammo_545x39Tracer`
  - `AmmoBox_545x39_20Rnd`
  - `AmmoBox_545x39Tracer_20Rnd`
- `5.56x45mm`
  - `Ammo_556x45`
  - `Ammo_556x45Tracer`
  - `AmmoBox_556x45_20Rnd`
  - `AmmoBox_556x45Tracer_20Rnd`
- `7.62x39mm`
  - `Ammo_762x39`
  - `Ammo_762x39Tracer`
  - `AmmoBox_762x39_20Rnd`
  - `AmmoBox_762x39Tracer_20Rnd`
- `7.62x54mmR`
  - `Ammo_762x54`
  - `Ammo_762x54Tracer`
  - `AmmoBox_762x54_20Rnd`
  - `AmmoBox_762x54Tracer_20Rnd`
- `9x19mm`
  - `Ammo_9x19`
  - `AmmoBox_9x19_25rnd`
- `9x39mm`
  - `Ammo_9x39`
  - `Ammo_9x39AP`
  - `AmmoBox_9x39_20Rnd`
  - `AmmoBox_9x39AP_20Rnd`
- `12 gauge`
  - `Ammo_12gaPellets`
  - `Ammo_12gaRubberSlug`
  - `Ammo_12gaSlug`
  - `AmmoBox_00buck_10rnd`
  - `AmmoBox_12gaRubberSlug_10Rnd`
  - `AmmoBox_12gaSlug_10Rnd`
- `40mm launcher rounds`
  - `Ammo_40mm_ChemGas`
  - `Ammo_40mm_Chemgas`
  - `Ammo_40mm_Explosive`
  - `Ammo_40mm_Smoke_Black`
  - `Ammo_40mm_Smoke_Green`
  - `Ammo_40mm_Smoke_Red`
  - `Ammo_40mm_Smoke_White`
- `Flares`
  - `Ammo_Flare`
  - `Ammo_FlareBlue`
  - `Ammo_FlareGreen`
  - `Ammo_FlareRed`
- `Bolts`
  - `Ammo_CupidsBolt`
  - `Ammo_HuntingBolt`
  - `Ammo_ImprovisedBolt_1`
  - `Ammo_ImprovisedBolt_2`
- `JMC Snow Owl ammunition`
  - `jmc_SnowOwl_Ammo`
  - `STAG_AmmoBox_SnowOwl`

## Firearm Families

### AK101

Firearms:

- `AK101`
- `AK101_Black`
- `AK101_Green`

Parts and accessories:

- `AK_Bayonet`
- `AK_FoldingBttstck`
- `AK_FoldingBttstck_Black`
- `AK_FoldingBttstck_Green`
- `AK_PlasticBttstck`
- `AK_PlasticBttstck_Black`
- `AK_PlasticBttstck_Green`
- `AK_PlasticHndgrd`
- `AK_RailHndgrd`
- `AK_RailHndgrd_Black`
- `AK_RailHndgrd_Green`
- `AK_Suppressor`
- `AK_WoodBttstck`
- `AK_WoodBttstck_Black`
- `AK_WoodBttstck_Camo`
- `AK_WoodHndgrd`
- `AK_WoodHndgrd_Black`
- `AK_WoodHndgrd_Camo`
- `STG_AK_RIS_Adapter`
- `STG_AK_Stck_Adapter`
- `STG_PBS5_Suppressor`
- `KashtanOptic`
- `KobraOptic`
- `KazuarOptic`
- `PSO1Optic`
- `PSO11Optic`
- `PSO6Optic`

Magazines:

- `Mag_AK101_30Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### AK74

Firearms:

- `AK74`
- `AK74_Black`
- `AK74_Green`

Parts and accessories:

- `AK74_Hndgrd`
- `AK74_WoodBttstck`
- AK-family furniture, muzzle, bayonet, and optics listed under `AK101`

Magazines:

- `Mag_AK74_30Rnd`
- `Mag_AK74_45Rnd`

Ammo:

- `Ammo_545x39`
- `Ammo_545x39Tracer`
- `AmmoBox_545x39_20Rnd`
- `AmmoBox_545x39Tracer_20Rnd`

### AKS74U

Firearms:

- `AKS74U`
- `AKS74U_Black`
- `AKS74U_Green`

Parts and accessories:

- `AKS74U_Bttstck`
- AK-family magazines and selected AK-family parts may apply depending on server-side weapon config

Magazines:

- `Mag_AK74_30Rnd`
- `Mag_AK74_45Rnd`

Ammo:

- `Ammo_545x39`
- `Ammo_545x39Tracer`
- `AmmoBox_545x39_20Rnd`
- `AmmoBox_545x39Tracer_20Rnd`

### AKM

Firearms:

- `AKM`

Parts and accessories:

- AK-family furniture, muzzle, bayonet, and optics listed under `AK101`

Magazines:

- `Mag_AKM_30Rnd`
- `Mag_AKM_Drum75Rnd`
- `Mag_AKM_Palm30Rnd`

Ammo:

- `Ammo_762x39`
- `Ammo_762x39Tracer`
- `AmmoBox_762x39_20Rnd`
- `AmmoBox_762x39Tracer_20Rnd`

### M4A1

Firearms:

- `M4A1`
- `M4A1_Black`
- `M4A1_Green`

Parts and accessories:

- `M4_CarryHandleOptic`
- `M4_CQBBttstck`
- `M4_MPBttstck`
- `M4_MPHndgrd`
- `M4_OEBttstck`
- `M4_PlasticHndgrd`
- `M4_RISHndgrd`
- `M4_RISHndgrd_Black`
- `M4_RISHndgrd_Green`
- `M4_Suppressor`
- `M4_T3NRDSOptic`
- `M9A1_Bayonet`
- `ACOGOptic`
- `ACOGOptic_6x`
- `BUISOptic`
- `M68Optic`
- `ReflexOptic`
- `TLRLight`
- `UniversalLight`

Magazines:

- `Mag_CMAG_10Rnd`
- `Mag_CMAG_20Rnd`
- `Mag_CMAG_30Rnd`
- `Mag_CMAG_40Rnd`
- `Mag_STANAG_30Rnd`
- `Mag_STANAG_60Rnd`
- `Mag_STANAGCoupled_30Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### M16A2

Firearms:

- `M16A2`

Parts and accessories:

- No dedicated M16 parts are listed in `types.xml`

Magazines:

- `Mag_CMAG_10Rnd`
- `Mag_CMAG_20Rnd`
- `Mag_CMAG_30Rnd`
- `Mag_CMAG_40Rnd`
- `Mag_STANAG_30Rnd`
- `Mag_STANAG_60Rnd`
- `Mag_STANAGCoupled_30Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### AUG

Firearms:

- `Aug`
- `AugShort`

Parts and accessories:

- `ACOGOptic`
- `BUISOptic`
- `M4_T3NRDSOptic`
- `ReflexOptic`
- `TLRLight`

Magazines:

- `Mag_Aug_30Rnd`
- `Mag_CMAG_40Rnd`
- `Mag_STANAG_60Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### FAMAS

Firearms:

- `FAMAS`
- `SawedoffFAMAS`

Parts and accessories:

- No dedicated FAMAS parts are listed in `types.xml`

Magazines:

- `Mag_FAMAS_25Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### FAL

Firearms:

- `FAL`

Parts and accessories:

- `Fal_FoldingBttstck`
- `Fal_OeBttstck`
- `ACOGOptic`
- `BUISOptic`
- `M4_T3NRDSOptic`
- `M68Optic`
- `ReflexOptic`

Magazines:

- `Mag_FAL_20Rnd`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### M14

Firearms:

- `M14`

Parts and accessories:

- `ACOGOptic`
- `ACOGOptic_6x`
- `M4_T3NRDSOptic`
- `ReflexOptic`

Magazines:

- `Mag_M14_10Rnd`
- `Mag_M14_20Rnd`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### SVD

Firearms:

- `SVD`
- `SVD_Wooden`

Parts and accessories:

- `KashtanOptic`
- `KazuarOptic`
- `KobraOptic`
- `PSO1Optic`
- `PSO11Optic`
- `PSO6Optic`

Magazines:

- `Mag_SVD_10Rnd`

Ammo:

- `Ammo_762x54`
- `Ammo_762x54Tracer`
- `AmmoBox_762x54_20Rnd`
- `AmmoBox_762x54Tracer_20Rnd`

### SV98

Firearms:

- `SV98`

Parts and accessories:

- No SV98-specific attachment relationship is visible in local XML

Magazines:

- `Mag_SV98_10Rnd`

Ammo:

- `Ammo_762x54`
- `Ammo_762x54Tracer`
- `AmmoBox_762x54_20Rnd`
- `AmmoBox_762x54Tracer_20Rnd`

### VSS

Firearms:

- `VSS`

Parts and accessories:

- `KashtanOptic`
- `PSO1Optic`
- `PSO11Optic`

Magazines:

- `Mag_VSS_10Rnd`

Ammo:

- `Ammo_9x39`
- `Ammo_9x39AP`
- `AmmoBox_9x39_20Rnd`
- `AmmoBox_9x39AP_20Rnd`

### ASVAL

Firearms:

- `ASVAL`

Parts and accessories:

- `ACOGOptic`
- `BUISOptic`
- `M4_T3NRDSOptic`
- `ReflexOptic`
- `TLRLight`

Magazines:

- `Mag_VAL_20Rnd`

Ammo:

- `Ammo_9x39`
- `Ammo_9x39AP`
- `AmmoBox_9x39_20Rnd`
- `AmmoBox_9x39AP_20Rnd`

### Vikhr

Firearms:

- `Vikhr`

Parts and accessories:

- No Vikhr-specific attachment relationship is visible in local XML

Magazines:

- `Mag_Vikhr_30Rnd`

Ammo:

- `Ammo_9x39`
- `Ammo_9x39AP`
- `AmmoBox_9x39_20Rnd`
- `AmmoBox_9x39AP_20Rnd`

### Mosin 9130

Firearms:

- `Mosin9130`
- `Mosin9130_Black`
- `Mosin9130_Camo`
- `Mosin9130_Green`
- `SawedoffMosin9130`
- `SawedoffMosin9130_Black`
- `SawedoffMosin9130_Camo`
- `SawedoffMosin9130_Green`

Parts and accessories:

- `Mosin_Bayonet`
- `Mosin_Compensator`
- `PUScopeOptic`

Magazines:

- Internal magazine; no detachable Mosin magazine in `types.xml`

Ammo:

- `Ammo_762x54`
- `Ammo_762x54Tracer`
- `AmmoBox_762x54_20Rnd`
- `AmmoBox_762x54Tracer_20Rnd`

### SKS

Firearms:

- `SKS`

Parts and accessories:

- `SKS_Bayonet`
- `PUScopeOptic`

Magazines:

- Internal magazine; no detachable SKS magazine in `types.xml`

Ammo:

- `Ammo_762x39`
- `Ammo_762x39Tracer`
- `AmmoBox_762x39_20Rnd`
- `AmmoBox_762x39Tracer_20Rnd`

### CZ527

Firearms:

- `CZ527`

Parts and accessories:

- `HuntingOptic`

Magazines:

- `Mag_CZ527_5rnd`

Ammo:

- `Ammo_762x39`
- `Ammo_762x39Tracer`
- `AmmoBox_762x39_20Rnd`
- `AmmoBox_762x39Tracer_20Rnd`

### CZ550

Firearms:

- `CZ550`

Parts and accessories:

- `HuntingOptic`

Magazines:

- `Mag_CZ550_10Rnd`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### B95 / Blaze

Firearms:

- `B95`
- `SawedoffB95`

Parts and accessories:

- `HuntingOptic`

Magazines:

- Internal chambers; no detachable magazine in `types.xml`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### Winchester 70

Firearms:

- `Winchester70`

Parts and accessories:

- `HuntingOptic`

Magazines:

- Internal magazine; no detachable Winchester magazine in `types.xml`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### Scout

Firearms:

- `Scout`
- `Scout_Chernarus`
- `Scout_Livonia`

Parts and accessories:

- `ACOGOptic`
- `ACOGOptic_6x`
- `M4_Suppressor`
- `M4_T3NRDSOptic`
- `M68Optic`

Magazines:

- `Mag_Scout_5Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### SSG82

Firearms:

- `SSG82`

Parts and accessories:

- No SSG82 optic entry is present in `types.xml`

Magazines:

- `Mag_SSG82_5rnd`

Ammo:

- `Ammo_545x39`
- `Ammo_545x39Tracer`
- `AmmoBox_545x39_20Rnd`
- `AmmoBox_545x39Tracer_20Rnd`

### Izh18 / BK-18

Firearms:

- `Izh18`
- `SawedoffIzh18`

Parts and accessories:

- `HuntingOptic`

Magazines:

- Single-shot internal chamber; no detachable magazine in `types.xml`

Ammo:

- `Ammo_762x39`
- `Ammo_762x39Tracer`
- `AmmoBox_762x39_20Rnd`
- `AmmoBox_762x39Tracer_20Rnd`

### Ruger 10/22

Firearms:

- `Ruger1022`

Parts and accessories:

- `HuntingOptic`

Magazines:

- `Mag_Ruger1022_15Rnd`
- `Mag_Ruger1022_30Rnd`

Ammo:

- `Ammo_22`
- `AmmoBox_22_50Rnd`

### MKII

Firearms:

- `MKII`

Parts and accessories:

- Integral suppressor; no separate MKII suppressor entry in `types.xml`

Magazines:

- `Mag_MKII_10Rnd`

Ammo:

- `Ammo_22`
- `AmmoBox_22_50Rnd`

### Repeater

Firearms:

- `Repeater`

Parts and accessories:

- No Repeater-specific attachment relationship is visible in local XML

Magazines:

- Internal tube magazine; no detachable magazine in `types.xml`

Ammo:

- `Ammo_357`
- `AmmoBox_357_20Rnd`

### Longhorn

Firearms:

- `Longhorn`

Parts and accessories:

- `PistolOptic`

Magazines:

- Single-shot internal chamber; no detachable magazine in `types.xml`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### Magnum

Firearms:

- `Magnum`
- `SawedoffMagnum`

Parts and accessories:

- No Magnum-specific attachment relationship is visible in local XML

Magazines:

- Internal cylinder; no detachable magazine in `types.xml`

Ammo:

- `Ammo_357`
- `AmmoBox_357_20Rnd`

### Derringer

Firearms:

- `Derringer_Black`
- `Derringer_Grey`
- `Derringer_Pink`

Parts and accessories:

- No Derringer-specific attachment relationship is visible in local XML

Magazines:

- Internal chambers; no detachable magazine in `types.xml`

Ammo:

- `Ammo_357`
- `AmmoBox_357_20Rnd`

### Deagle

Firearms:

- `Deagle`
- `Deagle_Gold`

Parts and accessories:

- `PistolOptic`

Magazines:

- `Mag_Deagle_9rnd`

Ammo:

- `Ammo_357`
- `AmmoBox_357_20Rnd`

### FNX45

Firearms:

- `FNX45`

Parts and accessories:

- `FNP45_MRDSOptic`
- `PistolSuppressor`
- `TLRLight`

Magazines:

- `Mag_FNX45_15Rnd`

Ammo:

- `Ammo_45ACP`
- `AmmoBox_45ACP_25rnd`

### Colt 1911

Firearms:

- `Colt1911`
- `Engraved1911`

Parts and accessories:

- `PistolSuppressor`

Magazines:

- `Mag_1911_7Rnd`

Ammo:

- `Ammo_45ACP`
- `AmmoBox_45ACP_25rnd`

### Glock 19

Firearms:

- `Glock19`

Parts and accessories:

- `PistolSuppressor`
- `TLRLight`

Magazines:

- `Mag_Glock_15Rnd`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### CZ75

Firearms:

- `CZ75`

Parts and accessories:

- `PistolSuppressor`
- `TLRLight`

Magazines:

- `Mag_CZ75_15Rnd`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### P1

Firearms:

- `P1`

Parts and accessories:

- `PistolSuppressor`

Magazines:

- `Mag_P1_8Rnd`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### Makarov IJ70

Firearms:

- `MakarovIJ70`

Parts and accessories:

- No Makarov suppressor entry is present in `types.xml`

Magazines:

- `Mag_IJ70_8Rnd`

Ammo:

- `Ammo_380`
- `AmmoBox_380_35rnd`

### CZ61 Skorpion

Firearms:

- `CZ61`

Parts and accessories:

- `PistolSuppressor`

Magazines:

- `Mag_CZ61_20Rnd`

Ammo:

- `Ammo_380`
- `AmmoBox_380_35rnd`

### MP5K

Firearms:

- `MP5K`

Parts and accessories:

- `MP5_Compensator`
- `MP5_PlasticHndgrd`
- `MP5_RailHndgrd`
- `MP5k_StockBttstck`
- `ACOGOptic`
- `BUISOptic`
- `M4_T3NRDSOptic`
- `M68Optic`
- `PistolSuppressor`
- `ReflexOptic`

Magazines:

- `Mag_MP5_15Rnd`
- `Mag_MP5_30Rnd`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### UMP45

Firearms:

- `UMP45`

Parts and accessories:

- `ACOGOptic`
- `BUISOptic`
- `M4_T3NRDSOptic`
- `M68Optic`
- `PistolSuppressor`
- `ReflexOptic`
- `TLRLight`

Magazines:

- `Mag_UMP_25Rnd`

Ammo:

- `Ammo_45ACP`
- `AmmoBox_45ACP_25rnd`

### PP19 Bizon

Firearms:

- `PP19`

Parts and accessories:

- `PP19_Bttstck`
- `KobraOptic`
- `PSO1Optic`
- `PSO11Optic`
- `PistolSuppressor`

Magazines:

- `Mag_PP19_64Rnd`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### PM73 RAK

Firearms:

- `PM73Rak`

Parts and accessories:

- No PM73-specific attachment relationship is visible in local XML

Magazines:

- `Mag_PM73_15Rnd`
- `Mag_PM73_25Rnd`

Ammo:

- `Ammo_380`
- `AmmoBox_380_35rnd`

### 12-Gauge Shotguns

Firearms:

- `Izh18Shotgun`
- `Izh43Shotgun`
- `Mp133Shotgun`
- `SawedoffIzh18Shotgun`
- `SawedoffIzh43Shotgun`
- `tt870`

Parts and accessories:

- No common detachable shotgun parts are listed for these classnames in local XML

Magazines:

- Internal chamber or tube magazine; no detachable magazine in `types.xml`

Ammo:

- `Ammo_12gaPellets`
- `Ammo_12gaRubberSlug`
- `Ammo_12gaSlug`
- `AmmoBox_00buck_10rnd`
- `AmmoBox_12gaRubberSlug_10Rnd`
- `AmmoBox_12gaSlug_10Rnd`

### Saiga

Firearms:

- `Saiga`

Parts and accessories:

- `Saiga_Bttstck`

Magazines:

- `Mag_Saiga_5Rnd`
- `Mag_Saiga_8Rnd`
- `Mag_Saiga_Drum20Rnd`

Ammo:

- `Ammo_12gaPellets`
- `Ammo_12gaRubberSlug`
- `Ammo_12gaSlug`
- `AmmoBox_00buck_10rnd`
- `AmmoBox_12gaRubberSlug_10Rnd`
- `AmmoBox_12gaSlug_10Rnd`

### R12

Firearms:

- `R12`

Parts and accessories:

- `ReflexOptic`
- `UniversalLight`

Magazines:

- No R12 magazine entry is present in `types.xml`; compatibility is not confirmed by local XML

Ammo:

- Likely `12 gauge`, but no local magazine or ammo relationship is exposed in XML

### M79 Grenade Launcher

Firearms:

- `M79`

Parts and accessories:

- No M79-specific attachment relationship is visible in local XML

Ammo:

- `Ammo_40mm_ChemGas`
- `Ammo_40mm_Chemgas`
- `Ammo_40mm_Explosive`
- `Ammo_40mm_Smoke_Black`
- `Ammo_40mm_Smoke_Green`
- `Ammo_40mm_Smoke_Red`
- `Ammo_40mm_Smoke_White`

### Flaregun

Firearms:

- `Flaregun`

Parts and accessories:

- No Flaregun-specific attachment relationship is visible in local XML

Ammo:

- `Ammo_Flare`
- `Ammo_FlareBlue`
- `Ammo_FlareGreen`
- `Ammo_FlareRed`

## Ranged Non-Firearm: Crossbows

Firearms/ranged weapons:

- `Crossbow_Autumn`
- `Crossbow_Black`
- `Crossbow_Summer`
- `Crossbow_Wood`

Parts and accessories:

- No crossbow-specific optic or accessory entry is visible in local XML

Ammo:

- `Ammo_CupidsBolt`
- `Ammo_HuntingBolt`
- `Ammo_ImprovisedBolt_1`
- `Ammo_ImprovisedBolt_2`

## Modded Firearms With Confirmed Local Relationships

### aek

Firearms:

- `aek`

Parts and accessories:

- `ACOGOptic`
- `M4_T3NRDSOptic`
- `ReflexOptic`

Magazines:

- `Mag_AK101_30Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### junkak

Firearms:

- `junkak`

Parts and accessories:

- No junk AK-specific furniture entries are present in `types.xml`

Magazines:

- `Mag_AKM_30Rnd`
- `Mag_AKM_Drum75Rnd`
- `Mag_AKM_Palm30Rnd`

Ammo:

- `Ammo_762x39`
- `Ammo_762x39Tracer`
- `AmmoBox_762x39_20Rnd`
- `AmmoBox_762x39Tracer_20Rnd`

### ESP_G36C

Firearms:

- `ESP_G36C`

Parts and accessories:

- `ACOGOptic`
- `M4_T3NRDSOptic`
- `ReflexOptic`
- `TLRLight`

Magazines:

- `Mag_STANAG_30Rnd`
- Likely also compatible with other STANAG/CMAG magazines, but only `Mag_STANAG_30Rnd` is shown in `cfgspawnabletypes.xml`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### ESP_M110

Firearms:

- `ESP_M110`
- `ESP_M110_Green`
- `ESP_M110_Tan`

Parts and accessories:

- `ESP_Hunter`
- `ESP_Long_Range`
- `ESP_Spectre`
- `ESP_Tac_Craftsmen`

Magazines:

- `Mag_FAL_20Rnd`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### ESP_M14

Firearms:

- `ESP_M14`
- `ESP_M14_Black`

Parts and accessories:

- No ESP_M14-specific attachment relationship is visible in local XML beyond the magazine

Magazines:

- `Mag_FAL_20Rnd`

Ammo:

- `Ammo_308Win`
- `Ammo_308WinTracer`
- `AmmoBox_308Win_20Rnd`
- `AmmoBox_308WinTracer_20Rnd`

### ttar9

Firearms:

- `ttar9`

Parts and accessories:

- `ACOGOptic`
- `M4_T3NRDSOptic`
- `M68Optic`
- `PistolSuppressor`
- `ReflexOptic`

Magazines:

- `Mag_MP5_15Rnd`
- `Mag_MP5_30Rnd` may also be compatible by magazine family, but only `Mag_MP5_15Rnd` is shown in `cfgspawnabletypes.xml`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### ttbar

Firearms:

- `ttbar`

Parts and accessories:

- No ttbar-specific attachment relationship is visible in local XML beyond the magazine

Magazines:

- `Mag_VAL_20Rnd`

Ammo:

- `Ammo_9x39`
- `Ammo_9x39AP`
- `AmmoBox_9x39_20Rnd`
- `AmmoBox_9x39AP_20Rnd`

### ttr700

Firearms:

- `ttr700`

Parts and accessories:

- `ESP_Hunter`
- `ESP_Long_Range`
- `HuntingOptic`

Magazines:

- `ttr700mag`

Ammo:

- Inferred as `.308 Win` from the R700 pattern; local XML does not expose cartridge binding directly

### ttrpd

Firearms:

- `ttrpd`

Parts and accessories:

- No ttrpd-specific attachment relationship is visible in local XML beyond the magazine

Magazines:

- `ttrpdmag`

Ammo:

- Local XML does not expose cartridge binding directly

### ttscarl

Firearms:

- `ttscarl`
- `ttscarl_Tan`

Parts and accessories:

- `ACOGOptic`
- `M4_T3NRDSOptic`
- `ReflexOptic`

Magazines:

- `Mag_CMAG_10Rnd`
- `Mag_CMAG_20Rnd`
- `Mag_CMAG_30Rnd`
- `Mag_CMAG_40Rnd`
- `Mag_STANAG_30Rnd`

Ammo:

- `Ammo_556x45`
- `Ammo_556x45Tracer`
- `AmmoBox_556x45_20Rnd`
- `AmmoBox_556x45Tracer_20Rnd`

### ttp90

Firearms:

- `ttp90`
- `ttp90_Tan`

Parts and accessories:

- `ACOGOptic`
- `M4_T3NRDSOptic`
- `M68Optic`
- `PistolSuppressor`
- `ReflexOptic`

Magazines:

- `ttp90mag`

Ammo:

- Local XML does not expose cartridge binding directly

### ttak458 / ttsok94

Firearms:

- `ttak458`
- `ttak458_Gold`
- `ttak458poli`
- `ttsok94`

Parts and accessories:

- `AK_PlasticBttstck`
- `AK_PlasticHndgrd`
- `KashtanOptic`
- `KobraOptic`
- `PSO11Optic`

Magazines:

- `Mag_AKM_30Rnd`
- `Mag_AKM_Drum75Rnd`
- `Mag_AKM_Palm30Rnd`

Ammo:

- `Ammo_762x39`
- `Ammo_762x39Tracer`
- `AmmoBox_762x39_20Rnd`
- `AmmoBox_762x39Tracer_20Rnd`

### ttstg44

Firearms:

- `ttstg44`

Parts and accessories:

- No ttstg44-specific attachment relationship is visible in local XML beyond the magazine

Magazines:

- `Mag_AKM_30Rnd`
- `Mag_AKM_Drum75Rnd`
- `Mag_AKM_Palm30Rnd`

Ammo:

- `Ammo_762x39`
- `Ammo_762x39Tracer`
- `AmmoBox_762x39_20Rnd`
- `AmmoBox_762x39Tracer_20Rnd`

### ots71

Firearms:

- `ots71`

Parts and accessories:

- No ots71-specific attachment relationship is visible in local XML beyond the magazine

Magazines:

- `Mag_MP5_15Rnd`
- `Mag_MP5_30Rnd` may also be compatible by magazine family, but only `Mag_MP5_15Rnd` is shown in `cfgspawnabletypes.xml`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### tt57

Firearms:

- `tt57blk`
- `tt57tan`

Parts and accessories:

- `PistolSuppressor`
- `TLRLight`

Magazines:

- `tt57mag`

Ammo:

- Local XML does not expose cartridge binding directly

### ttm9

Firearms:

- `ttm9`
- `ttm9_Gold`

Parts and accessories:

- `PistolSuppressor`

Magazines:

- `Mag_Glock_15Rnd`

Ammo:

- `Ammo_9x19`
- `AmmoBox_9x19_25rnd`

### JMC Snow Owl

Firearms:

- `jmc_SnowOwl`

Parts and accessories:

- `jmc_SnowOwl_Optic`

Magazines:

- `jmc_SnowOwl_Mag`

Ammo:

- `jmc_SnowOwl_Ammo`
- `STAG_AmmoBox_SnowOwl`

### JMC Prototype

Firearms:

- `jmc_Prototype_Blue`
- `jmc_Prototype_Red`
- `jmc_Prototype_White`

Parts and accessories:

- `jmc_Prototype_Optic_Blue`
- `jmc_Prototype_Optic_Red`
- `jmc_Prototype_Optic_White`

Magazines:

- `jmc_Prototype_Mag`

Ammo:

- Local XML does not expose cartridge binding directly

## Modded Firearms Without Confirmed Local Compatibility

These firearm classnames are present in `types.xml`, but no matching weapon-to-magazine entry was found in `cfgspawnabletypes.xml` and no clearly matching dedicated magazine exists in `types.xml`. Treat their ammo/accessory assignments as unknown unless verified against the mod config or in-game.

- `avs36`
- `ESPAR22`
- `ESPAR47`
- `flintlock`
- `higgins`
- `kar98`
- `kingcobra`
- `marlin`
- `tec9`
- `ttagram`
- `ttgarand`
- `ttl35`
- `ttmauser1914`
- `ttp38`

Related loose magazine-looking classnames without confirmed firearm mapping:

- `ttl35mag`

## Spawnable Attachments Referenced But Missing From types.xml

These are referenced by `cfgspawnabletypes.xml` as weapon attachments, but they are not present as loot-economy entries in `types.xml`.

- `AugOptic`
- `ESP_SCAR_RIS_HNDGRD_Black`
- `ESP_SCAR_RIS_HNDGRD_Tan`
- `ESP_SCAR_Stck_Black`
- `ESP_SCAR_Stck_Tan`
- `ESP_SCAR_URGI_HNDGRD_Black`
- `ESP_SCAR_URGI_HNDGRD_Tan`
- `MakarovPBSuppressor`
- `SRP_ButtstockPouch_Black`
- `SRP_ButtstockPouch_Linen`
- `SSG82Optic`
- `STG_AK_Junk_HNDGRD_Junk`
- `STG_AK_Junk_Stck_Junk`
- `STG_AK_Wood_HNDGRD_Dark_Wood`

## Firearm-Related Accessories Not Assigned To One Firearm Above

These accessories are present in `types.xml` but are broad-use or not tied to a single firearm family by local XML:

- `ESP_RedDot`
- `ESP_RIS_Adapter`
- `ESP_Short_Range`
- `ESP_Wrapped_Suppressor`
- `ESP_Wrapped_Suppressor_Black`
- `ESP_Wrapped_Suppressor_Green`
- `MK4Optic_Black`
- `MK4Optic_Green`
- `MK4Optic_Tan`
- `SRP_PistolSuppressor_Golden`
- `SportingOptic`
- `StarlightOptic`
