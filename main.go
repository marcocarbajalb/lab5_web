package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"database/sql"

	_ "modernc.org/sqlite"
)

func handleClient(conn net.Conn, db *sql.DB) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	_, err := reader.ReadString('\n')
	if err != nil {
		log.Println("Error leyendo request:", err)
		return
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
	}
	
	rows, err := db.Query("SELECT id, name, current_episode, total_episodes FROM series")
	if err != nil {
		log.Println("Error en query:", err)
		return
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
	var id int
	var serie string
	var actual int
	var total int
	for rows.Next() {
		err := rows.Scan(&id, &serie, &actual, &total)
		if err != nil {
			log.Println("Error en scan:", err)
			return
		}
		html += fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%d</td><td>%d</td></tr>", id, serie, actual, total)
	}
	html += "</table></body></html>"

	response := fmt.Sprintf(
		"HTTP/1.1 200 OK\r\n"+
			"Content-Type: text/html\r\n"+
			"Content-Length: %d\r\n"+
			"\r\n"+
			"%s",
		len(html),
		html,
	)

	conn.Write([]byte(response))
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