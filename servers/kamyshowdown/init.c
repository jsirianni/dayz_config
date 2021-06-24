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

	// 0 or less is always false
	// 100 or more is always true
	// the higher the value inbetween, the more likely false is returned
	int decide(int percentChance) 
	{
		if (percentChance >= 100) {
			return 1;
		}
		if (percentChance <= 0) {
			return 0;
		}
		int arr[100] = {};
		for (int i = 1; i < 100; i++) {
			if (i < percentChance) { 
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
		TStringArray vest = {"PlateCarrierVest"}
		TStringArray shoes = {"AthleticShoes_Black","AthleticShoes_Brown","AthleticShoes_Grey","HikingBootsLow_Beige","HikingBootsLow_Black","HikingBootsLow_Grey","HikingBoots_Black","HikingJacket_Black"};
		TStringArray gun = {"Flaregun","MakarovIJ70","FNX45","Glock19","MKII","Colt1911","Engraved1911","Izh18","Mosin9130","CZ527","Winchester70","SKS", "Mp133Shotgun","Izh43Shotgun","Saiga", "CZ61","UMP45","MP5K","AKS74U", "FAL","AKM","AK101","AK74","M4A1","VSS","B95","SVD"};
		TStringArray mellee = {"BrassKnuckles_Shiny","StunBaton","CattleProd",  "BaseballBat", "NailedBaseballBat"};

		player.RemoveAllItems();

		player.GetInventory().CreateInInventory(top.GetRandomElement());
		player.GetInventory().CreateInInventory(pants.GetRandomElement());
		player.GetInventory().CreateInInventory(shoes.GetRandomElement());

		// a vest should be rare
		if (decide(90) == true) {
			player.GetInventory().CreateInInventory(vest.GetRandomElement());
		}

		switch(gun.GetRandomElement()) {
		  case "Flaregun":
			player.GetInventory().CreateInInventory("Flaregun");
			player.GetInventory().CreateInInventory("Ammo_Flare");
			break;
		  case "MakarovIJ70":
		  	player.GetInventory().CreateInInventory("MakarovIJ70");
			player.GetInventory().CreateInInventory("Mag_IJ70_8Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			break;
		  case "FNX45":
			player.GetInventory().CreateInInventory("FNX45");
			player.GetInventory().CreateInInventory("Mag_FNX45_15Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolOptic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			break;
		  case "Glock19":
			player.GetInventory().CreateInInventory("Glock19");
			player.GetInventory().CreateInInventory("Mag_Glock_15Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolOptic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			break;
		  case "MKII":
			player.GetInventory().CreateInInventory("MKII");
			player.GetInventory().CreateInInventory("Mag_MKII_10Rnd");
			break;
		  case "Colt1911":
			player.GetInventory().CreateInInventory("Colt1911");
			player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			break;
		  case "Engraved1911":
			player.GetInventory().CreateInInventory("Engraved1911");
			player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			break;
		  case "Izh18":
			player.GetInventory().CreateInInventory("Izh18");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			break;
		  case "Mosin9130":
			player.GetInventory().CreateInInventory("Mosin9130");
			player.GetInventory().CreateInInventory("Ammo_762x54");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PUScopeOptic");
			}
			break;
		  case "CZ527":
			player.GetInventory().CreateInInventory("CZ527");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			break;
		  case "Winchester70":
			player.GetInventory().CreateInInventory("Winchester70");
			player.GetInventory().CreateInInventory("Ammo_308Win");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("HuntingOptic");
			}
			break;
		  case "SKS":
			player.GetInventory().CreateInInventory("SKS");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PUScopeOptic");
			}
			break;
		  case "Mp133Shotgun":
			player.GetInventory().CreateInInventory("Mp133Shotgun");
			player.GetInventory().CreateInInventory("Ammo_12gaSlug");
			break;
		  case "Izh43Shotgun":
			player.GetInventory().CreateInInventory("Izh43Shotgun");
			player.GetInventory().CreateInInventory("Ammo_12gaSlug");
			break;
		  case "Saiga":
			player.GetInventory().CreateInInventory("Saiga");
			TStringArray mag = {"Mag_Saiga_5Rnd","Mag_Saiga_8Rnd","Mag_Saiga_Drum20Rnd"}
			player.GetInventory().CreateInInventory(mag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			break;
		  case "CZ61":
			player.GetInventory().CreateInInventory("CZ61");
			player.GetInventory().CreateInInventory("Mag_CZ61_20Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			break;
		  case "UMP45":
			player.GetInventory().CreateInInventory("UMP45");
			player.GetInventory().CreateInInventory("Mag_UMP_25Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				TStringArray optic = {"ACOGOptic","M4_T3NRDSOptic"}
				player.GetInventory().CreateInInventory(optic.GetRandomElement());
			}
			break;
		  case "MP5K":
			player.GetInventory().CreateInInventory("MP5K");
			player.GetInventory().CreateInInventory("Mag_MP5_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				TStringArray optic = {"ACOGOptic","M4_T3NRDSOptic"}
				player.GetInventory().CreateInInventory(optic.GetRandomElement());
			}
			break;
		  case "AKS74U":
			player.GetInventory().CreateInInventory("AKS74U");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			break;
		  case "FAL":
			player.GetInventory().CreateInInventory("FAL");
			player.GetInventory().CreateInInventory("Mag_FAL_20Rnd");
			if (decide(70) == true) {
				TStringArray optic = {"ACOGOptic","M4_T3NRDSOptic"}
				player.GetInventory().CreateInInventory(optic.GetRandomElement());
			}
			break;
		  case "AKM":
			player.GetInventory().CreateInInventory("AKM");
			TStringArray mag = {"Mag_AKM_30Rnd","Mag_AKM_Palm30Rnd","Mag_AKM_Drum75Rnd"}
			player.GetInventory().CreateInInventory(mag.GetRandomElement());
			if (decide(70) == true) {
				TStringArray optic = {"PSO11Optic","KobraOptic"}
				player.GetInventory().CreateInInventory(optic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			break;
		  case "AK101":
			player.GetInventory().CreateInInventory("AK101");
			player.GetInventory().CreateInInventory("Mag_AK101_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			break;
		  case "AK74":
			player.GetInventory().CreateInInventory("AK74");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			if (decide(70) == true) {
				TStringArray optic = {"PSO11Optic","KobraOptic"}
				player.GetInventory().CreateInInventory(optic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			break;
		  case "M4A1":
			player.GetInventory().CreateInInventory("M4A1");
			TStringArray mag = {"Mag_STANAG_30Rnd","Mag_STANAGCoupled_30Rnd","Mag_CMAG_10Rnd","Mag_CMAG_20Rnd","Mag_CMAG_30Rnd","Mag_CMAG_40Rnd"}
			player.GetInventory().CreateInInventory(mag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(mag.GetRandomElement());
			}
			if (decide(70) == true) {
				TStringArray optic = {"ACOGOptic","M4_T3NRDSOptic"}
				player.GetInventory().CreateInInventory(optic.GetRandomElement());
			}
			break;
		  case "VSS":
			player.GetInventory().CreateInInventory("VSS");
			player.GetInventory().CreateInInventory("Mag_VSS_10Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			break;
		  case "B95":
			player.GetInventory().CreateInInventory("B95");
			player.GetInventory().CreateInInventory("Ammo_308Win");
			break;
		  case "SVD":
			player.GetInventory().CreateInInventory("SVD");
			player.GetInventory().CreateInInventory("Mag_SVD_10Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			break;
		  default:
			player.GetInventory().CreateInInventory("AK74");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
		}
		
		player.GetInventory().CreateInInventory(mellee.GetRandomElement());
	}
};

Mission CreateCustomMission(string path)
{
	return new CustomMission();
}