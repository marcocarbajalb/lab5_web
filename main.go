package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"os"

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

	parts := strings.Fields(requestLine)
	if len(parts) < 2 {
		return
	}
	method := parts[0]
	path := parts[1]

	// Leer headers
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(lengthStr)
		}
	}

	// Router
	var response string
	switch {
	case method == "GET" && path == "/":
		response = handleIndex(db)
	case method == "GET" && strings.HasPrefix(path, "/static/"):
    	response = handleStatic(path)
	case method == "GET" && path == "/create":
		response = handleCreateForm()
	case method == "POST" && path == "/create":
		response = handleCreate(reader, contentLength, db)
	case method == "POST" && strings.HasPrefix(path, "/update"):
		response = handleUpdate(path, db)
	case method == "POST" && strings.HasPrefix(path, "/decrease"):
    	response = handleDecrease(path, db)
	case method == "DELETE" && strings.HasPrefix(path, "/delete"):
    	response = handleDelete(path, db)
	default:
		response = buildResponse("404 Not Found", "<h1>404 - No encontrado</h1>")
	}

	conn.Write([]byte(response))
}

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
	<meta charset="UTF-8">
	<title>Registro de series</title>
	<link rel="stylesheet" href="/static/styles.css">
	</head>
	<body>
	<h1>Registro de series</h1>
	<table>
	<tr><th>#</th><th>Nombre de la serie</th><th>Episodio actual</th><th>Total de episodios</th><th>Progreso</th><th>Acción</th></tr>`

	var id, actual, total int
	var serie string
	for rows.Next() {
		err := rows.Scan(&id, &serie, &actual, &total)
		if err != nil {
			log.Println("Error en scan:", err)
			return buildResponse("500 Internal Server Error", "<h1>Error leyendo datos</h1>")
		}
		completado := ""
		if actual == total {
			completado = " <strong>¡Completa!</strong>"
		}

		html += fmt.Sprintf(
			`<tr>
				<td>%d</td>
				<td>%s%s</td>
				<td>%d</td>
				<td>%d</td>
				<td>
					<progress value="%d" max="%d"></progress>
				</td>
				<td>
					<button class="btn-decrease" onclick="decreaseEpisode(%d)">-1</button>
					<button class="btn-increase" onclick="nextEpisode(%d)">+1</button>
					<button class="btn-delete" onclick="deleteSerie(%d)">🗑</button>
				</td>
			</tr>`,
			id, serie, completado, actual, total, actual, total, id, id, id,
		)
	}

	html += `</table>
	<br></br>
	<a href="/create">Agregar una nueva serie</a>
	<script src="/static/script.js"></script>
	</body></html>`

	return buildResponse("200 OK", html)
}

func handleCreateForm() string {
	body := `<html>
	<head>
		<meta charset="UTF-8">
		<title>Agregar una nueva serie</title>
		<link rel="stylesheet" href="/static/styles.css">
	</head>
	<body>
	<h1>Agregar una nueva serie</h1>
	<form method="POST" action="/create">
		<label>Nombre de la serie:<br>
			<input type="text" name="series_name" required>
		</label><br><br>
		<label>Episodio actual:<br>
			<input type="number" name="current_episode" min="1" value="1" required>
		</label><br><br>
		<label>Total de episodios:<br>
			<input type="number" name="total_episodes" min="1" value="1" required>
		</label><br><br>
		<button type="submit" class="btn-submit">Agregar</button>
	</form>
	<br>
	<a href="/">Volver</a>
	</body></html>`

	return buildResponse("200 OK", body)
}

