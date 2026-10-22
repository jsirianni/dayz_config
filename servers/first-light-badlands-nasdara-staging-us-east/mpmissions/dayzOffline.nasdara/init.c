void main()
{
	//INIT ECONOMY--------------------------------------
	Hive ce = CreateHive();
	if ( ce )
		ce.InitOffline();

	//DATE RESET AFTER ECONOMY INIT-------------------------
	int year, month, day, hour, minute;
	int reset_month = 4, reset_day = 30;
	GetGame().GetWorld().GetDate(year, month, day, hour, minute);

	if ((month == reset_month) && (day < reset_day))
	{
		GetGame().GetWorld().SetDate(year, reset_month, reset_day, hour, minute);
	}
	else
	{
		if ((month == reset_month + 1) && (day > reset_day))
		{
			GetGame().GetWorld().SetDate(year, reset_month, reset_day, hour, minute);
		}
		else
		{
			if ((month < reset_month) || (month > reset_month + 1))
			{
				GetGame().GetWorld().SetDate(year, reset_month, reset_day, hour, minute);
			}
		}
	}
}

class CustomMission: MissionServer
{
	void SetRandomHealth(EntityAI itemEnt)
	{
		if ( itemEnt )
		{
			float rndHlt = Math.RandomFloat( 0.45, 0.65 );
			itemEnt.SetHealth01( "", "", rndHlt );
		}
	}

	override PlayerBase CreateCharacter(PlayerIdentity identity, vector pos, ParamsReadContext ctx, string characterName)
	{
		Entity playerEnt;
		playerEnt = GetGame().CreatePlayer( identity, characterName, pos, 0, "NONE" );
		Class.CastTo( m_player, playerEnt );

		GetGame().SelectPlayer( identity, m_player );

		return m_player;
	}

	// Cargo-only placement prevents optional PU scopes attaching themselves to a rifle.
	EntityAI GiveSpawnCargo(PlayerBase player, string className, EntityAI preferredContainer = null, int row = -1, int col = -1, bool flip = false)
	{
		EntityAI item;
		InventoryLocation location = new InventoryLocation();

		if (preferredContainer)
		{
			if (row >= 0 && col >= 0)
			{
				item = preferredContainer.GetInventory().CreateEntityInCargoEx(className, 0, row, col, flip);
				if (!item)
					Print("[SpawnGear] Planned cargo placement failed for " + className);
			}

			if (!item && preferredContainer.GetInventory().FindFirstFreeLocationForNewEntity(className, FindInventoryLocationType.CARGO, location))
				item = location.GetParent().GetInventory().CreateEntityInCargoEx(className, location.GetIdx(), location.GetRow(), location.GetCol(), location.GetFlip());
		}

		if (!item && player.GetInventory().FindFirstFreeLocationForNewEntity(className, FindInventoryLocationType.CARGO, location))
			item = location.GetParent().GetInventory().CreateEntityInCargoEx(className, location.GetIdx(), location.GetRow(), location.GetCol(), location.GetFlip());
		if (!item)
			item = player.GetHumanInventory().CreateInHands(className);
		if (!item)
			item = EntityAI.Cast(GetGame().CreateObjectEx(className, player.GetPosition(), ECE_PLACE_ON_SURFACE));

		if (item)
			item.SetHealth01("", "", 1.0);
		else
			Print("[SpawnGear] Unable to create " + className);
		return item;
	}

	EntityAI GiveSpawnAttachment(EntityAI parent, string className, string slotName)
	{
		if (!parent)
		{
			Print("[SpawnGear] Missing parent for " + className);
			return null;
		}

		EntityAI item = parent.GetInventory().CreateAttachmentEx(className, InventorySlots.GetSlotIdFromString(slotName));
		if (item)
			item.SetHealth01("", "", 1.0);
		else
			Print("[SpawnGear] Unable to attach " + className + " to " + slotName);
		return item;
	}

