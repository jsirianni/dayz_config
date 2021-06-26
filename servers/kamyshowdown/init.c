void main()
{
	//INIT WEATHER BEFORE ECONOMY INIT------------------------
	Weather weather = g_Game.GetWeather();

	weather.MissionWeather(false);    // false = use weather controller from Weather.c

	weather.GetOvercast().Set( Math.RandomFloatInclusive(0.4, 0.6), 1, 0);
	weather.GetRain().Set( 0, 0, 1);
	weather.GetFog().Set( Math.RandomFloatInclusive(0.05, 0.1), 1, 0);

	//INIT ECONOMY--------------------------------------
	Hive ce = CreateHive();
	if ( ce )
		ce.InitOffline();

	//DATE RESET AFTER ECONOMY INIT-------------------------
	int year, month, day, hour, minute;
	int reset_month = 9, reset_day = 20;
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

	// Function to decide true or false based on percent chance to be false
	int decide(int percentChanceFalse) 
	{
		if (percentChanceFalse >= 100) {
			return 0;
		}
		if (percentChanceFalse <= 0) {
			return 1;
		}
		int arr[100] = {};
		for (int i = 1; i < 100; i++) {
			if (i < percentChanceFalse) { 
				arr[i] = 0;
			}
			else {
				arr[i] = 1;
			}
		}
		int pickMe = Math.RandomInt(0, 100);
		return arr[pickMe];
	}

	override void StartingEquipSetup(PlayerBase player, bool clothesChosen)
	{
		TStringArray top = {"PrisonUniformJacket","BDUJacket","NurseDress_Blue","Raincoat_Green","Raincoat_Pink"};
		TStringArray pants = {"PrisonUniformPants","BDUPants","CargoPants_Beige","ShortJeans_Blue"};
		TStringArray vest = {"PlateCarrierVest","PoliceVest","PressVest_Blue"}
		TStringArray shoes = {"AthleticShoes_Black","AthleticShoes_Brown","AthleticShoes_Grey","HikingBootsLow_Beige","HikingBootsLow_Black","HikingBootsLow_Grey","HikingBoots_Black","HikingJacket_Black"};
		TStringArray gun = {"Flaregun","MakarovIJ70","FNX45","Glock19","MKII","Colt1911","Engraved1911","Izh18","Mosin9130","CZ527","Winchester70","SKS", "Mp133Shotgun","Izh43Shotgun","Saiga", "CZ61","UMP45","MP5K","AKS74U", "FAL","AKM","AK101","AK74","M4A1","VSS","B95","SVD"};
		TStringArray mellee = {"BrassKnuckles_Shiny","StunBaton","CattleProd",  "BaseballBat", "NailedBaseballBat"};
		TStringArray shotgunAmmo = {"Ammo_12gaPellets","Ammo_12gaRubberSlug","Ammo_12gaSlug"}
		TStringArray natoOptic = {"ACOGOptic","M4_T3NRDSOptic"}
		TStringArray sovietOptic = {"PSO11Optic","KobraOptic"}
		TStringArray helmet = {"BallisticHelmet_Black","BallisticHelmet_UN","ConstructionHelmet_Blue","DarkMotoHelmet_Blue","FirefightersHelmet_Yellow","GorkaHelmet"}
		TStringArray medical = {"Bandage","Rag"}
		TStringArray nades = {"RDG2SmokeGrenade_Black","RGD5Grenade","FlashGrenade"}

		player.RemoveAllItems();
		player.GetInventory().CreateInInventory(top.GetRandomElement());
		player.GetInventory().CreateInInventory(pants.GetRandomElement());
		player.GetInventory().CreateInInventory(shoes.GetRandomElement());

		// a vest should be rare
		if (decide(80) == true) {
			player.GetInventory().CreateInInventory(vest.GetRandomElement());
		}

		if (decide(70) == true) {
			player.GetInventory().CreateInInventory(helmet.GetRandomElement());
		}

		if (decide(50) == true) {
			player.GetInventory().CreateInInventory(medical.GetRandomElement());
		}

		if (decide(70) == true) {
			player.GetInventory().CreateInInventory(nades.GetRandomElement());
		}

		// function given string
		// gives player a random gun
		givePlayerRandomGun(player, gun.GetRandomElement());
	};
};

Mission CreateCustomMission(string path)
{
	return new CustomMission();
}