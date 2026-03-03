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
	case method == "GET" && strings.HasPrefix(path, "/edit"):
		response = handleEditForm(path, db)
	case method == "PUT" && path == "/edit":
		response = handleEdit(reader, contentLength, db)
	case method == "POST" && strings.HasPrefix(path, "/rating"):
    	response = handleRating(path, db)
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
	rows, err := db.Query(`SELECT s.id, s.name, s.current_episode, s.total_episodes, COALESCE(r.rating, 0)
	FROM series s
	LEFT JOIN ratings r ON s.id = r.series_id`)
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
	<tr><th>#</th><th class="th-nombre">Nombre de la serie</th><th>Episodio actual</th><th>Total de episodios</th><th>Progreso</th><th>Rating</th><th class="th-accion">Acción</th></tr>`

	var id, actual, total, rating int
	var serie string
	for rows.Next() {
		err := rows.Scan(&id, &serie, &actual, &total, &rating)
		if err != nil {
			log.Println("Error en scan:", err)
			return buildResponse("500 Internal Server Error", "<h1>Error leyendo datos</h1>")
		}
		completado := ""
		if actual == total {
			completado = " <strong>¡Completa!</strong>"
		}

		estrellas := ""
		for i := 1; i <= 5; i++ {
			if i <= rating {
				estrellas += fmt.Sprintf(`<span class="estrella activa" onclick="setRating(%d, %d)">★</span>`, id, i)
			} else {
				estrellas += fmt.Sprintf(`<span class="estrella" onclick="setRating(%d, %d)">★</span>`, id, i)
			}
		}

		html += fmt.Sprintf(
			`<tr>
				<td>%d</td>
				<td>%s%s</td>
				<td>%d</td>
				<td>%d</td>
				<td><progress value="%d" max="%d"></progress></td>
				<td>%s</td>
				<td>
					<button class="btn-decrease" onclick="decreaseEpisode(%d)">-1</button>
					<button class="btn-increase" onclick="nextEpisode(%d)">+1</button>
					<button class="btn-edit" onclick="window.location.href='/edit?id=%d'">✏️</button>
					<button class="btn-delete" onclick="deleteSerie(%d)">🗑</button>
				</td>
			</tr>`,
			id, serie, completado, actual, total, actual, total, estrellas, id, id, id, id,
		)
	}

	html += `</table>
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
	<a href="/"> ← Volver </a>
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

	currentEpInt, _ := strconv.Atoi(currentEp)
	totalEpsInt, _ := strconv.Atoi(totalEps)

	if currentEpInt > totalEpsInt {
		return buildResponse("400 Bad Request", `<html>
		<head><meta charset="UTF-8"><link rel="stylesheet" href="/static/styles.css"></head>
		<body>
		<h1>Error</h1>
		<p>El episodio actual no puede ser mayor al total de episodios.</p>
		<a href="/create"> ← Volver </a>
		</body></html>`)
	}

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

func handleEditForm(path string, db *sql.DB) string {
    parts := strings.SplitN(path, "?", 2)
    if len(parts) < 2 {
        return buildResponse("400 Bad Request", "<h1>Falta el id</h1>")
    }

    params, _ := url.ParseQuery(parts[1])
    id := params.Get("id")

    var name string
    var currentEp, totalEps int
    err := db.QueryRow("SELECT name, current_episode, total_episodes FROM series WHERE id = ?", id).
        Scan(&name, &currentEp, &totalEps)
    if err != nil {
        log.Println("Error buscando serie:", err)
        return buildResponse("404 Not Found", "<h1>Serie no encontrada</h1>")
    }

    body := fmt.Sprintf(`<html>
    <head>
        <meta charset="UTF-8">
        <title>Editar serie</title>
        <link rel="stylesheet" href="/static/styles.css">
    </head>
    <body>
    <h1>Editar serie</h1>
    <form id="form-editar">
        <input type="hidden" name="id" value="%s">
        <label>Nombre de la serie:
            <input type="text" name="series_name" value="%s" required>
        </label>
        <label>Episodio actual:
            <input type="number" name="current_episode" min="1" value="%d" required>
        </label>
        <label>Total de episodios:
            <input type="number" name="total_episodes" min="1" value="%d" required>
        </label>
        <button type="submit" class="btn-submit">Guardar cambios</button>
    </form>
    <a href="/"> ← Volver </a>
    <script src="/static/script.js"></script>
    </body></html>`, id, name, currentEp, totalEps)

    return buildResponse("200 OK", body)
}

func handleEdit(reader *bufio.Reader, contentLength int, db *sql.DB) string {
    body := make([]byte, contentLength)
    _, err := reader.Read(body)
    if err != nil {
        log.Println("Error leyendo body:", err)
        return buildResponse("400 Bad Request", "<h1>Error leyendo el cuerpo</h1>")
    }

    values, err := url.ParseQuery(string(body))
    if err != nil {
        log.Println("Error parseando body:", err)
        return buildResponse("400 Bad Request", "<h1>Error parseando el formulario</h1>")
    }

    id := values.Get("id")
    name := values.Get("series_name")
    currentEp := values.Get("current_episode")
    totalEps := values.Get("total_episodes")

	currentEpInt, _ := strconv.Atoi(currentEp)
	totalEpsInt, _ := strconv.Atoi(totalEps)

	if currentEpInt > totalEpsInt {
		return buildResponse("400 Bad Request", `<html>
		<head><meta charset="UTF-8"><link rel="stylesheet" href="/static/styles.css"></head>
		<body>
		<h1>Error</h1>
		<p>El episodio actual no puede ser mayor al total de episodios.</p>
		<a href="/"> ← Volver </a>
		</body></html>`)
	}

    log.Printf("Editando serie id=%s: nombre=%s, ep_actual=%s, ep_total=%s", id, name, currentEp, totalEps)

    _, err = db.Exec(
        "UPDATE series SET name = ?, current_episode = ?, total_episodes = ? WHERE id = ?",
        name, currentEp, totalEps, id,
    )
    if err != nil {
        log.Println("Error en update:", err)
        return buildResponse("500 Internal Server Error", "<h1>Error actualizando</h1>")
    }

    return "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\nok"
}

func handleRating(path string, db *sql.DB) string {
    parts := strings.SplitN(path, "?", 2)
    if len(parts) < 2 {
        return buildResponse("400 Bad Request", "<h1>Faltan parámetros</h1>")
    }

    params, _ := url.ParseQuery(parts[1])
    seriesId := params.Get("series_id")
    rating := params.Get("rating")

    log.Printf("Rating serie id=%s: %s estrellas", seriesId, rating)

    _, err := db.Exec(
        "INSERT OR REPLACE INTO ratings (series_id, rating) VALUES (?, ?)",
        seriesId, rating,
    )
    if err != nil {
        log.Println("Error guardando rating:", err)
        return buildResponse("500 Internal Server Error", "<h1>Error guardando rating</h1>")
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