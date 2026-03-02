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