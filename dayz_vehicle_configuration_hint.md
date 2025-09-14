# DayZ Server Vehicle Configuration Guide

## Overview
This guide explains how to configure vehicles on a DayZ server using four key XML files. Each file serves a specific purpose in defining vehicle spawning, equipment, and distribution across the game world.

## File Structure and Purpose

### 1. `types.xml` - Vehicle Class Definitions
**Location**: `mpmissions/[map_name]/db/types.xml`

This file contains class names for all in-game items, including vehicles and their attachments. Each vehicle type is defined with properties that control its spawning behavior.

**Key Properties:**
- `nominal`: Desired number of this vehicle type in the world
- `lifetime`: Time (in seconds) before vehicle despawns if not interacted with
- `min`: Minimum number of vehicles that must exist
- `restock`: Time interval for restocking (0 = no restock)
- `flags`: Control various behaviors (count_in_map, deletable, etc.)
- `category`: Item category (e.g., "vehicles")

**Important**: You must define ALL vehicle parts in types.xml, not just the main vehicle. This includes:
- **Color-matched parts**: Doors, hoods, trunks for each color variant
- **Common parts**: Wheels, bumpers, rollbars (if applicable)
- **Recommended values**: `nominal: 5`, `min: 2`, `lifetime: 7200` for vehicle parts

**Example Entry:**
```xml
<type name="ToyotaRunner_BOSS">
    <nominal>0</nominal>
    <lifetime>3</lifetime>
    <restock>1800</restock>
    <min>0</min>
    <quantmin>-1</quantmin>
    <quantmax>-1</quantmax>
    <cost>100</cost>
    <flags count_in_cargo="0" count_in_hoarder="0" count_in_map="1" count_in_player="0" crafted="0" deloot="0"/>
</type>
```

### 2. `events.xml` - Vehicle Spawn Events
**Location**: `mpmissions/[map_name]/db/events.xml`

Defines dynamic events that control vehicle spawning. Each event can contain multiple vehicle variants and specifies spawn parameters.

**Key Properties:**
- `nominal`: Target number of vehicles for this event
- `min`/`max`: Minimum and maximum number of vehicles that can spawn
- `lifetime`: How long vehicles persist (in seconds)
- `saferadius`: Minimum distance from players for spawning
- `distanceradius`: Maximum distance from players for spawning
- `cleanupradius`: Distance for cleanup operations
- `children`: Defines which vehicle variants can spawn in this event

**Example Event:**
```xml
<event name="VehicleToyotaRunner">
    <nominal>10</nominal>
    <min>5</min>
    <max>10</max>
    <lifetime>300</lifetime>
    <restock>0</restock>
    <saferadius>500</saferadius>
    <distanceradius>500</distanceradius>
    <cleanupradius>200</cleanupradius>
    <flags deletable="0" init_random="0" remove_damaged="1"/>
    <position>fixed</position>
    <limit>mixed</limit>
    <active>1</active>
    <children>
        <child lootmax="0" lootmin="0" max="1" min="1" type="ToyotaRunner_BOSS"/>
        <child lootmax="0" lootmin="0" max="1" min="1" type="ToyotaRunner_Black"/>
        <child lootmax="0" lootmin="0" max="1" min="1" type="ToyotaRunner_camogreen"/>
        <child lootmax="0" lootmin="0" max="1" min="1" type="ToyotaRunner_camoblac"/>
        <child lootmax="0" lootmin="0" max="1" min="1" type="ToyotaRunner_camowhite"/>
        <child lootmax="0" lootmin="0" max="1" min="1" type="ToyotaRunner_blue"/>
    </children>
</event>
```

### 3. `cfgspawnabletypes.xml` - Vehicle Equipment Configuration
**Location**: `mpmissions/[map_name]/cfgspawnabletypes.xml`

Defines the potential state of each vehicle upon spawning, including attached items and their spawn probabilities.

**Essential Vehicle Components (Recommended Spawn Rates):**
- **Wheels**: 4 wheels at 100% chance, 5th spare wheel at 40% chance (ensures drivability)
- **Doors**: Front and rear doors at 80-100% chance
- **Hood/Trunk**: 70-100% chance
- **Spark Plug**: 60% chance (essential for starting, but not guaranteed)
- **Radiator**: 60% chance (prevents overheating, but not guaranteed)
- **Battery**: 30% chance for trucks (TruckBattery), 60% for cars (CarBattery)
- **Headlights**: 50% chance each (optional but useful)

