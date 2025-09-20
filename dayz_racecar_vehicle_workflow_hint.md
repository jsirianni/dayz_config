# DayZ RaceCar Event Vehicle Addition Workflow

## Overview
This document provides a comprehensive workflow for adding new high-performance vehicles to the DayZ RaceCar event system. The RaceCar event is designed to spawn rare, valuable vehicles that players actively hunt for, with specific timing mechanics to maintain scarcity and excitement.

## RaceCar Event Configuration
The RaceCar event is configured with the following parameters:
- **nominal**: 1 (target number of vehicles)
- **min**: 0 (minimum vehicles that must exist)
- **max**: 1 (maximum vehicles that can spawn at once)
- **lifetime**: 10800 seconds (3 hours - how long each vehicle stays available)
- **restock**: 3600 seconds (1 hour - gap between spawns for scarcity)
- **Spawn Points**: 18 predefined locations in cfgeventspawns.xml

## Complete Workflow for Adding New Vehicles

### Step 1: Research Vehicle Availability
1. **Check classnames.txt** for available vehicle color variants
   - Search for the base vehicle name (e.g., "MitsubishiEVO", "BMW_M3")
   - Identify all available color variants
   - Note the exact naming convention used

2. **Check types.xml** for existing vehicle definitions
   - Verify the vehicle types are already defined
   - Check if color-matched parts exist
   - Note the part naming pattern (e.g., "Mitsubishi_LancerEvolutionIX_Driver_[color]")

### Step 2: Define Vehicle Types in types.xml
**Location**: `servers/first-light-deer-isle-hardcore-us-east1/mpmissions/db/types.xml`

For each vehicle color variant, add:
```xml
<type name="VehicleName_Color">
    <lifetime>3888000</lifetime>
    <flags count_in_cargo="0" count_in_hoarder="0" count_in_map="1" count_in_player="0" crafted="0" deloot="0"/>
</type>
```

For each color-matched part, add:
```xml
<type name="PartName_Color">
    <lifetime>7200</lifetime>
    <flags count_in_cargo="0" count_in_hoarder="0" count_in_map="1" count_in_player="0" crafted="0" deloot="0"/>
</type>
```

**Required Parts for Each Vehicle:**
- Driver door: `[VehicleBase]_Driver_[color]`
- Co-driver door: `[VehicleBase]_CoDriver_[color]` or `[VehicleBase]_CoDriver_[color]`
- Hood: `[VehicleBase]_Hood_[color]`
- Trunk: `[VehicleBase]_Trunk_[color]`
- Wheels: `[VehicleBase]_Wheel` (usually shared across all colors)

### Step 3: Add Vehicles to RaceCar Event
**Location**: `servers/first-light-deer-isle-hardcore-us-east1/mpmissions/db/events.xml`

Add each vehicle color variant to the RaceCar event's children section:
```xml
<child lootmax="0" lootmin="0" max="1" min="1" type="VehicleName_Color"/>
```

**Important**: Maintain alphabetical ordering within the children section for consistency.

### Step 4: Create Spawnable Types Configuration
**Location**: `servers/first-light-deer-isle-hardcore-us-east1/mpmissions/cfgspawnabletypes.xml`

For each vehicle color variant, create a complete spawnable type configuration:

```xml
<!-- VehicleName Color vehicle configuration -->
<type name="VehicleName_Color">
    <!-- 5 Wheels (100% spawn chance) -->
    <attachments chance="1.00">
        <item name="VehicleBase_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="VehicleBase_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="VehicleBase_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="VehicleBase_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="VehicleBase_Wheel" chance="1.00" />
    </attachments>
    
    <!-- 2 Headlights (100% spawn chance) -->
    <attachments chance="1.00">
        <item name="HeadlightH7" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="HeadlightH7" chance="1.00" />
    </attachments>
    
    <!-- Essential Components (100% spawn chance) -->
    <attachments chance="1.00">
        <item name="CarBattery" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="CarRadiator" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="SparkPlug" chance="1.00" />
    </attachments>
    
    <!-- Color-Matched Body Parts (100% spawn chance) -->
    <attachments chance="1.00">
        <item name="VehicleBase_Driver_[color]" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="VehicleBase_CoDriver_[color]" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="VehicleBase_Hood_[color]" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="VehicleBase_Trunk_[color]" chance="1.00" />
    </attachments>
</type>
```

