#!/bin/bash

# Se-Go Universal App Launcher v1.1
# Использование: ./se-go-run.sh [путь_к_папке_ui]

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 0. Проверка аргументов и папки
TARGET_DIR="${1:-ui}" # Если аргумент пуст, используем "ui"

if [ ! -d "$TARGET_DIR" ]; then
    echo -e "${RED}[!] Ошибка: Папка '$TARGET_DIR' не найдена.${NC}"
    echo -e "${YELLOW}[*] Использование: $0 [путь_к_папке_интерфейса]${NC}"
    exit 1
fi

if [ ! -f "./se-go" ]; then
    echo -e "${RED}[!] Ошибка: Бинарник 'se-go' не найден в текущей папке.${NC}"
    exit 1
fi

echo -e "${GREEN}[*] Инициализация Se-Go для проекта: $TARGET_DIR...${NC}"

# 1. Запуск Se-Go
# Флаг -root теперь динамический
./se-go -root "$TARGET_DIR" -port 0 > .se-go.log 2>&1 &
SERVER_PID=$!

# Функция очистки
cleanup() {
    echo -e "\n${GREEN}[*] Завершение сессии (PID: $SERVER_PID)...${NC}"
    kill $SERVER_PID 2>/dev/null
    rm -f .se-go.log
    exit
}
trap cleanup SIGINT SIGTERM

# 2. Ожидание URL и токена
echo -n "[*] Генерация безопасного канала..."
for i in {1..20}; do
    URL=$(grep -o "http://127.0.0.1:[0-9]*/?auth=[a-f0-9]*" .se-go.log)
    if [ ! -z "$URL" ]; then
        echo -e " ${GREEN}OK${NC}"
        break
    fi
    echo -n "."
    sleep 0.2
done

if [ -z "$URL" ]; then
    echo -e "\n${RED}[!] Ошибка: Сервер не выдал токен доступа.${NC}"
    cleanup
fi

echo -e "[*] Точка входа: ${GREEN}$URL${NC}"

# 3. Запуск UI (Surf в приоритете)
if command -v surf >/dev/null 2>&1; then
    echo "[*] Запуск интерфейса через surf..."
    # -N (без инспектора), -z 1.0 (зум 100%), -K (режим киоска при желании)
    surf "$URL"
else
    echo -e "${YELLOW}[!] surf не найден, использую системный браузер...${NC}"
    xdg-open "$URL"
fi

cleanup