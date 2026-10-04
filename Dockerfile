# Wir nutzen das offizielle Arch Linux als Basis
FROM archlinux:latest

# System aktualisieren und notwendige Laufzeit-Bibliotheken installieren
# (Mesa für Grafik/OpenGL, Wayland-Bibliotheken, xkbcommon für Tastatur-Input)
RUN pacman -Syu --noconfirm \
    base-devel \
    wayland \
    libxkbcommon \
    libxkbcommon-x11 \
    libxcursor \
    mesa \
    ttf-dejavu \
    && rm -rf /var/cache/pacman/pkg/*

# Arbeitsverzeichnis im Container festlegen
WORKDIR /app

# Kopiere deine App (oder den Quellcode/die Binärdatei) in den Container
# Ersetze "deine_app" durch den tatsächlichen Namen deiner Datei/Ordner
COPY . /app/

# WICHTIG: Setze Standard-Umgebungsvariablen für Wayland
# (Je nachdem, ob deine App auf GTK oder Qt basiert, greift eine davon)
ENV GDK_BACKEND=wayland
ENV QT_QPA_PLATFORM=wayland
ENV SDL_VIDEODRIVER=wayland

# Der Befehl, der beim Starten des Containers ausgeführt wird
# Ersetze das durch den Startbefehl deiner App:
CMD ["./ircgram"]
