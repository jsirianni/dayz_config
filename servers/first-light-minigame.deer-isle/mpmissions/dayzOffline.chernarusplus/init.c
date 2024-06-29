void main()
{
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

        override void StartingEquipSetup(PlayerBase player, bool clothesChosen)
        {
            player.GetStatEnergy().Set(5000);
            player.GetStatWater().Set(5000);

            player.RemoveAllItems();

            TStringArray pants = { "BDUPants", "HunterPants_Brown", "HunterPants_Autumn", "HunterPants_Spring" };
            player.GetInventory().CreateInInventory(pants.GetRandomElement());

            TStringArray jacket = { "BDUJacket", "GorkaEJacket_Autumn", "HikingJacket_Black" };
            player.GetInventory().CreateInInventory(jacket.GetRandomElement());

            TStringArray boot = { "JungleBoots_Beige", "JungleBoots_Green" };
            player.GetInventory().CreateInInventory(boot.GetRandomElement());

            TStringArray glove = { "LeatherGloves_Beige", "LeatherGloves_Brown" };
            player.GetInventory().CreateInInventory(glove.GetRandomElement());

            player.GetInventory().CreateInInventory("BalaclavaMask_Black");
            player.GetInventory().CreateInInventory("BeanieHat_Black");
        }
};

Mission CreateCustomMission(string path)
{
        return new CustomMission();
}