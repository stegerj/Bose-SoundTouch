const JSON_HEADERS = { 'Content-Type': 'application/json' };

async function req(url, opts = {}) {
    const r = await fetch(url, opts);

    // Se il server risponde con 204 No Content o un body vuoto, evita il parsing JSON
    if (r.status === 204 || r.headers.get("content-length") === "0") {
        return { success: true };
    }

    const text = await r.text();
    if (!text) return { success: true };

    // Tenta il parsing JSON in sicurezza, se fallisce restituisce il testo come stringa data
    try {
        return JSON.parse(text);
    } catch (e) {
        // Fallback se il server Go restituisce testo semplice (es: "OK" o "success")
        return { success: true, data: text };
    }
}

export const api = {
    devices: () => req('/api/control/devices'),
    device: (id) => req(`/api/control/devices/${id}`),
    removeDevice: (id) => req(`/api/control/devices/${id}`, { method: 'DELETE' }),
    discover: () => req('/api/control/discover', { method: 'POST' }),
    key: (id, key) => req(`/api/control/devices/${id}/key/${key}`, { method: 'POST' }),
    volume: (id, level) => req(`/api/control/devices/${id}/volume/${level}`, { method: 'POST' }),
    bass: (id, level) => req(`/api/control/devices/${id}/action/bass`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify({ level }),
    }),
    power: (id) => req(`/api/control/devices/${id}/power`, { method: 'POST' }),
    recents: (id) => req(`/api/control/devices/${id}/recents`),
    zone: (id) => req(`/api/control/devices/${id}/zone`),
    zoneAdd: (masterId, slaveId) => req(`/api/control/devices/${masterId}/zone/add/${slaveId}`, { method: 'POST' }),
    zoneRemove: (masterId, slaveId) => req(`/api/control/devices/${masterId}/zone/remove/${slaveId}`, { method: 'POST' }),
    zoneDissolve: (id) => req(`/api/control/devices/${id}/zone/dissolve`, { method: 'POST' }),
    zoneLeave: (id) => req(`/api/control/devices/${id}/zone/leave`, { method: 'POST' }),
    play: (id, item) => req(`/api/control/devices/${id}/play`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify(item),
    }),
    tuneInBrowse: (path) => req(path ? `/api/control/providers/tunein/navigate/${path}` : '/api/control/providers/tunein/navigate'),
    tuneInSearch: (q) => req(`/api/control/providers/tunein/search?q=${encodeURIComponent(q)}`),
    tuneInSearchNext: (cursor) => req(`/api/control/providers/tunein/search/next?cursor=${encodeURIComponent(cursor)}`),
    control: (id, action, presetId) => req(`/api/control/devices/${id}/action/${action}?id=${presetId}`),
    storePreset: (id, slotId) => req(`/api/control/devices/${id}/action/storepreset?id=${slotId}`),
    selectSource: (id, source, account) => req(`/api/control/devices/${id}/action/source?name=${encodeURIComponent(source)}&account=${encodeURIComponent(account || '')}`),
    tuneInPlay: (deviceId, item) => req(`/api/control/devices/${deviceId}/providers/tunein/play`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify(item),
    }),
    radioBrowserSearch: (q) => req(`/api/control/providers/radiobrowser/search?q=${encodeURIComponent(q)}`),
    radioBrowserPlay: (deviceId, item) => req(`/api/control/devices/${deviceId}/providers/radiobrowser/play`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify(item),
    }),
    playURL: (deviceId, url, name, imageUrl, serviceUrl) => req(`/api/control/devices/${deviceId}/providers/url/play`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify({ url, name, imageUrl, serviceUrl }),
    }),
    speak: (deviceId, text) => req(`/api/control/devices/${deviceId}/providers/tts/play`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify({ text }),
    }),
    libraryDiscover: (timeout) => req(`/api/control/providers/library/servers${timeout ? `?timeout=${timeout}` : ''}`),
    libraryServers: (id) => req(`/api/control/devices/${id}/library/servers`),
    libraryAddServer: (id, body) => req(`/api/control/devices/${id}/library/servers`, { method: 'POST', headers: JSON_HEADERS, body: JSON.stringify(body) }),
    libraryRemoveServer: (id, account) => req(`/api/control/devices/${id}/library/servers/${encodeURIComponent(account)}`, { method: 'DELETE' }),
    libraryBrowse: (id, { account, location, type, start, count }) => {
        const qs = [
            `account=${encodeURIComponent(account)}`,
            location !== undefined && location !== '' ? `location=${encodeURIComponent(location)}` : null,
            type ? `type=${encodeURIComponent(type)}` : null,
            start !== undefined ? `start=${encodeURIComponent(start)}` : null,
            count !== undefined ? `count=${encodeURIComponent(count)}` : null,
        ].filter(Boolean).join('&');
        return req(`/api/control/devices/${id}/library/browse?${qs}`);
    },
    libraryPlay: (id, body) => req(`/api/control/devices/${id}/library/play`, { method: 'POST', headers: JSON_HEADERS, body: JSON.stringify(body) }),

    // Deezer — browse (global, not device-scoped)
    deezerSearch: (q, type) => req(`/api/control/providers/deezer/search?q=${encodeURIComponent(q)}${type ? `&type=${encodeURIComponent(type)}` : ''}`),
    deezerArtistDetails: (artistId) => req(`/api/control/providers/deezer/artist/${artistId}`),
    deezerArtistRadio: (artistId) => req(`/api/control/providers/deezer/artist/${artistId}/radio`),
    deezerAlbumTracks: (albumId) => req(`/api/control/providers/deezer/album/${albumId}/tracks`),
    deezerArtistAlbums: (artistId) => req(`/api/control/providers/deezer/artist/${artistId}/albums`),
    deezerArtistTracklist: (artistId) => req(`/api/control/providers/deezer/artist/${artistId}/tracklist`),
    deezerArtistTopTracks: (artistId) => req(`/api/control/providers/deezer/artist/${artistId}/top`),

    // Deezer — playback nativo vecchio stile (device-scoped)
    deezerPlay: (deviceId, item) => req(`/api/control/devices/${deviceId}/providers/deezer/play`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify(item),
    }),

    // --- 2° PUNTO: Coda Dinamica Contestuale (Reindirizzato su /providers/deezer/devices/{id}/queue) ---
    deezerPlayFromContext: (deviceId, trackId, tracks, contextType, contextId) => req(`/api/control/providers/deezer/devices/${deviceId}/queue`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify({ tracks }),
    }),

    // --- 3° PUNTO: Coda Locale Visibile / Add to Queue (Reindirizzati sui corretti endpoint del tuo mount.go) ---
    deezerGetQueue: (deviceId) => req(`/api/control/providers/deezer/devices/${deviceId}/queue/status`),

    deezerAddToQueue: (deviceId, tracks) => req(`/api/control/providers/deezer/devices/${deviceId}/queue/add`, {
        method: 'POST',
        headers: JSON_HEADERS,
        body: JSON.stringify({ tracks: Array.isArray(tracks) ? tracks : [tracks] }),
    }),

    deezerRemoveFromQueue: (deviceId, index) => req(`/api/control/providers/deezer/devices/${deviceId}/queue/remove?index=${index}`, {
        method: 'POST',
    }),

    deezerQueueStop: (deviceId) => req(`/api/control/providers/deezer/devices/${deviceId}/queue/stop`, {
        method: 'POST',
    }),
};
