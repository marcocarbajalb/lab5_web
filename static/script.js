async function nextEpisode(id) {
    const url = "/update?id=" + id
    await fetch(url, { method: "POST" })
    location.reload()
}

async function decreaseEpisode(id) {
    const url = "/decrease?id=" + id
    await fetch(url, { method: "POST" })
    location.reload()
}

async function deleteSerie(id) {
    const confirmacion = confirm("¿Estás seguro de que quieres eliminar esta serie?")
    if (!confirmacion) return
    await fetch("/delete?id=" + id, { method: "DELETE" })
    location.reload()
}