**Important**: Essential components like spark plugs, radiators, and batteries should NOT be set to 100% chance. This creates realistic vehicle conditions where players need to scavenge for parts, adding to the survival experience. Base spawn chances on existing vehicle configurations in your cfgspawnabletypes.xml file.

**Example Configuration:**
```xml
<type name="ToyotaRunner_BOSS">
    <attachments chance="1.00">
        <item name="Runner_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="0.30">
        <item name="Runner_Wheel" chance="1.00" />
    </attachments>
    <attachments chance="0.50">
        <item name="HeadlightH7" chance="1.00" />
    </attachments>
    <attachments chance="0.50">
        <item name="HeadlightH7" chance="1.00" />
    </attachments>
    <attachments chance="0.30">
        <item name="CarBattery" chance="1.00" />
    </attachments>
    <attachments chance="0.30">
        <item name="CarRadiator" chance="1.00" />
    </attachments>
    <attachments chance="0.30">
        <item name="SparkPlug" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_Cargo1_BOSS" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_Cargo2_BOSS" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_CoDriver_Door_BOSS" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_DriverDoor_BOSS" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_Hood_BOSS" chance="1.00" />
    </attachments>
    <attachments chance="1.00">
        <item name="Runner_Trunk_BOSS" chance="1.00" />
    </attachments>
</type>
```

### 4. `cfgeventspawns.xml` - Spawn Location Configuration
**Location**: `mpmissions/[map_name]/cfgeventspawns.xml`

Defines specific locations where vehicles can spawn. **Important**: The number of spawn points should exceed the number of vehicles defined in `events.xml` to ensure random distribution.

**Coordinate Format:**
- `x`: East-West coordinate
- `z`: North-South coordinate  
- `a`: Rotation angle (0-360 degrees)
- `y`: Elevation/height (optional)

**Example Spawn Points:**
```xml
<event name="VehicleToyotaRunner">
    <pos x="4228.070313" z="11766.463867" a="146.389877" y="5.576471" />
    <pos x="5408.260742" z="10846.312500" a="99.492996" y="19.895000" />
    <pos x="5146.819336" z="13233.846680" a="95.529465" y="6.157139" />
    <!-- Additional positions... -->
</event>
```

## Configuration Workflow

1. **Define Vehicle Types** in `types.xml`
2. **Create Spawn Events** in `events.xml` with multiple vehicle variants
3. **Configure Equipment** in `cfgspawnabletypes.xml` with appropriate spawn chances
4. **Set Spawn Locations** in `cfgeventspawns.xml` with more points than vehicles

## User Input Required

**For `cfgeventspawns.xml` configuration, please provide:**

A list of coordinates where you want vehicles to spawn. Format each coordinate as:
```
X: [coordinate], Z: [coordinate], A: [angle], Y: [elevation]
```

**Example:**
```
X: 4228.07, Z: 11766.46, A: 146.39, Y: 5.58
X: 5408.26, Z: 10846.31, A: 99.49, Y: 19.90
X: 5146.82, Z: 13233.85, A: 95.53, Y: 6.16
```

**Guidelines:**
- Provide more spawn points than the number of vehicles defined in `events.xml`
- Choose locations that are accessible and make sense for vehicle placement
- Consider terrain elevation (Y coordinate) for proper vehicle placement
- Ensure spawn points are distributed across the map for variety

## Best Practices

1. **Vehicle Count**: Set `nominal` in `events.xml` to desired active vehicles
2. **Spawn Points**: Always have 2-3x more spawn points than vehicles for randomness
3. **Essential Parts**: Ensure wheels, battery, radiator, and spark plug have high spawn chances
4. **Variety**: Use multiple vehicle variants in a single event for visual diversity
5. **Testing**: Test spawn locations in-game to ensure vehicles spawn properly
6. **Performance**: Avoid too many vehicles or spawn points to maintain server performance

## Common Vehicle Class Names

- **Cars**: `Sedan_02`, `Hatchback_02`, `Offroad_02`
- **Trucks**: `Truck_01`, `TransitBus`
- **ATVs**: `ATVCivil`, `ATVSTAG`
- **Boats**: `BoatSTAG`, `RFWC_Watercraft`
- **Special**: `MotorHome`, `ToyotaRunner`

## Troubleshooting

- **Vehicles not spawning**: Check `events.xml` active status and spawn point validity
- **Missing parts**: Verify `cfgspawnabletypes.xml` attachment configurations
- **Poor distribution**: Increase number of spawn points in `cfgeventspawns.xml`
- **Performance issues**: Reduce vehicle counts or spawn point density
