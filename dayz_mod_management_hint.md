# DayZ Server Mod Management Guide

## Overview
This guide explains how to manage mods in the DayZ server using the `dayz-service.bat` file. The system automatically downloads, syncs, and configures mods from the Steam Workshop.

## File Structure and Purpose

### `dayz-service.bat` - Mod Management Script
**Location**: `servers/[server_name]/dayz-service.bat`

This batch file handles the complete mod lifecycle: downloading from Steam Workshop, syncing to server directories, and configuring the DayZ server launch parameters.

## Mod Management System Components

### 1. Workshop ID Definitions
**Location**: Lines 13-27 in `dayz-service.bat`

Each mod requires a unique Steam Workshop ID to be defined as a variable.

**Format:**
```batch
set "MOD_[NAME]=[WORKSHOP_ID]"
```

**Example:**
```batch
set "MOD_CF=1559212036"
set "MOD_VPP=1828439124"
set "MOD_YAHT=3354681846"
```

**Guidelines:**
- Use descriptive names that match the mod's purpose
- Keep names short but clear (avoid spaces and special characters)
- Use uppercase with underscores for consistency

### 2. Destination Folder Names
**Location**: Lines 29-43 in `dayz-service.bat`

Defines the server-side folder names where mods will be synced. These must match the mod's actual folder structure.

**Format:**
```batch
set "DST_[NAME]=@[FolderName]"
```

**Example:**
```batch
set "DST_CF=@CF"
set "DST_VPP=@VPPAdminTools"
set "DST_YAHT=@Yaht"
```

**Important Rules:**
- **NO SPACES** in folder names
- Must start with `@` symbol
- Must match the mod's actual folder structure from Steam Workshop
- Case-sensitive - use exact capitalization from the mod

### 3. Steam Workshop Download
**Location**: Lines 54-72 in `dayz-service.bat`

Downloads mods from Steam Workshop using SteamCMD.

**Format:**
```batch
+workshop_download_item 221100 %MOD_[NAME]% validate ^
```

**Example:**
```batch
+workshop_download_item 221100 %MOD_YAHT% validate ^
```

**Parameters:**
- `221100`: DayZ's Steam App ID
- `%MOD_[NAME]%`: Variable containing the Workshop ID
- `validate`: Verifies file integrity
- `^`: Line continuation character for multi-line commands

### 4. Mod Synchronization
**Location**: Lines 75-89 in `dayz-service.bat`

Syncs downloaded mods to server directories using the `:syncmod` function.

**Format:**
```batch
call :syncmod "%MOD_[NAME]%" "%DST_[NAME]%"
```

**Example:**
```batch
call :syncmod "%MOD_YAHT%" "%DST_YAHT%"
```

**What it does:**
- Copies mod files from Steam Workshop directory to server @ folder
- Automatically copies `.bikey` files to server keys directory
- Uses `robocopy` with `/MIR` flag for complete synchronization

### 5. Mod List Configuration
**Location**: Line 92 in `dayz-service.bat`

Builds the mod list for DayZ server launch. **Critical**: Order matters - CF must be first.

**Format:**
```batch
set "MODLINE=-mod=@CF;[@Mod1];[@Mod2];[@Mod3]"
```

**Example:**
```batch
set "MODLINE=-mod=@CF;@VPPAdminTools;@DeerIsle;@CodeLock;@Vehicle3PP;@RedFalconBoat;@Crocodile;@Shark;@DboDino;@Dino;@FireSticks;@4KBOSSKVehicles;@Dog;@Yaht"
```

**Important Rules:**
- **CF must be first** in the mod list
- Use semicolons (`;`) to separate mods
- No spaces around semicolons
- Use exact destination folder names (with @ prefix)

## Adding a New Mod - Step-by-Step Process

### Step 1: Get the Workshop ID
1. Go to the mod's Steam Workshop page
2. Copy the ID from the URL (e.g., `https://steamcommunity.com/sharedfiles/filedetails/?id=3354681846`)
3. The ID is `3354681846`

### Step 2: Add Workshop ID Definition
Add a new line in the "Workshop IDs" section:
```batch
set "MOD_[DESCRIPTIVE_NAME]=[WORKSHOP_ID]"
```

### Step 3: Add Destination Folder Name
Add a new line in the "Server @mod folder names" section:
```batch
set "DST_[DESCRIPTIVE_NAME]=@[ActualFolderName]"
```

**Note**: You need to check the actual folder name by downloading the mod first or checking the mod's documentation.

### Step 4: Add Download Command
Add a new line in the SteamCMD download section:
```batch
+workshop_download_item 221100 %MOD_[DESCRIPTIVE_NAME]% validate ^
```

### Step 5: Add Sync Command
Add a new line in the sync section:
```batch
call :syncmod "%MOD_[DESCRIPTIVE_NAME]%" "%DST_[DESCRIPTIVE_NAME]%"
```

### Step 6: Update Mod List
Add the mod to the MODLINE variable (after @CF):
```batch
set "MODLINE=-mod=@CF;@VPPAdminTools;@DeerIsle;@CodeLock;@Vehicle3PP;@RedFalconBoat;@Crocodile;@Shark;@DboDino;@Dino;@FireSticks;@4KBOSSKVehicles;@Dog;@Yaht;@[NewMod]"
```

