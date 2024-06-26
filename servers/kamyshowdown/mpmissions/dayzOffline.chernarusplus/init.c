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
		player.GetStatEnergy().Set(5000);
		player.GetStatWater().Set(5000);

		EntityAI weapon;
		EntityAI weapon2;
		EntityAI weapon3;
		TStringArray m4Mag = {"Mag_STANAG_30Rnd","Mag_STANAGCoupled_30Rnd","Mag_CMAG_10Rnd","Mag_CMAG_20Rnd","Mag_CMAG_30Rnd","Mag_CMAG_40Rnd"}

		// 1% chance of getting jugg
		if (decide(95) == true) {
			player.RemoveAllItems();
			player.GetInventory().CreateInInventory("Chainmail_Coif");
			player.GetInventory().CreateInInventory("Chainmail");
			player.GetInventory().CreateInInventory("Chainmail_Leggings");
			player.GetInventory().CreateInInventory("MedievalBoots");	
			player.GetInventory().CreateInInventory("PlateCarrierVest");
			player.GetInventory().CreateInInventory("DryBag_Black");	
			player.GetInventory().CreateInInventory("BandageDressing");	
			player.GetInventory().CreateInInventory("BandageDressing");	

			EntityAI nade0 = player.GetInventory().CreateInInventory("RGD5Grenade");	
			EntityAI nade1 = player.GetInventory().CreateInInventory("RGD5Grenade");	
			EntityAI nade2 = player.GetInventory().CreateInInventory("RGD5Grenade");	
			EntityAI nade3 = player.GetInventory().CreateInInventory("RGD5Grenade");
			player.SetQuickBarEntityShortcut(nade0, 3);
			player.SetQuickBarEntityShortcut(nade1, 4);
			player.SetQuickBarEntityShortcut(nade2, 5);
			player.SetQuickBarEntityShortcut(nade3, 6);

			weapon = player.GetHumanInventory().CreateInHands("M4A1");
			weapon.GetInventory().CreateAttachment("Mag_STANAGCoupled_30Rnd");
			EntityAI optic = weapon.GetInventory().CreateAttachment("M4_T3NRDSOptic");
			optic.GetInventory().CreateAttachment("Battery9V");
			weapon.GetInventory().CreateAttachment("M4_Suppressor");
			weapon.GetInventory().CreateAttachment("M4_OEBttstck");
			weapon.GetInventory().CreateAttachment("M4_PlasticHndgrd");
			player.GetInventory().CreateInInventory("Mag_CMAG_40Rnd");
			player.GetInventory().CreateInInventory("Mag_CMAG_40Rnd");
			player.GetInventory().CreateInInventory("Mag_CMAG_40Rnd");
			player.SetQuickBarEntityShortcut(weapon, 0);

			weapon2 = player.GetInventory().CreateInInventory("Saiga");
			weapon2.GetInventory().CreateAttachment("Mag_Saiga_Drum20Rnd");
			player.GetInventory().CreateInInventory("Mag_Saiga_Drum20Rnd");
			weapon2.GetInventory().CreateAttachment("Saiga_Bttstck");
			player.SetQuickBarEntityShortcut(weapon2, 1);

			weapon3 = player.GetInventory().CreateInInventory("m79");
			player.GetInventory().CreateInInventory("Ammo_40mm_Explosive");
			player.GetInventory().CreateInInventory("Ammo_40mm_Explosive");
			player.GetInventory().CreateInInventory("Ammo_40mm_Explosive");
			player.SetQuickBarEntityShortcut(weapon3, 2);

			player.GetInventory().CreateInInventory("EpoxyPutty");
			player.GetInventory().CreateInInventory("Epinephrine");
			player.GetInventory().CreateInInventory("Epinephrine");
			player.GetInventory().CreateInInventory("SalineBagIV");
			player.GetInventory().CreateInInventory("BloodBagIV");
			player.GetInventory().CreateInInventory("BandageDressing");
			player.GetInventory().CreateInInventory("BandageDressing");
			player.GetInventory().CreateInInventory("Splint");
			
			

			player.GetInventory().CreateInInventory("Battery9V");

			return;
		}

		TStringArray top = {"PrisonUniformJacket","BDUJacket","NurseDress_Blue","Raincoat_Green","Raincoat_Pink"};
		TStringArray pants = {"PrisonUniformPants","BDUPants","CargoPants_Beige","ShortJeans_Blue"};
		TStringArray vest = {"PlateCarrierVest","PoliceVest","PressVest_Blue"};
		TStringArray shoes = {"AthleticShoes_Black","AthleticShoes_Brown","AthleticShoes_Grey","HikingBootsLow_Beige","HikingBootsLow_Black","HikingBootsLow_Grey","HikingBoots_Black"};
		TStringArray gun = {
			"MakarovIJ70",
			"FNX45",
			"Glock19",
			"MKII",
			"Colt1911",
			"Engraved1911",
			"Izh18",
			"Mosin9130",
			"CZ527",
			"Winchester70",
			"SKS",
			"Mp133Shotgun",
			"Izh43Shotgun",
			"Saiga",
			"CZ61",
			"UMP45",
			"MP5K",
			"AKS74U",
			"FAL",
			"AKM",
			"AK101",
			"AK74",
			"M4A1",
			"VSS",
			"B95",
			"SVD",
			// NEW
			"ASVAL",
			"Vikhr",
			"M16A2",
			"FAMAS",
			"Aug",
			"AugShort",
			"Deagle_Gold",
			"PP19",
			"M14",
			"m79",
			"Crossbow_Black"
		};
		TStringArray mellee = {"BrassKnuckles_Shiny", "BaseballBat", "NailedBaseballBat"};
		TStringArray shotgunAmmo = {"Ammo_12gaPellets","Ammo_12gaRubberSlug","Ammo_12gaSlug"};
		TStringArray natoOptic = {"ACOGOptic","M4_T3NRDSOptic"};
		TStringArray sovietOptic = {"PSO11Optic","KobraOptic"};
		TStringArray helmet = {"BallisticHelmet_Black","BallisticHelmet_UN","ConstructionHelmet_Blue","DarkMotoHelmet_Blue","FirefightersHelmet_Yellow","GorkaHelmet"};
		TStringArray medical = {"Bandage","Rag"};
		TStringArray nades = {"RDG2SmokeGrenade_Black","RGD5Grenade","FlashGrenade"};

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
		  case "MakarovIJ70":
		  	weapon = player.GetHumanInventory().CreateInHands("MakarovIJ70");
			weapon.GetInventory().CreateAttachment("Mag_IJ70_8Rnd");
			player.GetInventory().CreateInInventory("Mag_IJ70_8Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_IJ70_8Rnd");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("DryBag_Black");
				player.GetInventory().CreateInInventory("TripwireTrap");
				player.GetInventory().CreateInInventory("TripwireTrap");
				player.GetInventory().CreateInInventory("TripwireTrap");
				player.GetInventory().CreateInInventory("ClaymoreMine");
				player.GetInventory().CreateInInventory("ClaymoreMine");
				player.GetInventory().CreateInInventory("ClaymoreMine");
			}
			break;
		  case "FNX45":
			weapon = player.GetHumanInventory().CreateInHands("FNX45");
			weapon.GetInventory().CreateAttachment("Mag_FNX45_15Rnd");
			player.GetInventory().CreateInInventory("Mag_FNX45_15Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolOptic");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_FNX45_15Rnd");
			}
			break;
		  case "Glock19":
			weapon = player.GetHumanInventory().CreateInHands("Glock19");
			weapon.GetInventory().CreateAttachment("Mag_Glock_15Rnd");
			player.GetInventory().CreateInInventory("Mag_Glock_15Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolOptic");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_Glock_15Rnd");
			}
			break;
		  case "MKII":
			weapon = player.GetHumanInventory().CreateInHands("MKII");
			weapon.GetInventory().CreateAttachment("Mag_MKII_10Rnd");
			player.GetInventory().CreateInInventory("Mag_MKII_10Rnd");
			if (decide(10) == true) {
				player.GetInventory().CreateInInventory("Mag_MKII_10Rnd");
			}
			break;
		  case "Colt1911":
			weapon = player.GetHumanInventory().CreateInHands("Colt1911");
			weapon.GetInventory().CreateAttachment("Mag_1911_7Rnd");
			player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(50) == true) {
				player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			}
			break;
		  case "Engraved1911":
			weapon = player.GetHumanInventory().CreateInHands("Engraved1911");
			weapon.GetInventory().CreateAttachment("Mag_1911_7Rnd");
			player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(50) == true) {
				player.GetInventory().CreateInInventory("Mag_1911_7Rnd");
			}
			break;
		  case "Izh18":
			weapon = player.GetHumanInventory().CreateInHands("Izh18");
			weapon.GetInventory().CreateAttachment("Ammo_762x39");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			break;
		  case "Mosin9130":
			EntityAI weaponMosin9130 = player.GetHumanInventory().CreateInHands("Mosin9130");
			weaponMosin9130.GetInventory().CreateAttachment("Ammo_762x54");
			player.GetInventory().CreateInInventory("Ammo_762x54");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PUScopeOptic");
			}
			break;
		  case "CZ527":
			weapon = player.GetHumanInventory().CreateInHands("CZ527");
			weapon.GetInventory().CreateAttachment("Mag_CZ527_5rnd");
			player.GetInventory().CreateInInventory("Mag_CZ527_5rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("Mag_CZ527_5rnd");
			}
			break;
		  case "Winchester70":
			weapon = player.GetHumanInventory().CreateInHands("Winchester70");
			player.GetInventory().CreateInInventory("Ammo_308Win");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("HuntingOptic");
			}
			break;
		  case "SKS":
			weapon = player.GetHumanInventory().CreateInHands("SKS");
			weapon.GetInventory().CreateAttachment("Ammo_762x39");
			player.GetInventory().CreateInInventory("Ammo_762x39");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PUScopeOptic");
			}
			if (decide(50) == true) {
				player.GetInventory().CreateInInventory("Ammo_762x39");
			}
			break;
		  case "Mp133Shotgun":
			weapon = player.GetHumanInventory().CreateInHands("Mp133Shotgun");
			player.GetInventory().CreateInInventory(shotgunAmmo.GetRandomElement());
			break;
		  case "Izh43Shotgun":
			weapon = player.GetHumanInventory().CreateInHands("Izh43Shotgun");
			player.GetInventory().CreateInInventory(shotgunAmmo.GetRandomElement());
			break;
		  case "Saiga":
			weapon = player.GetHumanInventory().CreateInHands("Saiga");
			TStringArray saigaMag = {"Mag_Saiga_5Rnd","Mag_Saiga_8Rnd","Mag_Saiga_Drum20Rnd"}
			weapon.GetInventory().CreateAttachment(saigaMag.GetRandomElement());
			player.GetInventory().CreateInInventory(saigaMag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(saigaMag.GetRandomElement());
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PSO11Optic");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("Saiga_Bttstck");
			}
			break;
		  case "CZ61":
			weapon = player.GetHumanInventory().CreateInHands("CZ61");
			weapon.GetInventory().CreateAttachment("Mag_CZ61_20Rnd");
			player.GetInventory().CreateInInventory("Mag_CZ61_20Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_CZ61_20Rnd");
			}
			break;
		  case "UMP45":
			weapon = player.GetHumanInventory().CreateInHands("UMP45");
			weapon.GetInventory().CreateAttachment("Mag_UMP_25Rnd");
			player.GetInventory().CreateInInventory("Mag_UMP_25Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_UMP_25Rnd");
			}
			break;
		  case "MP5K":
			weapon = player.GetHumanInventory().CreateInHands("MP5K");
			weapon.GetInventory().CreateAttachment("Mag_MP5_30Rnd");
			player.GetInventory().CreateInInventory("Mag_MP5_30Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			} else if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("MP5_Compensator");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_MP5_30Rnd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("MP5_RailHndgrd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("MP5k_StockBttstck");
			}
			break;
		  case "AKS74U":
			weapon = player.GetHumanInventory().CreateInHands("AKS74U");
			weapon.GetInventory().CreateAttachment("Mag_AK74_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("AKS74U_Bttstck");
			}
			break;
		  case "FAL":
			weapon = player.GetHumanInventory().CreateInHands("FAL");
			weapon.GetInventory().CreateAttachment("Mag_FAL_20Rnd");
			player.GetInventory().CreateInInventory("Mag_FAL_20Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_FAL_20Rnd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("Fal_FoldingBttstck");
			}
			break;
		  case "AKM":
			weapon = player.GetHumanInventory().CreateInHands("AKM");
			TStringArray akmMag = {"Mag_AKM_30Rnd","Mag_AKM_Palm30Rnd","Mag_AKM_Drum75Rnd"}
			weapon.GetInventory().CreateAttachment(akmMag.GetRandomElement());
			player.GetInventory().CreateInInventory(akmMag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(akmMag.GetRandomElement());
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(sovietOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_Suppressor");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_PlasticBttstck");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_PlasticHndgrd");
			}
			break;
		  case "AK101":
			weapon = player.GetHumanInventory().CreateInHands("AK101");
			weapon.GetInventory().CreateAttachment("Mag_AK101_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK101_30Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PSO11Optic");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_AK101_30Rnd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_PlasticBttstck");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_PlasticHndgrd");
			}
			break;
		  case "AK74":
			weapon = player.GetHumanInventory().CreateInHands("AK74");
			weapon.GetInventory().CreateAttachment("Mag_AK74_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(sovietOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK74_WoodBttstck");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK74_Hndgrd");
			}
			break;
		  case "M4A1":
			weapon = player.GetHumanInventory().CreateInHands("M4A1");
			weapon.GetInventory().CreateAttachment(m4Mag.GetRandomElement());
			player.GetInventory().CreateInInventory(m4Mag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(m4Mag.GetRandomElement());
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("M4_Suppressor");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("M4_OEBttstck");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("M4_PlasticHndgrd");
			}
			break;
		  case "VSS":
			weapon = player.GetHumanInventory().CreateInHands("VSS");
			weapon.GetInventory().CreateAttachment("Mag_VSS_10Rnd");
			player.GetInventory().CreateInInventory("Mag_VSS_10Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PSO11Optic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_Vikhr_30Rnd");
			}			
			break;
		  case "B95":
			weapon = player.GetHumanInventory().CreateInHands("B95");
			weapon.GetInventory().CreateAttachment("Ammo_308Win");
			player.GetInventory().CreateInInventory("Ammo_308Win");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("HuntingOptic");
			}
			break;
		  case "SVD":
			weapon = player.GetHumanInventory().CreateInHands("SVD");
			weapon.GetInventory().CreateAttachment("Mag_SVD_10Rnd");
			player.GetInventory().CreateInInventory("Mag_SVD_10Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PSO11Optic");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("AK_Suppressor");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_SVD_10Rnd");
			}	
			break;
		  case "ASVAL":
			weapon = player.GetHumanInventory().CreateInHands("ASVAL");
			weapon.GetInventory().CreateAttachment("Mag_Vikhr_30Rnd");
			player.GetInventory().CreateInInventory("Mag_Vikhr_30Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PSO11Optic");
			}		
			break;
		  case "Vikhr":
			weapon = player.GetHumanInventory().CreateInHands("Vikhr");
			weapon.GetInventory().CreateAttachment("Mag_Vikhr_30Rnd");
			player.GetInventory().CreateInInventory("Mag_Vikhr_30Rnd");
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PSO11Optic");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_Vikhr_30Rnd");
			}	
			break;
		  case "M16A2":
			weapon = player.GetHumanInventory().CreateInHands("M16A2");
			weapon.GetInventory().CreateAttachment(m4Mag.GetRandomElement());
			player.GetInventory().CreateInInventory(m4Mag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(m4Mag.GetRandomElement());
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("M4_Suppressor");
			}
			break;
		  case "FAMAS":
			weapon = player.GetHumanInventory().CreateInHands("FAMAS");
			weapon.GetInventory().CreateAttachment("Mag_FAMAS_25Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_FAMAS_25Rnd");
			}
			break;
		  case "Aug":
			weapon = player.GetHumanInventory().CreateInHands("Aug");
			weapon.GetInventory().CreateAttachment("Mag_Aug_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_Aug_30Rnd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(natoOptic.GetRandomElement());
			}
			break;
		  case "AugShort":
			weapon = player.GetHumanInventory().CreateInHands("AugShort");
			weapon.GetInventory().CreateAttachment("Mag_Aug_30Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_Aug_30Rnd");
			}
			break;
		  case "Deagle_Gold":
			weapon = player.GetHumanInventory().CreateInHands("Deagle_Gold");
			weapon.GetInventory().CreateAttachment("Mag_Deagle_9rnd");
			player.GetInventory().CreateInInventory("Mag_Deagle_9rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_Deagle_9rnd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolOptic");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			break;
		  case "PP19":
			weapon = player.GetHumanInventory().CreateInHands("PP19");
			weapon.GetInventory().CreateAttachment("Mag_PP19_64Rnd");
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_PP19_64Rnd");
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory("Mag_PP19_64Rnd");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolOptic");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PistolSuppressor");
			}
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment("PP19_Bttstck");
			}
			break;
		  case "M14":
			weapon = player.GetHumanInventory().CreateInHands("M14");
			TStringArray m14Mag = {"Mag_M14_20Rnd", "Mag_M14_10Rnd"}
			weapon.GetInventory().CreateAttachment(m14Mag.GetRandomElement());
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(natoOptic.GetRandomElement());
			}
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(m14Mag.GetRandomElement());
			}
			break;
		  case "m79":
			weapon = player.GetHumanInventory().CreateInHands("m79");
			TStringArray m79Ammo = {
				"Ammo_40mm_Explosive",
				"Ammo_40mm_POX"
			}
			weapon.GetInventory().CreateAttachment(m79Ammo.GetRandomElement());
			player.GetInventory().CreateInInventory(m79Ammo.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(m79Ammo.GetRandomElement());
				player.GetInventory().CreateInInventory(m79Ammo.GetRandomElement());
			}
			break;
		  case "Crossbow_Black":
			weapon = player.GetHumanInventory().CreateInHands("Crossbow_Black");
			weapon.GetInventory().CreateAttachment("Ammo_HuntingBolt");
			player.GetInventory().CreateInInventory("Ammo_HuntingBolt");
			player.GetInventory().CreateInInventory("Ammo_HuntingBolt");
			player.GetInventory().CreateInInventory("Ammo_HuntingBolt");
			player.GetInventory().CreateInInventory("Ammo_HuntingBolt");
			break;
		  default:
			weapon = player.GetHumanInventory().CreateInHands("AK74");
			weapon.GetInventory().CreateAttachment("Mag_AK74_30Rnd");
			player.GetInventory().CreateInInventory("Mag_AK74_30Rnd");
			weapon.GetInventory().CreateAttachment("AK74_WoodBttstck");
			weapon.GetInventory().CreateAttachment("AK74_Hndgrd");
		}

		player.SetQuickBarEntityShortcut(weapon, 0);

		player.GetInventory().CreateInInventory("Battery9V");
		
		player.GetInventory().CreateInInventory(mellee.GetRandomElement());
	}
};

Mission CreateCustomMission(string path)
{
	return new CustomMission();
}