## Vehicle Naming Conventions by Manufacturer

### Audi RS6 ABT
- **Base**: `Audi_RS6_ABT`
- **Colors**: `_Black`, `_Blue`, `_Green`, `_MidNightBlue`, `_Pink`
- **Parts**: `Audi_RS6_ABT_Driver_[color]`, `Audi_RS6_ABT_CoDriver_[color]`, `Audi_RS6_ABT_Cargo1_[color]`, `Audi_RS6_ABT_Cargo2_[color]`, `Audi_RS6_ABT_Hood_[color]`, `Audi_RS6_ABT_Trunk_[color]`
- **Wheels**: `Audi_RS6_ABT_Wheel`

### BMW M3
- **Base**: `BMW_M3`
- **Colors**: `_Black`, `_White`, `_MidNightBlue`, `_Blue`, `_Orange`
- **Parts**: `BMW_M3_Driver_[color]`, `BMW_M3_CoDriver_[color]`, `BMW_M3_Hood_[color]`, `BMW_M3_Trunk_[color]`
- **Wheels**: `BMW_M3_Wheel`

### Chevrolet NapalmNova
- **Base**: `Chevrolet_NapalmNova`
- **Colors**: `_Black`, `_Blue`, `_Green`, `_Red`, `_Yellow`
- **Parts**: `Chevrolet_NapalmNova_Driver_[color]`, `Chevrolet_NapalmNova_CoDriver_[color]`, `Chevrolet_NapalmNova_Hood_[color]`, `Chevrolet_NapalmNova_Trunk_[color]`
- **Wheels**: `Chevrolet_NapalmNova_Wheel`

### Honda Civic
- **Base**: `Civic`
- **Colors**: `_Black`, `_Blue`, `_Green`, `_RedCustom`, `_White`
- **Parts**: `Honda_Civic_DriverDoor_[color]`, `Honda_Civic_CoDriver_[color]`, `Honda_Civic_Hood_[color]`, `Honda_Civic_Trunk_[color]`
- **Wheels**: `Honda_Civic_Wheel`

### Ford Mustang Shelby GT500
- **Base**: `Ford_Mustang_Shelby_GT500`
- **Colors**: `_Black`, `_Blue`, `_DarkBlue`, `_Green`, `_Orange`
- **Parts**: `Ford_Mustang_Shelby_GT500_Driver_[color]`, `Ford_Mustang_Shelby_GT500_CoDriver_[color]`, `Ford_Mustang_Shelby_GT500_Hood_[color]`, `Ford_Mustang_Shelby_GT500_Trunk_[color]`
- **Wheels**: `Ford_Mustang_Shelby_GT500_Wheel`

### MitsubishiEVO
- **Base**: `MitsubishiEVO`
- **Colors**: `_FastAnFurious`, `_FastAnFurious2`, `_darkblue`, `_pink`, `_bee`
- **Parts**: `Mitsubishi_LancerEvolutionIX_Driver_[color]`, `Mitsubishi_LancerEvolutionIX_CoDriver_[color]`, `Mitsubishi_LancerEvolutionIX_Hood_[color]`, `Mitsubishi_LancerEvolutionIX_Trunk_[color]`
- **Wheels**: `Mitsubishi_LancerEvolutionIX_Wheel`

### Nissan GTR
- **Base**: `NissanGTR`
- **Colors**: `_Blue`, `_Green`, `_Orange`, `_Pink`, `_Red`
- **Parts**: `NissanGTR_Driverdoor_[color]`, `NissanGTR_Codriverdoor_[color]`, `NissanGTR_Hood_[color]`, `NissanGTR_Trunk_[color]`
- **Wheels**: `NissanGTRNismo_Wheel`

