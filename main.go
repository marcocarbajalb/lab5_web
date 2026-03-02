package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net"
	"strings"

	_ "modernc.org/sqlite"
)

func handleClient(conn net.Conn, db *sql.DB) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	requestLine, err := reader.ReadString('\n')
	if err != nil {
		log.Println("Error leyendo request:", err)
		return
	}

	parts := strings.Fields(requestLine) // ["GET", "/", "HTTP/1.1"]
	if len(parts) < 2 {
		return
	}
	method := parts[0]
	path := parts[1]

	// Leer y descartar los headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
	}

	// Router
	var response string
	switch {
	case method == "GET" && path == "/":
		response = handleIndex(db)
	case method == "GET" && path == "/create":
		response = handleCreateForm()
	default:
		response = buildResponse("404 Not Found", "<h1>404 - No encontrado</h1>")
	}

	conn.Write([]byte(response))
}

// Construye una respuesta HTTP genérica
func buildResponse(status string, body string) string {
	return fmt.Sprintf(
		"HTTP/1.1 %s\r\nContent-Type: text/html\r\nContent-Length: %d\r\n\r\n%s",
		status, len(body), body,
	)
}

func handleIndex(db *sql.DB) string {
	rows, err := db.Query("SELECT id, name, current_episode, total_episodes FROM series")
	if err != nil {
		log.Println("Error en query:", err)
		return buildResponse("500 Internal Server Error", "<h1>Error en la base de datos</h1>")
	}
	defer rows.Close()

	html := `<html>
	<head>
	<title>Registro de series</title>
	<style>
		table { border-collapse: collapse; }
		td, th { border: 1px solid black; padding: 8px; }
	</style>
	</head>
	<body>
	<h1>Registro de series</h1>
	<table>
	<tr><th>#</th><th>Nombre de la serie</th><th>Episodio actual</th><th>Total de episodios</th></tr>`

	var id, actual, total int
	var serie string
	for rows.Next() {
		err := rows.Scan(&id, &serie, &actual, &total)
		if err != nil {
			log.Println("Error en scan:", err)
			return buildResponse("500 Internal Server Error", "<h1>Error leyendo datos</h1>")
		}
		html += fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%d</td><td>%d</td></tr>", id, serie, actual, total)
	}

	html += `</table>
	<br></br>
	<a href="/create">Agregar serie</a>
	<script>
		alert("Hola! Bienvenid@ al registro de series");
	</script>
	</body></html>`

	return buildResponse("200 OK", html)
}

// Handler para GET /create
func handleCreateForm() string {
	return buildResponse("200 OK", "<h1>Crear serie</h1><a href='/'>Volver</a>")
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("Error al iniciar servidor:", err)
	}
	defer listener.Close()

	db, err := sql.Open("sqlite", "file:series.db")
	if err != nil {
		log.Fatal("Error abriendo base de datos:", err)
	}
	defer db.Close()

	log.Println("Servidor escuchando en puerto 8080...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error aceptando conexión:", err)
			continue
		}
		go handleClient(conn, db)
	}
}