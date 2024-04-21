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
		TStringArray m4Mag = {"Mag_STANAG_30Rnd","Mag_STANAGCoupled_30Rnd","Mag_CMAG_10Rnd","Mag_CMAG_20Rnd","Mag_CMAG_30Rnd","Mag_CMAG_40Rnd"};
		TStringArray mellee = {"BrassKnuckles_Shiny", "BaseballBat", "NailedBaseballBat"};
		TStringArray shotgunAmmo = {"Ammo_12gaPellets","Ammo_12gaRubberSlug","Ammo_12gaSlug"};
		TStringArray saigaMag = {"Mag_Saiga_5Rnd","Mag_Saiga_8Rnd","Mag_Saiga_Drum20Rnd"};
		TStringArray akmMag = {"Mag_AKM_30Rnd","Mag_AKM_Palm30Rnd","Mag_AKM_Drum75Rnd"};
		TStringArray m14Mag = {"Mag_M14_20Rnd", "Mag_M14_10Rnd"};
		TStringArray m79Ammo = {"Ammo_40mm_Explosive","Ammo_40mm_POX"};
		TStringArray shotguns = {"Mp133Shotgun","Izh43Shotgun",};

		TStringArray natoOptic = {"ACOGOptic","M4_T3NRDSOptic"};
		TStringArray sovietOptic = {"PSO11Optic","KobraOptic"};
		TStringArray explosives = {"TripwireTrap","ClaymoreMine"};

		TStringArray helmet = {"BallisticHelmet_Black","BallisticHelmet_UN","ConstructionHelmet_Blue","DarkMotoHelmet_Blue","FirefightersHelmet_Yellow","GorkaHelmet"};
		TStringArray medical = {"Bandage","Rag"};
		TStringArray nades = {"RDG2SmokeGrenade_Black","RGD5Grenade","FlashGrenade"};

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

		// Custom gun kits that attempt to use maps as an elegant approach 
		// to customization for the kits
		auto gunMap = new map<string, map<string, TStringArray>>();

		auto makarovKit = new map<string, TStringArray>();
		makarovKit.Insert("common", {"MakarovIJ70","Mag_IJ70_8Rnd"});
		makarovKit.Insert("attachments", {"PistolSuppressor"});
		makarovKit.Insert("extra", {"DryBag_Black","TripwireTrap","ClaymoreMine"});
		gunMap.Insert("MakarovIJ70", makarovKit);

		auto fnx45Kit = new map<string, TStringArray>();
		fnx45Kit.Insert("common", {"FNX45","Mag_FNX45_15Rnd"});
		fnx45Kit.Insert("attachments", {"PistolSuppressor","PistolOptic"});
		gunMap.Insert("FNX45", fnx45Kit);
		
		auto glock19Kit = new map<string, TStringArray>();
		glock19Kit.Insert("common", {"FNX45","Mag_Glock_15Rnd"});
		glock19Kit.Insert("attachments", {"PistolSuppressor","PistolOptic"});
		gunMap.Insert("Glock19", glock19Kit);

		auto mkiiKit = new map<string, TStringArray>();
		glock19Kit.Insert("common", {"MKII","Mag_MKII_10Rnd"});
		glock19Kit.Insert("attachments", {"PistolSuppressor","PistolOptic"});
		gunMap.Insert("MKII", mkiiKit);

		auto colt1911Kit = new map<string, TStringArray>();
		colt1911Kit.Insert("common", {"Colt1911","Mag_1911_7Rnd"});
		colt1911Kit.Insert("attachments", {"PistolSuppressor"});
		gunMap.Insert("Colt1911", colt1911Kit);
		
		auto engraved1911Kit = new map<string, TStringArray>();
		engraved1911Kit.Insert("common", {"Colt1911","Mag_1911_7Rnd"});
		engraved1911Kit.Insert("attachments", {"PistolSuppressor"});
		gunMap.Insert("Engraved1911", engraved1911Kit);

		auto izh18Kit = new map<string, TStringArray>();
		izh18Kit.Insert("common", {"Izh18","Ammo_762x39"});
		gunMap.Insert("Izh18", izh18Kit);

		auto mosin9130Kit = new map<string, TStringArray>();
		mosin9130Kit.Insert("common", {"Mosin9130","Ammo_762x54"});
		mosin9130Kit.Insert("attachments", {"PUScopeOptic"});
		gunMap.Insert("Mosin9130", mosin9130Kit);

		auto cz527Kit = new map<string, TStringArray>();
		cz527Kit.Insert("common", {"CZ527","Mag_CZ527_5rnd"});
		gunMap.Insert("CZ527", cz527Kit);

		auto winchester70Kit = new map<string, TStringArray>();
		winchester70Kit.Insert("common", {"Winchester70","Ammo_308Win"});
		winchester70Kit.Insert("attachments", {"HuntingOptic"});
		gunMap.Insert("Winchester70", winchester70Kit);
		
		auto sksKit = new map<string, TStringArray>();
		sksKit.Insert("common", {"SKS","Ammo_762x39"});
		sksKit.Insert("attachments", {"PUScopeOptic"});
		gunMap.Insert("SKS", sksKit);

		auto mp133Kit = new map<string, TStringArray>();
		mp133Kit.Insert("common", {"Mp133Shotgun"});
		gunMap.Insert("Mp133Shotgun", mp133Kit);

		auto izh43Kit = new map<string, TStringArray>();
		izh43Kit.Insert("common", {"Izh43Shotgun"});
		gunMap.Insert("Izh43Shotgun", mp133Kit);

		auto saigaKit = new map<string, TStringArray>();
		saigaKit.Insert("common", {"Saiga"});
		sksKit.Insert("attachments", {"PSO11Optic","Saiga_Bttstck"});
		saigaKit.Insert("saigaMag", saigaMag);
		gunMap.Insert("Saiga", saigaKit);

		auto cz61Kit = new map<string, TStringArray>();
		cz61Kit.Insert("common", {"CZ61","Mag_CZ61_20Rnd"});
		cz61Kit.Insert("attachments", {"PistolSuppressor"});
		gunMap.Insert("CZ61", cz61Kit);

		auto ump45Kit = new map<string, TStringArray>();
		ump45Kit.Insert("common", {"UMP45","Mag_UMP_25Rnd"});
		ump45Kit.Insert("attachments", {"PistolSuppressor"});
		ump45Kit.Insert("natoOptic", natoOptic);
		gunMap.Insert("UMP45", ump45Kit);

		auto mp5kKit = new map<string, TStringArray>();
		mp5kKit.Insert("common", {"MP5K","Mag_UMP_25Rnd"});
		mp5kKit.Insert("attachments", {"PistolSuppressor","MP5_Compensator","MP5_RailHndgrd","MP5k_StockBttstck"});
		mp5kKit.Insert("natoOptic", natoOptic);
		gunMap.Insert("MP5K", mp5kKit);

		auto aks74uKit = new map<string, TStringArray>();
		aks74uKit.Insert("common", {"AKS74U","Mag_AK74_30Rnd"});
		aks74uKit.Insert("attachments", {"AK_Suppressor","AKS74U_Bttstck"});
		gunMap.Insert("AKS74U", aks74uKit);

		auto falKit = new map<string, TStringArray>();
		falKit.Insert("common", {"FAL","Mag_FAL_20Rnd"});
		falKit.Insert("attachments", {"Fal_FoldingBttstck"});
		falKit.Insert("natoOptic", natoOptic);
		gunMap.Insert("FAL", falKit);

		auto akmKit = new map<string, TStringArray>();
		akmKit.Insert("common", {"AKM"});
		akmKit.Insert("attachments", {"AK_Suppressor","AK_PlasticBttstck","AK_PlasticHndgrd"});
		akmKit.Insert("sovietOptic", sovietOptic);
		gunMap.Insert("AKM", akmKit);

		auto ak101Kit = new map<string, TStringArray>();
		ak101Kit.Insert("common", {"AK101","Mag_AK101_30Rnd"});
		ak101Kit.Insert("attachments", {"AK_Suppressor","PSO11Optic","AK_PlasticBttstck","AK_PlasticHndgrd"});
		gunMap.Insert("AK101", ak101Kit);

		auto ak74Kit = new map<string, TStringArray>();
		ak74Kit.Insert("common", {"AK74","Mag_AK74_30Rnd"});
		ak74Kit.Insert("attachments", {"AK_Suppressor","AK74_WoodBttstck","AK74_Hndgrd"});
		ak74Kit.Insert("sovietOptic", sovietOptic);
		gunMap.Insert("AK74", ak74Kit);

		auto m4a1Kit = new map<string, TStringArray>();
		m4a1Kit.Insert("common", {"M4A1"});
		m4a1Kit.Insert("attachments", {"M4_Suppressor","M4_OEBttstck","M4_PlasticHndgrd"});
		m4a1Kit.Insert("m4Mag", m4Mag);
		m4a1Kit.Insert("natoOptic", natoOptic);
		gunMap.Insert("M4A1", m4a1Kit);

		auto vssKit = new map<string, TStringArray>();
		vssKit.Insert("common", {"VSS","Mag_VSS_10Rnd"});
		vssKit.Insert("attachments", {"PSO11Optic","Mag_Vikhr_30Rnd"});
		gunMap.Insert("VSS", vssKit);

		auto svdKit = new map<string, TStringArray>();
		svdKit.Insert("common", {"SVD","Mag_SVD_10Rnd"});
		svdKit.Insert("attachments", {"PSO11Optic","AK_Suppressor"});
		gunMap.Insert("SVD", svdKit);

		auto asvalKit = new map<string, TStringArray>();
		asvalKit.Insert("common", {"ASVAL","Mag_Vikhr_30Rnd"});
		asvalKit.Insert("attachments", {"PSO11Optic"});
		gunMap.Insert("ASVAL", asvalKit);

		auto vikhrKit = new map<string, TStringArray>();
		vikhrKit.Insert("common", {"Vikhr","Mag_Vikhr_30Rnd"});
		vikhrKit.Insert("attachments", {"PSO11Optic"});
		gunMap.Insert("Vikhr", vikhrKit);

		auto m16a2Kit = new map<string, TStringArray>();
		m16a2Kit.Insert("common", {"M16A2"});
		m16a2Kit.Insert("m4Mag", m4Mag);
		m16a2Kit.Insert("attachments", {"M4_Suppressor"});
		gunMap.Insert("M16A2", m16a2Kit);

		auto famasKit = new map<string, TStringArray>();
		famasKit.Insert("common", {"FAMAS","Mag_FAMAS_25Rnd"});
		gunMap.Insert("FAMAS", famasKit);

		auto augKit = new map<string, TStringArray>();
		augKit.Insert("common", {"Aug","Mag_Aug_30Rnd"});
		augKit.Insert("natoOptic", natoOptic);
		gunMap.Insert("Aug", augKit);

		auto augShortKit = new map<string, TStringArray>();
		augShortKit.Insert("common", {"AugShort","Mag_Aug_30Rnd"});
		gunMap.Insert("AugShort", augShortKit);

		auto deagleKit = new map<string, TStringArray>();
		deagleKit.Insert("common", {"Deagle_Gold","Mag_Deagle_9rnd"});
		deagleKit.Insert("attachments", {"PistolOptic","PistolSuppressor"});
		gunMap.Insert("Deagle_Gold", deagleKit);

		auto pp19Kit = new map<string, TStringArray>();
		pp19Kit.Insert("common", {"PP19","Mag_PP19_64Rnd"});
		pp19Kit.Insert("attachments", {"PistolOptic","PistolSuppressor","PP19_Bttstck"});
		gunMap.Insert("PP19", pp19Kit);

		auto m14Kit = new map<string, TStringArray>();
		m14Kit.Insert("common", {"M14"});
		m14Kit.Insert("natoOptic", natoOptic);
		gunMap.Insert("M14", m14Kit);

		auto m79Kit = new map<string, TStringArray>();
		m79Kit.Insert("common", {"m79"});
		m79Kit.Insert("m79Ammo", m79Ammo);
		gunMap.Insert("m79", m79Kit);

		auto crossbowKit = new map<string, TStringArray>();
		crossbowKit.Insert("common", {"Crossbow_Black","Ammo_HuntingBolt"});
		gunMap.Insert("Crossbow_Black", crossbowKit);

		// this replaced a horrendous switch statement
		// and there are some new if statements to account for
		// cases where kits are doing different things.
		//
		// it's not as wild/repetitive as the switch statement
		// which is part of why I believe it's a better way
		// even if it doesn't trim as many lines as
		// i'd hoped for.
		auto gunKit = gunMap.Get(gun.GetRandomElement());
		auto common = gunKit.Get("common");
		weapon = player.GetHumanInventory().CreateInHands(common[0]);

		if (common.Count() > 1) {
			weapon.GetInventory().CreateAttachment(common[1]);
			if (decide(70) == true) {
				if (gunKit.Find("natoOptic", true)) {
					weapon.GetInventory().CreateAttachment(natoOptic.GetRandomElement());
				}
				player.GetInventory().CreateInInventory(common[1]);
			}
		}

		if (gunKit.Find("attachments", true)) {
			if (decide(70) == true) {
				auto attachments = gunKit.Get("attachments");
				int i = 0;
				while (i < attachments.Count()) {
					weapon.GetInventory().CreateAttachment(attachments[i]);
					i++;
				}
			}
		}

		// IF makarov kit
		if (common[0] == "MakarovIJ70") {
			if (decide(70) == true) {
				auto extra = gunKit.Get("extra");
				int j = 0;
				while (j < extra.Count()) {
					if (extra[j] == "DryBag_Black") {
						player.GetInventory().CreateInInventory(extra[j]);
						j++;
					} else {
						int k = 0;
						int quantity = 3;
						while (k < quantity) {
							player.GetInventory().CreateInInventory(extra[j]);
							k++;
						}
						j++;
					}
				}
			}
		}

		// IF M14 kit
		if (common[0] == "M14") {
			weapon.GetInventory().CreateAttachment(m14Mag.GetRandomElement());
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(natoOptic.GetRandomElement());
				player.GetInventory().CreateInInventory(m14Mag.GetRandomElement());
			}
		}

		// IF M79 kit
		if (common[0] == "m79") {
			weapon.GetInventory().CreateAttachment(m79Ammo.GetRandomElement());
			player.GetInventory().CreateInInventory(m79Ammo.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(m79Ammo.GetRandomElement());
				player.GetInventory().CreateInInventory(m79Ammo.GetRandomElement());
			}
		}

		// IF shotgun
		if (shotguns.Find(common[0], true)) {
			player.GetInventory().CreateInInventory(shotgunAmmo.GetRandomElement());
		}

		// IF AKM kit
		if (common[0] == "AKM") {
			weapon.GetInventory().CreateAttachment(akmMag.GetRandomElement());
			player.GetInventory().CreateInInventory(akmMag.GetRandomElement());
			if (decide(70) == true) {
				weapon.GetInventory().CreateAttachment(sovietOptic.GetRandomElement());
			}
		}

		// IF AK74 Kit
		if (common[0] == "AK74") {
			weapon.GetInventory().CreateAttachment(sovietOptic.GetRandomElement());
			player.GetInventory().CreateInInventory(common[1]);
		}

		// IF saiga kit
		if (common[0] == "Saiga") {
			weapon.GetInventory().CreateAttachment(saigaMag.GetRandomElement());
			player.GetInventory().CreateInInventory(saigaMag.GetRandomElement());
			if (decide(70) == true) {
				player.GetInventory().CreateInInventory(saigaMag.GetRandomElement());
			}
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