### Nissan Skyline
- **Base**: `Nissan_Skyline`
- **Colors**: `_Black`, `_Blue`, `_Orange`, `_Pink`, `_White`
- **Parts**: `Skyline_driverdoor_[color]`, `Skyline_codriver_door_[color]`, `Skyline_Hood_[color]`, `Skyline_Trunk_[color]`
- **Wheels**: `Skyline_Wheel`

### Porsche 911RWB
- **Base**: `Porsche_911RWB`
- **Colors**: `_Black`, `_Blue`, `_Green`, `_Orange`, `_Red`
- **Parts**: `Porsche_911RWB_Driver_Door_[color]`, `Porsche_911RWB_CoDriver_Door_[color]`, `Porsche_911RWB_Hood_[color]`, `Porsche_911RWB_Trunk_[color]`
- **Wheels**: `Porsche_911RWB_Front_Wheel`

### Toyota Supra
- **Base**: `Supra`
- **Colors**: `_Black`, `_Blue`, `_Green`, `_Red`, `_White`
- **Parts**: `Supra_DriverDoor_[color]`, `Supra_CoDriverDoor_[color]`, `Supra_Hood_[color]`, `Supra_Trunk_[color]`
- **Wheels**: `Supra_Wheel`

## Quality Assurance Checklist

### Before Implementation
- [ ] Verify vehicle exists in classnames.txt
- [ ] Check if vehicle types are defined in types.xml
- [ ] Identify correct part naming convention
- [ ] Confirm color variants are available

### During Implementation
- [ ] Add all vehicle types to types.xml with correct lifetime/flags
- [ ] Add all part types to types.xml with correct lifetime/flags
- [ ] Add vehicles to RaceCar event in events.xml
- [ ] Create complete spawnable types in cfgspawnabletypes.xml
- [ ] Ensure all parts have 100% spawn chance
- [ ] Verify correct part names match classnames.txt

### After Implementation
- [ ] Check for any linter errors
- [ ] Verify XML syntax is valid
- [ ] Confirm all vehicle colors are properly configured
- [ ] Test that vehicle names match exactly between all files

## Common Pitfalls to Avoid

1. **Incorrect Part Naming**: Each manufacturer uses different naming conventions for parts
2. **Missing Color Variants**: Not all colors may be available for every vehicle
3. **Wrong Wheel Type**: Some vehicles use specific wheel types (e.g., NissanGTRNismo_Wheel vs generic wheels)
4. **Case Sensitivity**: Vehicle and part names are case-sensitive
5. **XML Syntax**: Ensure all tags are properly closed and attributes are correctly formatted

## File Locations Summary

- **Vehicle Types**: `servers/first-light-deer-isle-hardcore-us-east1/mpmissions/db/types.xml`
- **Event Configuration**: `servers/first-light-deer-isle-hardcore-us-east1/mpmissions/db/events.xml`
- **Spawnable Types**: `servers/first-light-deer-isle-hardcore-us-east1/mpmissions/cfgspawnabletypes.xml`
- **Spawn Locations**: `servers/first-light-deer-isle-hardcore-us-east1/mpmissions/cfgeventspawns.xml`
- **Class Names Reference**: `boss/classnames.txt`

## AI/LLM Prompt Template

When requesting new vehicles to be added to the RaceCar event, use this template:

```
Add [VehicleName] vehicles to the RaceCar event with colors [list of colors]. 
Follow the complete workflow:
1. Check classnames.txt for available variants and part names
2. Add all vehicle and part types to types.xml with proper lifetime and flags
3. Add vehicles to RaceCar event in events.xml
4. Create spawnable types in cfgspawnabletypes.xml with 100% spawn chance for all parts
5. Ensure color-matched parts are properly configured

Vehicle: [VehicleName]
Colors: [Color1], [Color2], [Color3], [Color4], [Color5]
Expected parts: [Driver], [CoDriver], [Hood], [Trunk], [Wheels]
```

This workflow ensures consistent, high-quality vehicle additions to the RaceCar event system while maintaining the intended rare vehicle spawning mechanics.
