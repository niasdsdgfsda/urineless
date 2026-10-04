## ⚠️ Wichtiger Hinweis: KI-generierter Code & IRC-Nutzung

Dieses Repository enthält einen **KI-generierten Chat-Client für das IRC-Protokoll**. Da die Software (ganz oder teilweise) durch künstliche Intelligenz erstellt wurde und mit klassischen IRC-Netzwerken interagiert, beachte bitte die folgenden Punkte:

* **Experimenteller Code (KI-generiert):** Da der Quellcode durch eine KI generiert wurde, kann er unentdeckte Bugs, Sicherheitslücken, Speicherlecks oder unerwartetes Verhalten aufweisen. Nutze den Client auf eigene Gefahr und prüfe den Code vor dem produktiven Einsatz.
* **IRC-Netzwerk-Richtlinien (AUP/ToS):** Wenn du diesen Client (oder einen darin eingebauten KI-Bot) in öffentlichen IRC-Netzwerken (wie Libera.Chat, EFnet etc.) einsetzt, bist du allein dafür verantwortlich, dass du gegen keine Netzwerk-Regeln verstößt. Das automatische Spammen, Floodings oder das ungefragte Generieren von Inhalten kann zum Ausschluss (Ban) führen.
* **Datenschutz & Protokollierung:** IRC ist ein offenes, unverschlüsseltes Protokoll (sofern kein TLS/SSL verwendet wird). Alle Nachrichten und eventuell angebundene API-Daten können im Netzwerk oder von Dritten eingesehen werden. Gib keine sensiblen Passwörter, API-Schlüssel oder privaten Daten über diesen Client preis.
* **Keine Gewährleistung:** Die Software wird im aktuellen Zustand („As-Is“) ohne jegliche ausdrückliche oder implizite Garantie bereitgestellt. Die Entwickler oder Ersteller übernehmen keine Haftung für Schäden, Datenverluste oder Netzwerksperren.


# urineless

IRC chat client with optional end-to-end encryption. Use at your own risk.

## Stickers

Open the sticker picker with the **Sticker** button or `/sticker`. Local image
stickers can be placed in `<user-config>/urineless/stickers` (PNG, JPEG, or GIF).
The picker also includes Nekos and Tenor search. Nekos loads up to 20 stickers
per category, and **Mehr Sticker** appends results from further categories up
to 160. Tenor search requests up to 50 results.

Tenor search first scrapes direct GIF links from Tenor's public search page, so
it works without an API key when Tenor exposes media URLs in its HTML. This is
best-effort and can break if Tenor changes its page format. If `TENOR_API_KEY`
is set, the client uses Tenor's official API instead. The picker sends direct
HTTPS GIF URLs as normal chat messages, so other IRC clients can open them and
urineless clients can show them inline. Direct Tenor media links are also
recognized when pasted into chat
use this command to try it (on own risk ofc)

build:

docker build -t my-wayland-app .

run:

docker run -it --rm \
    --env="WAYLAND_DISPLAY=$WAYLAND_DISPLAY" \
    --env="XDG_RUNTIME_DIR=/tmp/runtime-dir" \
    --volume="$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY:/tmp/runtime-dir/$WAYLAND_DISPLAY" \
    --device=/dev/dri \
    my-wayland-app

