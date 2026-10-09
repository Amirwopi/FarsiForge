with open(r'D:\FarsiForge\frontend\src\app\page.tsx', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace("gameInfo.Engine", "gameInfo.engine")
content = content.replace("gameInfo.Backend", "gameInfo.backend")
content = content.replace("gameInfo.Confidence", "gameInfo.confidence")
content = content.replace("gameInfo.GameRoot", "gameInfo.game_root")
content = content.replace("item.Source", "item.source")
content = content.replace("item.Translation", "item.translation")

with open(r'D:\FarsiForge\frontend\src\app\page.tsx', 'w', encoding='utf-8') as f:
    f.write(content)

print("Casing fixed!")
