void main()
{
    Hive ce = CreateHive();
    if (ce)
        ce.InitOffline();
}

class CustomMission: MissionServer
{
    override PlayerBase CreateCharacter(PlayerIdentity identity, vector pos)
    {
        PlayerBase player;
        player = GetGame().CreatePlayer(identity, "Survivor", pos, 0, "NONE");
        return player;
    }
};

Mission CreateCustomMission(string path)
{
    return new CustomMission();
}