	void GivePlayerCombatKit(PlayerBase player, EntityAI bodyClothing, EntityAI legClothing)
	{
		EntityAI carrier = GiveSpawnAttachment(EntityAI.Cast(player), "PlateCarrierVest", "Vest");
		EntityAI pouches = GiveSpawnAttachment(carrier, "PlateCarrierPouches", "VestPouch");
		EntityAI helmet = GiveSpawnAttachment(EntityAI.Cast(player), "GorkaHelmet", "Headgear");
		GiveSpawnAttachment(helmet, "GorkaHelmetVisor", "Glass");

		// Pouches: pistol 3x2; two bandages 1x2; scope 3x1; two ammo stacks 1x1.
		Weapon_Base pistol = Weapon_Base.Cast(GiveSpawnCargo(player, "Glock19", pouches, 0, 0));
		if (pistol)
		{
			Magazine pistolMagazine = pistol.SpawnAttachedMagazine("Mag_Glock_15Rnd", WeaponWithAmmoFlags.CHAMBER);
			if (pistolMagazine)
				pistolMagazine.SetHealth01("", "", 1.0);
			else
				Print("[SpawnGear] Unable to load Glock magazine");
		}

		// These are the only bandage creations: exactly two full rolls per character.
		for (int dressingIndex = 0; dressingIndex < 2; dressingIndex++)
		{
			ItemBase bandage = ItemBase.Cast(GiveSpawnCargo(player, "BandageDressing", pouches, 0, 3 + dressingIndex));
			if (bandage)
			{
				bandage.SetQuantityMax();
				if (dressingIndex == 0)
					player.SetQuickBarEntityShortcut(bandage, 2);
			}
		}

		ItemBase saline = ItemBase.Cast(GiveSpawnCargo(player, "SalineBag", bodyClothing, 0, 0));
		if (saline)
			saline.SetQuantityMax();
		GiveSpawnCargo(player, "StartKitIV", legClothing, 0, 0);
		GiveSpawnCargo(player, "Epinephrine", legClothing, 0, 1);
		for (int morphineIndex = 0; morphineIndex < 2; morphineIndex++)
			GiveSpawnCargo(player, "Morphine", legClothing, 0, 2 + morphineIndex);

		string rifleClassName;
		string ammoClassName;
		if (Math.RandomInt(0, 2) == 0)
		{
			rifleClassName = "SKS";
			ammoClassName = "Ammo_762x39";
		}
		else
		{
			rifleClassName = "Mosin9130";
			ammoClassName = "Ammo_762x54";
		}

		// No ammo-spawn/chamber call is made on the rifle.
		EntityAI rifle = GiveSpawnAttachment(EntityAI.Cast(player), rifleClassName, "Shoulder");
		if (!rifle)
			GiveSpawnCargo(player, rifleClassName);
		for (int stackIndex = 0; stackIndex < 4; stackIndex++)
		{
			Magazine ammoStack;
			if (stackIndex < 2)
				ammoStack = Magazine.Cast(GiveSpawnCargo(player, ammoClassName, pouches, 2, 3 + stackIndex));
			else
				ammoStack = Magazine.Cast(GiveSpawnCargo(player, ammoClassName, bodyClothing, 2, stackIndex - 2));
			if (ammoStack)
				ammoStack.ServerSetAmmoCount(ammoStack.GetAmmoMax());
		}

		// Roll independently of rifle selection; never attach the scope at spawn.
		if (Math.RandomFloat01() < 0.5)
			GiveSpawnCargo(player, "PUScopeOptic", pouches, 2, 0);

		EntityAI grenade = GiveSpawnAttachment(carrier, "RGD5Grenade", "VestGrenadeA");
		if (!grenade)
			GiveSpawnCargo(player, "RGD5Grenade");
	}

	override void StartingEquipSetup(PlayerBase player, bool clothesChosen)
	{
		player.RemoveAllItems();

		EntityAI bodyClothing;
		EntityAI legClothing;
		EntityAI itemEnt;

		string bodyArray[] = { "KameezShirt_Beige", "KameezShirt_Black", "KameezShirt_BlueFlowers", "KameezShirt_Brown", "KameezShirt_Grey", "KameezShirt_NavyBlue", "KameezShirt_Olive", "KameezShirt_Orange", "KameezShirt_Pink", "KameezShirt_Purple", "KameezShirt_Red", "KameezShirt_SkyBlue", "KameezShirt_White", "KameezShirt_Yellow" };
		int rndBodyIndex = Math.RandomInt( 0, 14 );
		bodyClothing = player.GetInventory().CreateInInventory( bodyArray[rndBodyIndex] );
		SetRandomHealth( bodyClothing );

		string legsArray[] = { "MensShalwarPants_Beige", "MensShalwarPants_Black", "MensShalwarPants_Brown", "MensShalwarPants_Grey", "MensShalwarPants_NavyBlue", "MensShalwarPants_SkyBlue", "MensShalwarPants_White", "WomensShalwarPants_NavyBlue", "WomensShalwarPants_Olive", "WomensShalwarPants_Orange", "WomensShalwarPants_Pink", "WomensShalwarPants_Purple", "WomensShalwarPants_Red" };
		int rndLegsIndex = Math.RandomInt( 0, 13 );
		legClothing = player.GetInventory().CreateInInventory( legsArray[rndLegsIndex] );
		SetRandomHealth( legClothing );

		string feetArray[] = { "SandalsShoes_Black", "SandalsShoes_BlueNavy", "SandalsShoes_Brown", "SandalsShoes_Grey", "TraditionalSandalShoes_Black", "TraditionalSandalShoes_Brown", "TraditionalSandalShoes_LightBrown", "TraditionalSandalShoes_NavyBlue" };
		int rndFeetIndex = Math.RandomInt( 0, 8 );
		itemEnt = player.GetInventory().CreateInInventory( feetArray[rndFeetIndex] );
		SetRandomHealth( itemEnt );

		// Reserve kit cargo before placing stock consumables into the remaining cells.
		GivePlayerCombatKit(player, bodyClothing, legClothing);

		string chemlightArray[] = { "Chemlight_White", "Chemlight_Yellow", "Chemlight_Green", "Chemlight_Red" };
		int rndChemIndex = Math.RandomInt( 0, 4 );
		itemEnt = GiveSpawnCargo(player, chemlightArray[rndChemIndex], bodyClothing, 0, 2);
		if (itemEnt)
			player.SetQuickBarEntityShortcut(itemEnt, 1);
		SetRandomHealth( itemEnt );

		string sodaArray[] = { "SodaCan_Pipsi", "SodaCan_Spite", "SodaCan_Cola", "SodaCan_Fronta" };
		int rndSodaIndex = Math.RandomInt( 0, 4 );
		// Rotate the 1x2 soda into the bottom row of every 4x3 Shalwar variant.
		itemEnt = GiveSpawnCargo(player, sodaArray[rndSodaIndex], legClothing, 2, 0, true);
		SetRandomHealth( itemEnt );

		GiveSpawnCargo(player, "Figs", bodyClothing, 2, 2);

		player.GetStatWater().Set( 1000 );
		player.GetStatEnergy().Set( 800 );
	}
};

Mission CreateCustomMission(string path)
{
	return new CustomMission();
}