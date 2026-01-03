void main()
{
	//INIT ECONOMY--------------------------------------
	Hive ce = CreateHive();
	if ( ce )
		ce.InitOffline();

	//DATE RESET AFTER ECONOMY INIT-------------------------
	int year, month, day, hour, minute;
	int reset_month = 2, reset_day = 1;
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
			float rndHlt = Math.RandomFloat( 0.25, 0.65 );
			itemEnt.SetHealth01( "", "", rndHlt );
		}
	}
	
	void SetLowHealth(EntityAI itemEnt)
	{
		if ( itemEnt )
		{
			float rndHlt = Math.RandomFloat( 0.15, 0.35 );
			itemEnt.SetHealth01( "", "", rndHlt );
		}
	}
	
	void SetQuantity(EntityAI itemEnt)
	{
		if ( itemEnt )
		{
			float rndHlt = Math.RandomInt( 1, 5 );
			itemEnt.SetQuantity(rndHlt);
		}
	}
	
	void ClearDefaultGear(PlayerBase player)
	{
		InventoryLocation il = new InventoryLocation;
		for ( int i = player.GetInventory().AttachmentCount() - 1; i >= 0; i-- )
		{
			EntityAI att = player.GetInventory().GetAttachmentFromIndex(i);
			if ( att )
			{
				att.GetInventory().GetCurrentInventoryLocation( il );
				att.Delete();
			}
		}
	}
	
	EntityAI EquipClothing(PlayerBase player, string typeName)
	{
		EntityAI clothing = player.GetInventory().CreateAttachment( typeName );
		SetRandomHealth( clothing );
		return clothing;
	}
	
	EntityAI AddItem(PlayerBase player, string typeName, int quickbarIndex = -1)
	{
		EntityAI item = player.GetInventory().CreateInInventory( typeName );
		SetRandomHealth( item );
		
		if ( quickbarIndex > -1 && item )
		{
			player.SetQuickBarEntityShortcut( item, quickbarIndex );
		}
		
		return item;
	}
	
	void AddItemMultiple(PlayerBase player, string typeName, int count, int quickbarStart = -1)
	{
		for ( int i = 0; i < count; i++ )
		{
			AddItem( player, typeName, quickbarStart > -1 ? quickbarStart + i : -1 );
		}
	}
	
	void SetupMedic(PlayerBase player)
	{
		string tops[]   = { "ParamedicJacket_Blue", "ParamedicJacket_Crimson", "ParamedicJacket_Green" };
		string pants[]  = { "ParamedicPants_Blue", "ParamedicPants_Crimson", "ParamedicPants_Green" };
		string gloves[] = { "SurgicalGloves_Blue", "SurgicalGloves_Green" };
		string shoes[]  = { "AthleticShoes_Black", "AthleticShoes_Blue" };
		
		EquipClothing( player, tops[ Math.RandomInt( 0, tops.Count() ) ] );
		EquipClothing( player, pants[ Math.RandomInt( 0, pants.Count() ) ] );
		EquipClothing( player, gloves[ Math.RandomInt( 0, gloves.Count() ) ] );
		EquipClothing( player, shoes[ Math.RandomInt( 0, shoes.Count() ) ] );
		EquipClothing( player, "SurgicalMask" );
		
		AddItemMultiple( player, "BandageDressing", 2, 0 );
		AddItemMultiple( player, "SalineBagIV", 2, 2 );
		AddItem( player, "VitaminBottle", 4 );
		AddItem( player, "TetracyclineAntibiotics", 5 );
	}
	
	void SetupPoliceOfficer(PlayerBase player)
	{
		string tops[]  = { "PoliceJacket", "PoliceJacketOrel" };
		string pants[] = { "PolicePants", "PolicePantsOrel" };
		
		EquipClothing( player, "PoliceCap" );
		EquipClothing( player, tops[ Math.RandomInt( 0, tops.Count() ) ] );
		EquipClothing( player, pants[ Math.RandomInt( 0, pants.Count() ) ] );
		EquipClothing( player, "PoliceVest" );
		EquipClothing( player, "WorkingBoots_Brown" );
		EquipClothing( player, "WorkingGloves_Black" );
		
		int weaponRoll = Math.RandomInt( 0, 2 );
		if ( weaponRoll == 0 )
		{
			EntityAI pistol = player.GetHumanInventory().CreateInHands( "MakarovIJ70" );
			SetRandomHealth( pistol );
			player.SetQuickBarEntityShortcut( pistol, 0 );
			AddItemMultiple( player, "Mag_IJ70_8Rnd", 2, 1 );
		}
		else
		{
			EntityAI shotgun = player.GetHumanInventory().CreateInHands( "Mp133Shotgun" );
			SetRandomHealth( shotgun );
			player.SetQuickBarEntityShortcut( shotgun, 0 );
			AddItemMultiple( player, "AmmoBox_00buck_10rnd", 2, 1 );
		}
		
		AddItem( player, "BandageDressing", 2 );
		AddItem( player, "Crowbar", 3 );
	}
	
	void SetupFirefighter(PlayerBase player)
	{
		string jackets[] = { "FirefighterJacket_Beige", "FirefighterJacket_Black" };
		
		EquipClothing( player, jackets[ Math.RandomInt( 0, jackets.Count() ) ] );
		EquipClothing( player, "FirefightersHelmet_Red" );
		EquipClothing( player, "FirefightersPants_Black" );
		EquipClothing( player, "WorkingBoots_Brown" );
		EquipClothing( player, "WorkingGloves_Beige" );
		
		EntityAI axe = player.GetHumanInventory().CreateInHands( "FirefighterAxe" );
		SetRandomHealth( axe );
		player.SetQuickBarEntityShortcut( axe, 0 );
		
		AddItemMultiple( player, "BandageDressing", 2, 1 );
	}

	override PlayerBase CreateCharacter(PlayerIdentity identity, vector pos, ParamsReadContext ctx, string characterName)
	{
		Entity playerEnt;
		playerEnt = GetGame().CreatePlayer( identity, characterName, pos, 0, "NONE" );
		Class.CastTo( m_player, playerEnt );

		GetGame().SelectPlayer( identity, m_player );

		return m_player;
	}

	override void StartingEquipSetup(PlayerBase player, bool clothesChosen)
	{
		ClearDefaultGear( player );
		
		int loadoutChoice = Math.RandomInt( 0, 3 );
		
		switch ( loadoutChoice )
		{
			case 0:
				SetupMedic( player );
				break;
			case 1:
				SetupPoliceOfficer( player );
				break;
			case 2:
				SetupFirefighter( player );
				break;
		}
		
		player.GetStatWater().Set( 750 );
		player.GetStatEnergy().Set( 1000 );
		
		player.SetTemporaryResistanceToAgent(eAgents.INFLUENZA, 900);
	}
};

Mission CreateCustomMission(string path)
{
	return new CustomMission();
}