func handleCreate(reader *bufio.Reader, contentLength int, db *sql.DB) string {
	
	// Leer exactamente contentLength bytes del cuerpo
	body := make([]byte, contentLength)
	_, err := reader.Read(body)
	if err != nil {
		log.Println("Error leyendo body:", err)
		return buildResponse("400 Bad Request", "<h1>Error leyendo el cuerpo</h1>")
	}

	// Parsear los campos del formulario
	values, err := url.ParseQuery(string(body))
	if err != nil {
		log.Println("Error parseando body:", err)
		return buildResponse("400 Bad Request", "<h1>Error parseando el formulario</h1>")
	}

	name := values.Get("series_name")
	currentEp := values.Get("current_episode")
	totalEps := values.Get("total_episodes")

	log.Printf("Nueva serie: nombre=%s, ep_actual=%s, ep_total=%s", name, currentEp, totalEps)

	// Insertar en la base de datos
	_, err = db.Exec(
		"INSERT INTO series (name, current_episode, total_episodes) VALUES (?, ?, ?)",
		name, currentEp, totalEps,
	)
	if err != nil {
		log.Println("Error en insert:", err)
		return buildResponse("500 Internal Server Error", "<h1>Error guardando en la base de datos</h1>")
	}

	// Redirigir con 303 POST/Redirect/GET
	return "HTTP/1.1 303 See Other\r\nLocation: /\r\n\r\n"
}

func handleUpdate(path string, db *sql.DB) string {
	
	// Extraer el query string de la ruta "/update?id=3"
	parts := strings.SplitN(path, "?", 2)
	if len(parts) < 2 {
		return buildResponse("400 Bad Request", "<h1>Falta el id</h1>")
	}

	params, err := url.ParseQuery(parts[1])
	if err != nil {
		log.Println("Error parseando params:", err)
		return buildResponse("400 Bad Request", "<h1>Error parseando parámetros</h1>")
	}

	id := params.Get("id")
	log.Printf("Actualizando episodio de serie id=%s", id)

	_, err = db.Exec(
		"UPDATE series SET current_episode = current_episode + 1 WHERE id = ? AND current_episode < total_episodes",
		id,
	)
	if err != nil {
		log.Println("Error en update:", err)
		return buildResponse("500 Internal Server Error", "<h1>Error actualizando</h1>")
	}

	return "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\nok"
}

func handleDecrease(path string, db *sql.DB) string {
    parts := strings.SplitN(path, "?", 2)
    if len(parts) < 2 {
        return buildResponse("400 Bad Request", "<h1>Falta el id</h1>")
    }

    params, err := url.ParseQuery(parts[1])
    if err != nil {
        log.Println("Error parseando params:", err)
        return buildResponse("400 Bad Request", "<h1>Error parseando parámetros</h1>")
    }

    id := params.Get("id")
    log.Printf("Decrementando episodio de serie id=%s", id)

    _, err = db.Exec(
        "UPDATE series SET current_episode = current_episode - 1 WHERE id = ? AND current_episode > 1",
        id,
    )
    if err != nil {
        log.Println("Error en decrease:", err)
        return buildResponse("500 Internal Server Error", "<h1>Error actualizando</h1>")
    }

    return "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\nok"
}

func handleStatic(path string) string {
    filePath := "." + path

    content, err := os.ReadFile(filePath)
    if err != nil {
        return buildResponse("404 Not Found", "<h1>Archivo no encontrado</h1>")
    }

    contentType := "text/plain"
    if strings.HasSuffix(path, ".css") {
        contentType = "text/css"
    } else if strings.HasSuffix(path, ".js") {
        contentType = "application/javascript"
    }

    return fmt.Sprintf(
        "HTTP/1.1 200 OK\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n%s",
        contentType, len(content), string(content),
    )
}

func handleDelete(path string, db *sql.DB) string {
    parts := strings.SplitN(path, "?", 2)
    if len(parts) < 2 {
        return buildResponse("400 Bad Request", "<h1>Falta el id</h1>")
    }

    params, err := url.ParseQuery(parts[1])
    if err != nil {
        log.Println("Error parseando params:", err)
        return buildResponse("400 Bad Request", "<h1>Error parseando parámetros</h1>")
    }

    id := params.Get("id")
    log.Printf("Eliminando serie id=%s", id)

    _, err = db.Exec("DELETE FROM series WHERE id = ?", id)
    if err != nil {
        log.Println("Error en delete:", err)
        return buildResponse("500 Internal Server Error", "<h1>Error eliminando</h1>")
    }

    return "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\nok"
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