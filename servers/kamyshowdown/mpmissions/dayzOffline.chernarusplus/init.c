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

		switch(gun.GetRandomElement()) {
		  case "Flaregun":
			player.GetInventory().CreateInInventory("Flaregun");
			player.GetInventory().CreateInInventory("Ammo_Flare");
			break;
		  case "MakarovIJ70":
		  	EntityAI weaponMakarovIJ70 = player.GetHumanInventory().CreateInHands("MakarovIJ70");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponMakarovIJ70.GetInventory().CreateAttachment("Mag_IJ70_8Rnd");
			player.GetInventory().CreateInInventory("Mag_IJ70_8Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_IJ70_8Rnd");
			}
			break;
		  case "FNX45":
			EntityAI weaponFNX45 = player.GetHumanInventory().CreateInHands("FNX45");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponFNX45.GetInventory().CreateAttachment("Mag_FNX45_15Rnd");
			player.GetInventory().CreateInInventory("Mag_FNX45_15Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolOptic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_FNX45_15Rnd");
			}
			break;
		  case "Glock19":
			EntityAI weaponGlock19 = player.GetHumanInventory().CreateInHands("Glock19");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponGlock19.GetInventory().CreateAttachment("Mag_Glock_15Rnd");
			player.GetInventory().CreateInInventory("Mag_Glock_15Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolOptic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_Glock_15Rnd");
			}
			break;
		  case "MKII":
			EntityAI weaponMKII = player.GetHumanInventory().CreateInHands("MKII");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponMKII.GetInventory().CreateAttachment("Mag_MKII_10Rnd");
			player.GetInventory().CreateInInventory("Mag_MKII_10Rnd");
			if (decide(10) == true) {
				player.GetInventory().CreateInInventory("Mag_MKII_10Rnd");
			}
			break;
		  case "Colt1911":
			EntityAI weaponColt1911 = player.GetHumanInventory().CreateInHands("Colt1911");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponColt1911.GetInventory().CreateAttachment("Mag_1911_7Rnd");
			player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(50) == true) {
				player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			}
			break;
		  case "Engraved1911":
			EntityAI weaponEngraved1911 = player.GetHumanInventory().CreateInHands("Engraved1911");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponEngraved1911.GetInventory().CreateAttachment("Mag_1911_7Rnd");
			player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(50) == true) {
				player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			}
			break;
		  case "Izh18":
			EntityAI weaponIzh18 = player.GetHumanInventory().CreateInHands("Izh18");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponIzh18.GetInventory().CreateAttachment("Ammo_762x39");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			break;
		  case "Mosin9130":
			EntityAI weaponMosin9130 = player.GetHumanInventory().CreateInHands("Mosin9130");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponMosin9130.GetInventory().CreateAttachment("Ammo_762x54");
			player.GetInventory().CreateInInventory("Ammo_762x54");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PUScopeOptic");
			}
			break;
		  case "CZ527":
			EntityAI weaponCZ527 = player.GetHumanInventory().CreateInHands("CZ527");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponCZ527.GetInventory().CreateAttachment("Ammo_762x39");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			break;
		  case "Winchester70":
			EntityAI weaponWinchester70 = player.GetHumanInventory().CreateInHands("Winchester70");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponWinchester70.GetInventory().CreateAttachment("Mag_IJ70_8Rnd");
			player.GetInventory().CreateInInventory("Ammo_308Win");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("HuntingOptic");
			}
			break;
		  case "SKS":
			EntityAI weaponSKS = player.GetHumanInventory().CreateInHands("SKS");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponSKS.GetInventory().CreateAttachment("Ammo_762x39");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PUScopeOptic");
			}
			if (decide(50) == true) {
				player.GetInventory().CreateInInventory("Ammo_762x39");
			}
			break;
		  case "Mp133Shotgun":
			EntityAI weaponMp133Shotgun = player.GetHumanInventory().CreateInHands("Mp133Shotgun");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			player.GetInventory().CreateInInventory(shotgunAmmo.GetRandomElement());
			break;
		  case "Izh43Shotgun":
			EntityAI weaponIzh43Shotgun = player.GetHumanInventory().CreateInHands("Izh43Shotgun");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			player.GetInventory().CreateInInventory(shotgunAmmo.GetRandomElement());
			break;
		  case "Saiga":
			EntityAI weaponSaiga = player.GetHumanInventory().CreateInHands("Saiga");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			TStringArray saigaMag = {"Mag_Saiga_5Rnd","Mag_Saiga_8Rnd","Mag_Saiga_Drum20Rnd"}
			weaponSaiga.GetInventory().CreateAttachment(saigaMag.GetRandomElement());
			player.GetInventory().CreateInInventory(saigaMag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(saigaMag.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			break;
		  case "CZ61":
			EntityAI weaponCZ61 = player.GetHumanInventory().CreateInHands("CZ61");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponCZ61.GetInventory().CreateAttachment("Mag_CZ61_20Rnd");
			player.GetInventory().CreateInInventory("Mag_CZ61_20Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_CZ61_20Rnd");
			}
			break;
		  case "UMP45":
			EntityAI weaponUMP45 = player.GetHumanInventory().CreateInHands("UMP45");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponUMP45.GetInventory().CreateAttachment("Mag_UMP_25Rnd");
			player.GetInventory().CreateInInventory("Mag_UMP_25Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_UMP_25Rnd");
			}
			break;
		  case "MP5K":
			EntityAI weaponMP5K = player.GetHumanInventory().CreateInHands("MP5K");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponMP5K.GetInventory().CreateAttachment("Mag_MP5_30Rnd");
			player.GetInventory().CreateInInventory("Mag_MP5_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_MP5_30Rnd");
			}
			break;
		  case "AKS74U":
			EntityAI weaponAKS74U = player.GetHumanInventory().CreateInHands("AKS74U");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponAKS74U.GetInventory().CreateAttachment("Mag_AK74_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			}
			break;
		  case "FAL":
			EntityAI weaponFAL = player.GetHumanInventory().CreateInHands("FAL");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponFAL.GetInventory().CreateAttachment("Mag_FAL_20Rnd");
			player.GetInventory().CreateInInventory("Mag_FAL_20Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_FAL_20Rnd");
			}
			break;
		  case "AKM":
			EntityAI weaponAKM = player.GetHumanInventory().CreateInHands("AKM");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			TStringArray akmMag = {"Mag_AKM_30Rnd","Mag_AKM_Palm30Rnd","Mag_AKM_Drum75Rnd"}
			weaponAKM.GetInventory().CreateAttachment(akmMag.GetRandomElement());
			player.GetInventory().CreateInInventory(akmMag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(akmMag.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(sovietOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			break;
		  case "AK101":
			EntityAI weaponAK101 = player.GetHumanInventory().CreateInHands("AK101");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponAK101.GetInventory().CreateAttachment("Mag_AK101_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK101_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_AK101_30Rnd");
			}
			break;
		  case "AK74":
			EntityAI weaponAK74 = player.GetHumanInventory().CreateInHands("AK74");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponAK74.GetInventory().CreateAttachment("Mag_AK74_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(sovietOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			}
			break;
		  case "M4A1":
			EntityAI weaponM4A1 = player.GetHumanInventory().CreateInHands("M4A1");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			TStringArray m4Mag = {"Mag_STANAG_30Rnd","Mag_STANAGCoupled_30Rnd","Mag_CMAG_10Rnd","Mag_CMAG_20Rnd","Mag_CMAG_30Rnd","Mag_CMAG_40Rnd"}
			weaponM4A1.GetInventory().CreateAttachment(m4Mag.GetRandomElement());
			player.GetInventory().CreateInInventory(m4Mag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(m4Mag.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("M4_Suppressor");
			}
			break;
		  case "VSS":
			EntityAI weaponVSS = player.GetHumanInventory().CreateInHands("VSS");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponVSS.GetInventory().CreateAttachment("Mag_VSS_10Rnd");
			player.GetInventory().CreateInInventory("Mag_VSS_10Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_VSS_10Rnd");
			}			
			break;
		  case "B95":
			EntityAI weaponB95 = player.GetHumanInventory().CreateInHands("B95");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponB95.GetInventory().CreateAttachment("Ammo_308Win");
			player.GetInventory().CreateInInventory("Ammo_308Win");
			break;
		  case "SVD":
			EntityAI weaponSVD = player.GetHumanInventory().CreateInHands("SVD");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponSVD.GetInventory().CreateAttachment("Mag_SVD_10Rnd");
			player.GetInventory().CreateInInventory("Mag_SVD_10Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("PSO11Optic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_SVD_10Rnd");
			}	
			break;
		  default:
			EntityAI weaponDefault = player.GetHumanInventory().CreateInHands("AK74");
			// player.SetQuickBarEntityShortcut(weapon, 0);
			weaponDefault.GetInventory().CreateAttachment("Mag_AK74_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
		}
		
		player.GetInventory().CreateInInventory(mellee.GetRandomElement());
	}
};

Mission CreateCustomMission(string path)
{
	return new CustomMission();
}
