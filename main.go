package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var (
	port         = flag.String("port", "0", "Порт сервера (0 для автовыбора)")
	uiDir        = flag.String("root", "./ui", "Путь к папке с UI (HTML/JS)")
	tokenLen     = 16
	sessionToken string
)

// Генерация случайного токена безопасности
func generateToken() string {
	b := make([]byte, tokenLen)
	if _, err := rand.Read(b); err != nil {
		log.Fatal("Ошибка генерации токена:", err)
	}
	return hex.EncodeToString(b)
}

// Middleware для проверки авторизации (URL-параметр или Cookie)
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверка токена в URL (?auth=...)
		urlToken := r.URL.Query().Get("auth")
		if urlToken != "" && urlToken == sessionToken {
			http.SetCookie(w, &http.Cookie{
				Name:     "se-go_session",
				Value:    sessionToken,
				HttpOnly: true,
				Path:     "/",
				// Используем простое присваивание для совместимости версий
				SameSite: http.SameSiteLaxMode, 
			})
			// Редирект на чистый URL без токена
			r.URL.RawQuery = ""
			http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
			return
		}

		// 2. Проверка токена в Cookie
		cookie, err := r.Cookie("se-go_session")
		if err != nil || cookie.Value != sessionToken {
			http.Error(w, "Unauthorized: Se-Go token missing or invalid", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Обработчик выполнения команд (API)
func runHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Cmd  string   `json:"cmd"`
		Args []string `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("EXEC: %s %v", req.Cmd, req.Args)
	out, err := exec.Command(req.Cmd, req.Args...).CombinedOutput()
	
	w.Header().Set("Content-Type", "application/json")
	
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	resp := map[string]string{
		"output": string(out),
		"error":  errStr,
	}
	
	json.NewEncoder(w).Encode(resp)
}

// Монитор родительского процесса (для предотвращения зомби-процессов в Linux)
func monitorParent() {
	for {
		// Если PPID становится 1, значит родитель (bash/launcher) завершился
		if os.Getppid() == 1 {
			log.Println("Parent process exited. Se-Go shutting down...")
			os.Exit(0)
		}
		time.Sleep(2 * time.Second)
	}
}

func main() {
	flag.Parse()
	sessionToken = generateToken()

	// Настройка роутинга
	mux := http.NewServeMux()
	
	// API под защитой
	mux.Handle("/api/run", authMiddleware(http.HandlerFunc(runHandler)))
	
	// Раздача статики (UI) под защитой
	// Важно: папка ui должна существовать рядом с бинарником
	fileServer := http.FileServer(http.Dir(*uiDir))
	mux.Handle("/", authMiddleware(fileServer))

	// Запуск на localhost для безопасности
	listener, err := net.Listen("tcp", "127.0.0.1:"+*port)
	if err != nil {
		log.Fatalf("Ошибка запуска: %v", err)
	}
	
	actualPort := listener.Addr().(*net.TCPAddr).Port
	
	fmt.Printf("\n--- Se-Go STARTED ---\n")
	fmt.Printf("PORT: %d\n", actualPort)
	fmt.Printf("URL:  http://127.0.0.1:%d/?auth=%s\n", actualPort, sessionToken)
	fmt.Printf("--------------------\n")

	// Фоновый мониторинг родителя
	if runtime.GOOS != "windows" {
		go monitorParent()
	}

	server := &http.Server{
		Handler: mux,
	}

	if err := server.Serve(listener); err != nil {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}