## Mod Management Functions

### `:syncmod` Function
**Location**: Lines 101-115 in `dayz-service.bat`

This function handles the synchronization of mods from Steam Workshop to server directories.

**Parameters:**
- `%1`: Workshop ID
- `%2`: Destination folder name

**What it does:**
1. Sets source path: `%WSCONTENT%\%~1`
2. Sets destination path: `%INSTALL%\%~2`
3. Creates destination directory if it doesn't exist
4. Uses `robocopy` to mirror the mod files
5. Automatically copies `.bikey` files to server keys directory

**Key Features:**
- **Automatic key copying**: Searches for `.bikey` files in common locations
- **Error handling**: Warns if source path doesn't exist
- **Complete sync**: Uses `/MIR` flag to ensure exact replication

## Directory Structure

### Steam Workshop Content
```
C:\SteamCMD\steamapps\workshop\content\221100\
├── 1559212036\          # CF mod files
├── 1828439124\          # VPP mod files
├── 3354681846\          # Yaht mod files
└── [other_mod_ids]\
```

### Server Installation
```
C:\DayZ\server\
├── @CF\                 # CF mod files
├── @VPPAdminTools\      # VPP mod files
├── @Yaht\               # Yaht mod files
├── keys\                # All .bikey files
└── DayZServer_x64.exe
```

## Best Practices

### 1. Mod Order
- **Always put CF first** in the mod list
- Place core/admin mods early (VPP, etc.)
- Put content mods after core mods
- Place map mods last

### 2. Naming Conventions
- Use descriptive variable names: `MOD_YAHT` not `MOD_MOD1`
- Keep destination names short: `@Yaht` not `@YahtMod`
- Use consistent capitalization

### 3. Testing New Mods
1. Add the mod to the script
2. Run the script to download and sync
3. Check the server logs for mod loading errors
4. Test in-game functionality
5. Remove if causing issues

### 4. Mod Dependencies
- Check mod requirements before adding
- Some mods require specific load order
- Document dependencies in comments

### 5. Performance Considerations
- Too many mods can impact server performance
- Monitor server resources after adding mods
- Consider removing unused mods

## Troubleshooting

### Common Issues

**Mod not loading:**
- Check folder name matches exactly (case-sensitive)
- Verify Workshop ID is correct
- Ensure mod is in the MODLINE variable

**Server won't start:**
- Check mod order (CF must be first)
- Verify all mods are properly synced
- Check server logs for specific errors

**Missing keys:**
- The script automatically copies `.bikey` files
- Check if mod has keys in non-standard locations
- Manually copy keys if needed

**Mod conflicts:**
- Check mod compatibility
- Adjust load order
- Remove conflicting mods

### Debugging Steps

1. **Check file paths:**
   ```batch
   echo Source: %WSCONTENT%\[MOD_ID]
   echo Destination: %INSTALL%\[@FolderName]
   ```

2. **Verify mod download:**
   - Check if mod folder exists in Steam Workshop directory
   - Verify file count and size

3. **Test mod sync:**
   - Run sync command manually
   - Check destination folder contents

4. **Validate mod list:**
   - Ensure all mods in MODLINE exist
   - Check for typos in folder names

## Example: Adding a New Mod

Let's say you want to add a mod called "Advanced Medical" with Workshop ID `1234567890`:

### Step 1: Add Workshop ID
```batch
set "MOD_ADVMED=1234567890"
```

### Step 2: Add Destination (assuming folder name is @AdvancedMedical)
```batch
set "DST_ADVMED=@AdvancedMedical"
```

### Step 3: Add Download
```batch
+workshop_download_item 221100 %MOD_ADVMED% validate ^
```

### Step 4: Add Sync
```batch
call :syncmod "%MOD_ADVMED%" "%DST_ADVMED%"
```

### Step 5: Update Mod List
```batch
set "MODLINE=-mod=@CF;@VPPAdminTools;@DeerIsle;@CodeLock;@Vehicle3PP;@RedFalconBoat;@Crocodile;@Shark;@DboDino;@Dino;@FireSticks;@4KBOSSKVehicles;@Dog;@Yaht;@AdvancedMedical"
```

## Security Considerations

### Steam Credentials
- The script uses hardcoded Steam credentials
- Consider using environment variables for production
- Rotate credentials regularly

### Mod Validation
- Always use `validate` parameter in SteamCMD
- Check mod integrity before deployment
- Monitor for mod updates and security issues

### File Permissions
- Ensure proper permissions on server directories
- Limit access to mod management scripts
- Regular backups of mod configurations

## Maintenance

### Regular Tasks
1. **Update mods**: Run script regularly to get latest versions
2. **Clean unused mods**: Remove mods no longer needed
3. **Monitor performance**: Check server resources
4. **Backup configurations**: Save working mod lists

### Mod Updates
- Steam Workshop mods update automatically when you run the script
- Test updates in a staging environment first
- Document any breaking changes

### Configuration Backup
- Keep copies of working `dayz-service.bat` files
- Document mod purposes and dependencies
- Version control your configurations
