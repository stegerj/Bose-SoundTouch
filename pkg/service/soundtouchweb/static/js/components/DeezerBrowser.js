import { h } from 'preact';
import { useState } from 'preact/hooks';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

export function DeezerBrowser({ devices }) {
    const [sections, setSections] = useState([]);
    const [searchQuery, setSearchQuery] = useState('');
    const [searchType, setSearchType] = useState('album'); // 'album', 'artist', or 'track'
    const [loading, setLoading] = useState(false);
    const [statusMessage, setStatusMessage] = useState('');
    const [pendingPlay, setPendingPlay] = useState(null);

    async function search(q, type) {
        if (!q.trim()) return;
        setLoading(true);
        setStatusMessage('Searching Deezer library...');
        setSections([]);

        try {
            const result = await api.deezerSearch(q, type);
            setLoading(false);

            const itemsList = result?.data || result;

            if (!itemsList || itemsList.length === 0) {
                setStatusMessage('No results found matching your request.');
                return;
            }

            setStatusMessage(`Found ${itemsList.length} results:`);
            mapAndSetItems(itemsList, type, `${type.charAt(0).toUpperCase() + type.slice(1)} Results`);

        } catch (err) {
            console.error("Deezer fetch failed: ", err);
            setLoading(false);
            setStatusMessage('Error pulling details from Deezer API.');
        }
    }

    // Funzione ausiliaria aggiornata per filtrare gli artisti senza album
    function mapAndSetItems(itemsList, defaultType, sectionName) {
        // 1. Filtriamo la lista originale prima di mappare gli elementi
        const filteredList = itemsList.filter(item => {
            const actualType = item.type || defaultType;
            // Se l'elemento è un artista ed ha 0 album, lo scartiamo (restituisce false)
            if (actualType === 'artist' && item.nb_album === 0) {
                return false;
            }
            return true;
        });

        // 2. Mappiamo solo gli elementi rimasti dopo il filtro
        setSections([{
            name: sectionName,
            items: filteredList.map(item => {
                let imageUrl = '';
                let name = '';
                let subtitle = '';
                const actualType = item.type || defaultType;

                if (actualType === 'album') {
                    imageUrl = item.cover_medium || item.cover_small;
                    name = item.title;
                    subtitle = item.artist ? item.artist.name : '';
                } else if (actualType === 'artist') {
                    imageUrl = item.picture_medium || item.picture_small;
                    name = item.name;
                    subtitle = item.nb_album !== undefined ? `${item.nb_album} Albums` : 'Artist';
                } else if (actualType === 'track') {
                    imageUrl = item.album ? (item.album.cover_medium || item.album.cover_small) : '';
                    name = item.title;
                    subtitle = item.artist ? item.artist.name : '';
                }

                return {
                    id: item.id,
                    name: name,
                    subtitle: subtitle,
                    imageUrl: imageUrl,
                    type: actualType
                };
            })
        }]);
    }

    async function handleItemSelection(item) {
        if (item.type === 'artist') {
            setLoading(true);
            setStatusMessage(`Loading discography for ${item.name}...`);
            setSections([]);
            try {
                const response = await api.deezerArtistDetails(item.id);
                setLoading(false);

                const artistData = response?.data || response;

                if (!artistData || (!artistData.albums && !artistData.tracks)) {
                    setStatusMessage('No items found for this artist.');
                    return;
                }

                setStatusMessage(`Discography for ${item.name}:`);

                const newSections = [];

                if (artistData.albums && artistData.albums.length > 0) {
                    newSections.push({
                        name: 'Albums',
                        items: artistData.albums.map(a => ({
                            id: a.id,
                            name: a.title,
                            subtitle: 'Album',
                            imageUrl: a.cover_medium || a.cover_small,
                            type: 'album'
                        }))
                    });
                }

                if (artistData.tracks && artistData.tracks.length > 0) {
                    newSections.push({
                        name: 'Top Tracks',
                        items: artistData.tracks.map(t => ({
                            id: t.id,
                            name: t.title,
                            subtitle: 'Track',
                            imageUrl: t.album ? (t.album.cover_medium || t.album.cover_small) : '',
                            type: 'track'
                        }))
                    });
                }

                setSections(newSections);
            } catch (err) {
                console.error("Failed to fetch artist details:", err);
                setLoading(false);
                setStatusMessage('Error fetching details for this artist.');
            }
        } else {
            setPendingPlay(item);
        }
    }

    async function playOn(deviceId) {
        if (!pendingPlay) return;
        setLoading(true);

        try {
            await api.deezerPlay(deviceId, {
                location: String(pendingPlay.id),
                type: pendingPlay.type,
                name: pendingPlay.name
            });
        } catch (err) {
            console.error("Failed to play on speaker:", err);
        }

        setLoading(false);
        setPendingPlay(null);
    }

    const deviceEntries = Object.entries(devices);

    return html`
        <div class="tunein-browser deezer-browser">
            <div class="tunein-toolbar">
                <select
                    class="btn-secondary"
                    style=${{ marginRight: '8px', padding: '0 8px', height: '36px', borderRadius: '4px' }}
                    value=${searchType}
                    onChange=${(e) => setSearchType(e.target.value)}
                >
                    <option value="album">Albums</option>
                    <option value="artist">Artists</option>
                    <option value="track">Tracks</option>
                </select>

                <input
                    type="text"
                    class="tunein-search-input"
                    placeholder="Search Deezer..."
                    value=${searchQuery}
                    onInput=${(e) => setSearchQuery(e.target.value)}
                    onKeyDown=${(e) => e.key === 'Enter' && search(searchQuery, searchType)}
                />
                <button class="btn-primary" onClick=${() => search(searchQuery, searchType)}>Search</button>
                <button class="btn-secondary" onClick=${() => {
                    setSearchQuery('');
                    setStatusMessage('');
                    setSections([]);
                }}>Clear</button>
            </div>

            ${statusMessage ? html`<div class="tunein-item-desc" style=${{ padding: '8px 0' }}>${statusMessage}</div>` : null}
            ${loading ? html`<div class="loading-bar"></div>` : null}

            ${sections.map(section => html`
                <div>
                    ${section.name ? html`<h4 class="tunein-section-name">${section.name}</h4>` : null}
                    <ul class="tunein-list">
                        ${section.items.map((item, i) => html`
                            <li key=${item.id || i} class="tunein-item" onClick=${() => handleItemSelection(item)}>
                                ${item.imageUrl ? html`<img class="tunein-thumb" src=${item.imageUrl} alt="" />` : null}
                                <div class="tunein-item-info">
                                    <span class="tunein-item-name">${item.name}</span>
                                    ${item.subtitle ? html`<span class="tunein-item-desc">${item.subtitle}</span>` : null}
                                </div>
                                <button
                                    class="tunein-play-btn"
                                    title=${item.type === 'artist' ? 'View Discography' : 'Play'}
                                    onClick=${(e) => {
                                        e.stopPropagation();
                                        handleItemSelection(item);
                                    }}
                                >
                                    ${item.type === 'artist' ? '👁' : '▶'}
                                </button>
                            </li>
                        `)}
                    </ul>
                </div>
            `)}

            ${pendingPlay ? html`
                <div class="overlay" onClick=${() => setPendingPlay(null)}>
                    <div class="device-picker" onClick=${(e) => e.stopPropagation()}>
                        <h3 class="picker-title">Play on device</h3>
                        <p class="picker-item-name">${pendingPlay.name}</p>
                        <div class="picker-devices">
                            ${deviceEntries.length === 0 ? html`<p class="picker-no-devices">No devices found. Try discovering first.</p>` : null}
                            ${deviceEntries.map(([id, d]) => html`
                                <button class="picker-device-btn" key=${id} onClick=${() => playOn(id)}>
                                    <div class="picker-device-info">
                                        <span class="picker-device-name">${d.info?.name || id}</span>
                                        <span class="picker-device-ip">${d.info?.ip_address || ''}</span>
                                    </div>
                                </button>
                            `)}
                        </div>
                    </div>
                </div>
            ` : null}
        </div>
    `;
}
