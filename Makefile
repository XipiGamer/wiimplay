VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo 0.3-gnome)
BUILD:=$(shell date +%FT%T)
FLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Build=$(BUILD)"
Q?=@
PREFIX?=$(HOME)/.local

wiimplay: $(shell find . -name '*.go') go.mod
	$(Q)go build $(FLAGS) -o $@ .

# Instala el binario y el .desktop (necesario en GNOME para notificaciones
# y para que la ventana se asocie al icono de la aplicación).
install: wiimplay
	$(Q)install -Dm755 wiimplay $(PREFIX)/bin/wiimplay
	$(Q)install -Dm644 data/com.shumatech.wiimplay.desktop \
	    $(PREFIX)/share/applications/com.shumatech.wiimplay.desktop
	$(Q)-update-desktop-database $(PREFIX)/share/applications 2>/dev/null

uninstall:
	$(Q)rm -f $(PREFIX)/bin/wiimplay \
	    $(PREFIX)/share/applications/com.shumatech.wiimplay.desktop

clean:
	$(Q)rm -f wiimplay

.PHONY: install uninstall clean
