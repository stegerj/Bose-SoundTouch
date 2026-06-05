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

    // Protezione centralizzata per evitare il crash di Preact all'avvio
    const deviceEntries = Object.entries(devices || {}).filter(([id, dev]) => id && dev);

    // Funzione intelligente unica per gestire il play istantaneo o l'apertura del popup originale
    async function triggerPlay(item) {
        if (!item) return;

        // CASO 1: SE È UN ARTISTA -> Recuperiamo i brani della sua radio dal backend Go e prendiamo il primo
        if (item.type === 'artist') {
            setLoading(true);
            setStatusMessage(`Loading radio for ${item.name}...`);
            try {
                // Chiamiamo il nuovo endpoint Go creato via api.js
                const response = await api.deezerArtistRadio(item.id);
                const radioTracks = response?.data || response;

                 // Dentro il blocco dell'artista in triggerPlay, assicurati che estragga l'indice 0:
                if (radioTracks && Array.isArray(radioTracks) && radioTracks.length > 0) {
                 const firstTrack = radioTracks[0]; // <-- Deve prendere l'indice 0 dell'array!
                 item = {
                     id: String(firstTrack.id),
                     name: firstTrack.title,
                     type: 'track'
                 };
               } else {
                    setStatusMessage(`No tracks found for ${item.name}'s radio.`);
                    setLoading(false);
                    return;
                }
            } catch (err) {
                console.error("Failed to load artist radio via backend Go:", err);
                setStatusMessage(`Error loading radio for ${item.name}.`);
                setLoading(false);
                return;
            }
        }

        // CASO 2 E 3: ALBUM O TRACK (O L'ARTISTA TRASFORMATO IN TRACK)
        // Se c'è un solo dispositivo reale attivo, invia il play direttamente e salta il popup
        if (deviceEntries.length === 1) {
            // FIXED: Estrae il primo elemento (l'ID/IP stringa) dal primo sotto-array di deviceEntries
            const [[singleDeviceId]] = deviceEntries;
            setLoading(true);

            try {
                await api.deezerPlay(singleDeviceId, {
                    location: String(item.id),
                    type: item.type,
                    itemName: item.name || item.title // Coerente con la struct del backend Go
                });
            } catch (err) {
                console.error("Immediate playback failed:", err);
            } finally {
                setLoading(false);
            }

        } else {
            // Se ci sono più dispositivi, imposta lo stato per aprire il popup classico centrato
            setPendingPlay(item);
            setLoading(false);
        }
    }


    async function search(q, type) {
        if (!q || !q.trim()) return;
        setLoading(true);
        setStatusMessage('Searching Deezer library...');
        setSections([]);

        try {
            const result = await api.deezerSearch(q, type);
            setLoading(false);

            const itemsList = result?.data || result?.Data || result;

            if (!itemsList || !Array.isArray(itemsList) || itemsList.length === 0) {
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

    function mapAndSetItems(itemsList, defaultType, sectionName) {
        if (!Array.isArray(itemsList)) return;

        const filteredList = itemsList.filter(item => {
            if (!item) return false;
            const actualType = item.type || defaultType;
            if (actualType === 'artist' && item.nb_album === 0) {
                return false;
            }
            return true;
        });

        setSections([{
            name: sectionName,
            items: filteredList.map(item => {
                let imageUrl = '';
                let name = '';
                let subtitle = '';
                const actualType = item.type || defaultType;

                if (actualType === 'album') {
                    imageUrl = item.cover_medium || item.cover_small || '';
                    name = item.title || '';
                    subtitle = item.artist ? item.artist.name : '';
                } else if (actualType === 'artist') {
                    imageUrl = item.picture_medium || item.picture_small || '';
                    name = item.name || '';
                    subtitle = item.nb_album !== undefined ? `${item.nb_album} Albums` : 'Artist';
                } else if (actualType === 'track') {
                    imageUrl = item.album ? (item.album.cover_medium || item.album.cover_small) : '';
                    name = item.title || '';
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

    async function playOn(deviceId) {
        if (!pendingPlay) return;
        setLoading(true);

        try {
            await api.deezerPlay(deviceId, {
                location: String(pendingPlay.id),
                type: pendingPlay.type,
                itemName: pendingPlay.name || pendingPlay.title // Coerente con la struct del backend Go
            });
        } catch (err) {
            console.error("Failed to play on speaker:", err);
        }

        setLoading(false);
        setPendingPlay(null);
    }

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
                        ${section.items.map((item, i) => {
                            if (item.type === 'artist') {
                                return html`<${ArtistAccordionItem} key=${item.id || i} item=${item} setPendingPlay=${triggerPlay} />`;
                            } else if (item.type === 'album') {
                                return html`<${AlbumAccordionItem} key=${item.id || i} item=${item} setPendingPlay=${triggerPlay} />`;
                            } else {
                                return html`
                                    <li class="tunein-item" onClick=${() => triggerPlay(item)}>
                                        ${item.imageUrl ? html`<img class="tunein-thumb" src=${item.imageUrl} alt="" />` : null}
                                        <div class="tunein-item-info">
                                            <div class="tunein-item-title">${item.name}</div>
                                            <div class="tunein-item-desc">${item.subtitle}</div>
                                        </div>
                                    </li>
                                `;
                            }
                        })}
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
                                         <span class="picker-device-name">${d.DeviceInfo?.Name || d.info?.name || id}</span>
                                         <span class="picker-device-ip">${d.DeviceInfo?.IPAddress || d.info?.ip_address || ''}</span>
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

// ============================================================================
// INCOLLA QUESTO IN FONDO AL FILE DEEZERBROWSER.JS PER RISOLVERE L'ERRORE
// ============================================================================

function ArtistAccordionItem({ item, setPendingPlay }) {
    const [isOpen, setIsOpen] = useState(false);
    const [details, setDetails] = useState({ albums: [], tracks: [] });
    const [localLoading, setLocalLoading] = useState(false);

    async function toggleArtist(e) {
        if (e) e.stopPropagation();
        const nextState = !isOpen;
        setIsOpen(nextState);

        if (nextState && details.albums.length === 0 && details.tracks.length === 0) {
            setLocalLoading(true);
            try {
                const response = await api.deezerArtistDetails(item.id);
                const artistData = response?.data || response;
                if (artistData) {
                    setDetails({
                        albums: artistData.albums || [],
                        tracks: artistData.tracks || []
                    });
                }
            } catch (err) {
                console.error("Failed to load artist details inside accordion:", err);
            } finally {
                setLocalLoading(false);
            }
        }
    }

    return html`
        <li class="tunein-item accordion-item" style=${{ flexDirection: 'column', alignItems: 'stretch' }}>
            <div style=${{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
                <div onClick=${toggleArtist} style=${{ display: 'flex', alignItems: 'center', flexGrow: 1, cursor: 'pointer' }}>
                    ${item.imageUrl ? html`<img class="tunein-thumb" src=${item.imageUrl} alt="" style=${{ borderRadius: '50%' }} />` : null}
                    <div class="tunein-item-info">
                        <div class="tunein-item-title">${item.name} <span style=${{ fontSize: '12px', marginLeft: '6px' }}>${isOpen ? '▲' : '▼'}</span></div>
                        <div class="tunein-item-desc">${item.subtitle}</div>
                    </div>
                </div>
                <button class="btn-secondary" style=${{ height: '30px', padding: '0 8px', fontSize: '12px' }}
                        onClick=${(e) => { e.stopPropagation(); setPendingPlay(item); }}>
                    ▶ Radio
                </button>
            </div>

            ${isOpen ? html`
                <div class="accordion-content" style=${{ paddingLeft: '24px', marginTop: '8px', borderLeft: '2px solid #ddd', width: '100%' }}>
                    ${localLoading ? html`<div class="tunein-item-desc">Loading discography...</div>` : null}

                    ${details.tracks.length > 0 ? html`
                        <div style=${{ marginBottom: '12px' }}>
                            <div class="tunein-section-name" style=${{ fontSize: '13px', margin: '4px 0', textTransform: 'uppercase' }}>Top Tracks</div>
                            <ul style=${{ listStyle: 'none', padding: 0, margin: 0 }}>
                                ${details.tracks.map(t => {
                                    const trackImg = t.album ? (t.album.cover_medium || t.album.cover_small) : item.imageUrl;
                                    return html`
                                        <li key=${t.id} class="tunein-item" style=${{ padding: '6px 0', borderBottom: '1px solid #eee' }}
                                            onClick=${(e) => { e.stopPropagation(); setPendingPlay({ id: t.id, name: t.title, imageUrl: trackImg, type: 'track' }); }}>
                                            <div class="tunein-item-info">
                                                <div class="tunein-item-title" style=${{ fontSize: '14px' }}>🎵 ${t.title}</div>
                                            </div>
                                        </li>
                                    `;
                                })}
                            </ul>
                        </div>
                    ` : null}

                    ${details.albums.length > 0 ? html`
                        <div>
                            <div class="tunein-section-name" style=${{ fontSize: '13px', margin: '4px 0', textTransform: 'uppercase' }}>Albums (${details.albums.length})</div>
                            <ul style=${{ listStyle: 'none', padding: 0, margin: 0 }}>
                                ${details.albums.map(a => html`
                                    <${AlbumAccordionItem}
                                        key=${a.id}
                                        item=${{ id: a.id, name: a.title, imageUrl: a.cover_medium || a.cover_small, subtitle: 'Album', type: 'album' }}
                                        setPendingPlay=${setPendingPlay}
                                    />
                                `)}
                            </ul>
                        </div>
                    ` : null}
                </div>
            ` : null}
        </li>
    `;
}

function AlbumAccordionItem({ item, setPendingPlay }) {
    const [isOpen, setIsOpen] = useState(false);
    const [tracks, setTracks] = useState([]);
    const [localLoading, setLocalLoading] = useState(false);

    async function toggleAlbum(e) {
        if (e) e.stopPropagation();
        const nextState = !isOpen;
        setIsOpen(nextState);

        if (nextState && tracks.length === 0) {
            setLocalLoading(true);
            try {
                const response = await api.deezerAlbumTracks(item.id);
                const tracksData = response?.data || response;
                if (tracksData) {
                    setTracks(tracksData);
                }
            } catch (err) {
                console.error("Failed to load album tracks inside accordion:", err);
            } finally {
                setLocalLoading(false);
            }
        }
    }

    return html`
        <li class="tunein-item accordion-item" style=${{ flexDirection: 'column', alignItems: 'stretch', padding: '8px 0', borderBottom: '1px solid #f0f0f0' }}>
            <div style=${{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
                <div onClick=${toggleAlbum} style=${{ display: 'flex', alignItems: 'center', flexGrow: 1, cursor: 'pointer' }}>
                    ${item.imageUrl ? html`<img class="tunein-thumb" src=${item.imageUrl} alt="" style=${{ width: '34px', height: '34px' }} />` : null}
                    <div class="tunein-item-info">
                        <div class="tunein-item-title" style=${{ fontSize: '14px' }}>${item.name} <span style=${{ fontSize: '11px', marginLeft: '4px' }}>${isOpen ? '▲' : '▼'}</span></div>
                        <div class="tunein-item-desc" style=${{ fontSize: '12px' }}>${item.subtitle}</div>
                    </div>
                </div>
                <button class="btn-secondary" style=${{ height: '26px', padding: '0 6px', fontSize: '11px' }}
                        onClick=${(e) => { e.stopPropagation(); setPendingPlay(item); }}>
                    ▶ Album
                </button>
            </div>

            ${isOpen ? html`
                <div class="accordion-content" style=${{ paddingLeft: '16px', marginTop: '6px', borderLeft: '2px dashed #ccc', width: '100%' }}>
                    ${localLoading ? html`<div class="tunein-item-desc" style=${{ fontSize: '12px' }}>Loading tracks...</div>` : null}
                    <ul style=${{ listStyle: 'none', padding: 0, margin: 0 }}>
                        ${tracks.map((t, index) => html`
                            <li key=${t.id} class="tunein-item" style=${{ padding: '4px 0', borderBottom: '1px solid #f9f9f9', minHeight: 'auto' }}
                                onClick=${(e) => { e.stopPropagation(); setPendingPlay({ id: t.id, name: t.title, imageUrl: item.imageUrl, type: 'track' }); }}>
                                <div class="tunein-item-info">
                                    <div class="tunein-item-title" style=${{ fontSize: '13px', fontWeight: 'normal' }}>
                                        <span style=${{ color: '#888', marginRight: '6px' }}>${index + 1}.</span> ${t.title}
                                    </div>
                                </div>
                            </li>
                        `)}
                    </ul>
                </div>
            ` : null}
        </li>
    `;
}
