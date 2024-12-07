import valve.source.a2s

SERVER_ADDRESS = ('50.108.116.1', 2324)

with valve.source.a2s.ServerQuerier(SERVER_ADDRESS) as server:
    info = server.info()
    players = server.players()

print("Server Info:")
print(info)

print("\nPlayers:")
for player in players["players"]:
    print(f"{player['name']} - {player['score']} points, {player['duration']} seconds")

