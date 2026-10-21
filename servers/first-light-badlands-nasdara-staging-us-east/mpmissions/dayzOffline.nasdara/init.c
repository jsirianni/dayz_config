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

	// Each category rolls once per new character; choices within a category are uniform.
	bool RollBonusGear()
	{
		return Math.RandomFloat01() < 0.05;
	}

	// Keep successful rolls even if the chosen clothing has insufficient cargo space.
	EntityAI GiveBonusItem(PlayerBase player, string className, EntityAI preferredContainer = null)
	{
		EntityAI item;
		if (preferredContainer)
			item = preferredContainer.GetInventory().CreateInInventory(className);
		if (!item)
			item = player.GetInventory().CreateInInventory(className);
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

	void GiveRandomBonusGear(PlayerBase player)
	{
		EntityAI item;
		ItemBase bottle;
		Weapon_Base pistol;
		Magazine magazine;
		Magazine buckshot;

		// Roll storage first, before adding the larger optional items.
		if (RollBonusGear())
		{
			string vests[] = { "LinenVest_Black", "LinenVest_Brown", "LinenVest_DarkGrey", "LinenVest_Green", "LinenVest_Grey", "LinenVest_NavyBlue", "RobeVest_Beige", "RobeVest_Brown", "RobeVest_Green", "RobeVest_Grey", "RobeVest_Navy", "ChestRigVest_Khaki", "ChestRigVest_Olive", "ChestRigVest_Tan" };
			GiveBonusItem(player, vests[Math.RandomInt(0, 14)]);
		}

		if (RollBonusGear())
		{
			string backpacks[] = { "CourierBag", "TaloonBag_Blue", "TaloonBag_Green", "TaloonBag_Orange", "TaloonBag_Violet" };
			GiveBonusItem(player, backpacks[Math.RandomInt(0, 5)]);
		}

		if (RollBonusGear())
		{
			item = GiveBonusItem(player, "HipPack_Medical");
			if (item)
				GiveBonusItem(player, "BandageDressing", item);
		}

		if (RollBonusGear())
		{
			string bottles[] = { "WaterBottle", "GlassBottle", "Canteen", "FilteringBottle", "SmallGlassBottle_Afza", "SmallGlassBottle_Jallab", "WineBottle", "AyranBottle" };
			bottle = ItemBase.Cast(GiveBonusItem(player, bottles[Math.RandomInt(0, 8)]));
			if (bottle)
			{
				bottle.RemoveAllAgents();
				bottle.SetLiquidType(LIQUID_WATER);
				bottle.SetQuantityMax();
			}
		}

		if (RollBonusGear())
		{
			pistol = Weapon_Base.Cast(GiveBonusItem(player, "MakarovIJ70"));
			if (pistol)
			{
				magazine = pistol.SpawnAttachedMagazine("Mag_IJ70_8Rnd", WeaponWithAmmoFlags.CHAMBER);
				if (magazine)
					magazine.SetHealth01("", "", 1.0);
				else
					Print("[SpawnGear] Unable to load IJ-70 magazine");
			}
		}

		if (RollBonusGear())
			GiveBonusItem(player, "KitchenKnife");

		if (RollBonusGear())
		{
			string goggles[] = { "DustGoggles_Black", "DustGoggles_Transparent", "TacticalGoggles", "SkiGoggles_BlackYellow", "SkiGoggles_WhiteClear", "SkiGoggles_WhiteDark" };
			GiveBonusItem(player, goggles[Math.RandomInt(0, 6)]);
		}

		if (RollBonusGear())
		{
			string masks[] = { "Keffiyeh_Black", "Keffiyeh_Blue", "Keffiyeh_Brown", "Keffiyeh_Gray", "Keffiyeh_LightBlue", "Keffiyeh_LightBrown", "Keffiyeh_LightRed", "Keffiyeh_Red", "BalaclavaMask_Beige", "BalaclavaMask_Black", "BalaclavaMask_Green", "SurgicalMask", "NioshFaceMask" };
			GiveBonusItem(player, masks[Math.RandomInt(0, 13)]);
		}

		if (RollBonusGear())
		{
			item = GiveBonusItem(player, "Izh18Shotgun");
			if (item)
			{
				buckshot = Magazine.Cast(GiveBonusItem(player, "Ammo_12gaPellets"));
				if (buckshot)
					buckshot.ServerSetAmmoCount(buckshot.GetAmmoMax());
			}
		}

		if (RollBonusGear())
			GiveBonusItem(player, "Roadflare");

		if (RollBonusGear())
		{
			string compasses[] = { "Compass", "OrienteeringCompass" };
			GiveBonusItem(player, compasses[Math.RandomInt(0, 2)]);
		}

		// No magazine or ammunition was requested for the Lee-Enfield bonus.
		if (RollBonusGear())
			GiveBonusItem(player, "LeeEnfield");
	}

	override void StartingEquipSetup(PlayerBase player, bool clothesChosen)
	{
		player.RemoveAllItems();

		EntityAI itemClothing;
		EntityAI itemEnt;
		ItemBase itemBs;
		float rand;

		string bodyArray[] = { "KameezShirt_Beige", "KameezShirt_Black", "KameezShirt_BlueFlowers", "KameezShirt_Brown", "KameezShirt_Grey", "KameezShirt_NavyBlue", "KameezShirt_Olive", "KameezShirt_Orange", "KameezShirt_Pink", "KameezShirt_Purple", "KameezShirt_Red", "KameezShirt_SkyBlue", "KameezShirt_White", "KameezShirt_Yellow" };
		int rndBodyIndex = Math.RandomInt( 0, 14 );
		itemEnt = player.GetInventory().CreateInInventory( bodyArray[rndBodyIndex] );
		SetRandomHealth( itemEnt );

		string legsArray[] = { "MensShalwarPants_Beige", "MensShalwarPants_Black", "MensShalwarPants_Brown", "MensShalwarPants_Grey", "MensShalwarPants_NavyBlue", "MensShalwarPants_SkyBlue", "MensShalwarPants_White", "WomensShalwarPants_NavyBlue", "WomensShalwarPants_Olive", "WomensShalwarPants_Orange", "WomensShalwarPants_Pink", "WomensShalwarPants_Purple", "WomensShalwarPants_Red" };
		int rndLegsIndex = Math.RandomInt( 0, 13 );
		itemEnt = player.GetInventory().CreateInInventory( legsArray[rndLegsIndex] );
		SetRandomHealth( itemEnt );

		string feetArray[] = { "SandalsShoes_Black", "SandalsShoes_BlueNavy", "SandalsShoes_Brown", "SandalsShoes_Grey", "TraditionalSandalShoes_Black", "TraditionalSandalShoes_Brown", "TraditionalSandalShoes_LightBrown", "TraditionalSandalShoes_NavyBlue" };
		int rndFeetIndex = Math.RandomInt( 0, 8 );
		itemEnt = player.GetInventory().CreateInInventory( feetArray[rndFeetIndex] );
		SetRandomHealth( itemEnt );

		string chemlightArray[] = { "Chemlight_White", "Chemlight_Yellow", "Chemlight_Green", "Chemlight_Red" };
		int rndChemIndex = Math.RandomInt( 0, 4 );
		itemEnt = player.GetInventory().CreateInInventory( chemlightArray[rndChemIndex] );
		player.SetQuickBarEntityShortcut(itemEnt, 1);
		SetRandomHealth( itemEnt );

		string sodaArray[] = { "SodaCan_Pipsi", "SodaCan_Spite", "SodaCan_Cola", "SodaCan_Fronta" };
		int rndSodaIndex = Math.RandomInt( 0, 4 );
		itemEnt = player.GetInventory().CreateInInventory( sodaArray[rndSodaIndex] );
		SetRandomHealth( itemEnt );

		itemEnt = player.GetInventory().CreateInInventory( "Figs" );
		itemEnt = player.GetInventory().CreateInInventory( "BandageDressing" );
		player.SetQuickBarEntityShortcut(itemEnt, 2);

		player.GetStatWater().Set( 1000 );
		player.GetStatEnergy().Set( 800 );

		GiveRandomBonusGear(player);
	}
};

Mission CreateCustomMission(string path)
{
	return new CustomMission();
}